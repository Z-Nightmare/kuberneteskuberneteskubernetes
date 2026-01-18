# discovery 模块

`cmd/discovery` 是一个基于 **Consul** 的服务注册与发现模块：它通过 Consul 注册当前节点服务，并从 Consul 发现其他节点，然后将发现到的节点以 **`corev1.Node`** 的形式写入共享的 **`storage.Store`**（etcd/mysql/memory），从而让 controller / scheduler / dashboard 能看到"当前有哪些节点"。

## 目标与边界

- **目标**：使用 Consul 作为服务注册中心，自动发现注册在 Consul 中的节点，并把它们注册到 node 列表（`v1/Node`）。
- **边界**：
  - 使用 Consul 的 Catalog API 进行服务发现
  - 支持健康检查，自动同步服务健康状态到 Node 条件
  - 不负责真正节点的 kubelet 上报；若 controller 已上报节点，本模块不会覆盖

## 实现概览

核心实现位于 `internal/discovery/service.go`，整体流程如下：

1. **注册服务到 Consul**
   - 使用 Consul Agent API 注册当前节点服务
   - 配置健康检查（HTTP 健康检查端点）
   - 设置服务元数据（节点名称、PID 等）

2. **服务发现循环**
   - 定期从 Consul Catalog 查询服务列表
   - 解析服务信息并同步到 Kubernetes Node 资源
   - 根据 Consul 健康检查状态更新 Node 条件

3. **写入 Node（同步到 storage.Store）**
   - 以 `schema.GroupVersionKind{Version:"v1", Kind:"Node"}` 写入
   - 从 Consul 服务元数据中提取节点名称
   - 根据健康检查状态设置 Node Ready 条件

## 使用方法

### 启动 discovery 模块

#### 使用 Go 直接运行

```bash
# 使用默认配置
go run ./cmd/discovery

# 指定 Consul 地址和服务名称
go run ./cmd/discovery --consul-addr localhost:8500 --service-name k3-node

# 指定节点名称
go run ./cmd/discovery --node-name my-node --consul-addr localhost:8500

# 使用配置文件
go run ./cmd/discovery --config ./cmd/discovery/config-example.yaml
```

#### 使用 Docker 运行

```bash
# 构建 Docker 镜像
docker build -f deploy/docker/discovery/Dockerfile -t k3-discovery:latest .

# 运行容器（需要连接到 Consul 和存储后端）
docker run -d \
  --name k3-discovery \
  --network host \
  -e NODE_NAME=node-1 \
  -v $(pwd)/cmd/discovery/config-example.yaml:/app/config.yaml \
  k3-discovery:latest \
  --consul-addr localhost:8500 \
  --service-name k3-node \
  --node-name node-1 \
  --config /app/config.yaml

# 或者使用 docker-compose（见下方示例）
```

#### 使用 Docker Compose

创建一个 `docker-compose.discovery.yaml` 文件：

```yaml
version: '3.8'

services:
  consul:
    image: consul:latest
    container_name: consul
    ports:
      - "8500:8500"
    command: consul agent -dev -client=0.0.0.0

  etcd:
    image: quay.io/coreos/etcd:v3.5.0
    container_name: etcd
    ports:
      - "2379:2379"
    environment:
      - ETCD_ADVERTISE_CLIENT_URLS=http://0.0.0.0:2379
      - ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:2379

  discovery:
    build:
      context: .
      dockerfile: deploy/docker/discovery/Dockerfile
    container_name: k3-discovery
    depends_on:
      - consul
      - etcd
    environment:
      - NODE_NAME=node-1
      - CONFIG_PATH=/app/config.yaml
    volumes:
      - ./cmd/discovery/config-example.yaml:/app/config.yaml
    ports:
      - "7946:7946"
    command:
      - --consul-addr
      - consul:8500
      - --service-name
      - k3-node
      - --node-name
      - node-1
      - --config
      - /app/config.yaml
```

然后运行：

```bash
docker-compose -f docker-compose.discovery.yaml up -d
```

### 命令行参数

- `--config`: 配置文件路径（等价于环境变量 CONFIG_PATH）
- `--consul-addr`: Consul 服务器地址（默认：localhost:8500）
- `--consul-token`: Consul ACL token（可选）
- `--service-name`: 服务名称（默认：k3-node）
- `--service-id`: 服务 ID（默认：service-name-node-name）
- `--service-port`: 服务端口（默认：7946）
- `--node-name`: 节点名称（默认：NODE_NAME 环境变量或 hostname）
- `--register-self`: 是否将当前节点也注册到 store（默认：true）
- `--watch-interval`: 从 Consul 发现服务并同步到 store 的间隔（默认：15s）
- `--health-check-interval`: 健康检查间隔（默认：10s）
- `--health-check-timeout`: 健康检查超时（默认：3s）
- `--deregister-after`: 服务不健康后多久注销（默认：30s）

### 配置

discovery 模块使用项目的统一配置文件，需要配置存储后端：

```yaml
storage:
  type: etcd    # 建议使用 etcd/mysql 以便多节点共享
  etcd:
    endpoints:
      - http://127.0.0.1:2379
```

## Consul 服务注册格式

注册到 Consul 的服务信息：

- **服务名称**: 由 `--service-name` 指定（默认：k3-node）
- **服务 ID**: 由 `--service-id` 指定（默认：service-name-node-name）
- **服务地址**: 自动检测本地 IP 地址
- **服务端口**: 由 `--service-port` 指定（默认：7946）
- **健康检查**: HTTP 健康检查，端点：`http://<service-address>:<service-port>/healthz`
- **元数据**:
  - `node`: 节点名称
  - `pid`: 进程 ID

## 健康检查

discovery 模块注册的服务包含 HTTP 健康检查：

- **检查端点**: `http://<service-address>:<service-port>/healthz`
- **检查间隔**: 由 `--health-check-interval` 指定（默认：10s）
- **检查超时**: 由 `--health-check-timeout` 指定（默认：3s）
- **不健康后注销时间**: 由 `--deregister-after` 指定（默认：30s）

**注意**: 健康检查端点需要由应用程序提供。如果应用程序没有提供健康检查端点，Consul 会认为服务不健康。

## 与 network 模块的区别

- **network 模块**: 使用 mDNS（zeroconf）在局域网内进行服务发现，适合本地开发和小规模部署
- **discovery 模块**: 使用 Consul 进行服务注册与发现，适合生产环境和大规模部署，支持跨网络、跨数据中心的服务发现

## 注意事项

- 确保 Consul 服务器正在运行并可访问
- 健康检查端点需要由应用程序提供（`/healthz`）
- 如果使用 ACL，需要提供有效的 Consul token
- Node 资源没有 namespace，存储时会忽略 namespace 字段
- 如果 controller 已上报节点，discovery 模块不会覆盖，只会更新心跳时间
