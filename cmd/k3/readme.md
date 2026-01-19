# k3 命令行工具

`k3` 是 KubernetesKubernetesKubernetes 项目的统一命令行入口，用于管理 k3 集群的各个模块（storage、controller、web），以及提交 Kubernetes 资源到 apiserver。

## 快速开始

```bash
# 使用默认配置启动完整集群（storage -> controller -> web）
go run ./cmd/k3 start

# 指定配置文件
go run ./cmd/k3 start --config .config.yaml
```

## 命令概览

```
Usage:
  k3 <command> [flags]

Commands:
  start                 按顺序启动 storage -> controller -> web（单进程）
  run                   根据配置中的 role 启动不同模式（master/node/one）
  storage               仅启动 storage（包含按需拉起 mysql/etcd 容器）
  controller            启动 storage + controller
  web                   仅启动 web 模块（假设 storage 已运行）
  apply                 将 Kubernetes YAML/JSON 提交到 apiserver（最小 apply 子集）
  cluster create        创建 k3 集群配置骨架（多节点配置文件）
  cluster clear         删除 k3 集群配置目录以及关联的容器

Flags:
  --config <path>       指定配置文件路径（默认: ./.config.yaml；也支持环境变量 CONFIG_PATH）
```

## 命令详解

### `run` - 根据角色启动不同模式

根据配置文件中的 `role` 字段启动不同的模块组合。这是推荐的部署方式，支持三种角色模式。

**角色模式**：

- **`master`**：启动 apiserver、storage、discovery 模块
  - 适用于控制平面节点
  - 提供 API Server 和 Dashboard
  - 提供服务发现功能
  - 不运行控制器（由 node 节点运行）

- **`node`**：启动 controller 模块
  - 适用于工作节点
  - 运行控制器管理器（Deployment、Scheduler、Runtime）
  - 需要连接到共享的 storage（mysql/etcd）

- **`one`**：启动 apiserver、storage、discovery、controller 模块
  - 适用于单节点部署或开发环境
  - 所有功能集成在一个进程中

**使用示例**：

```bash
# 使用默认配置（role: one）
go run ./cmd/k3 run

# 指定配置文件（master 模式）
go run ./cmd/k3 run --config master-config.yaml

# 指定配置文件（node 模式）
go run ./cmd/k3 run --config node-config.yaml

# 使用环境变量指定配置
CONFIG_PATH=master-config.yaml go run ./cmd/k3 run
```

**配置文件示例**：

```yaml
# master-config.yaml
debug: true
role: master  # master/node/one

web:
  port: 8080
  cors: true

log:
  level: debug
  path: ""

storage:
  type: etcd  # 多节点部署建议使用 etcd 或 mysql
  etcd:
    endpoints:
      - http://localhost:2379
    dial_timeout: 5s

jwt:
  signing_key: secret
```

```yaml
# node-config.yaml
debug: true
role: node  # node 模式

log:
  level: debug
  path: ""

storage:
  type: etcd  # 必须与 master 使用相同的存储
  etcd:
    endpoints:
      - http://master-node:2379  # 连接到 master 的 etcd
    dial_timeout: 5s
```

**环境变量**（用于 discovery 模块）：

- `CONSUL_ADDR`：Consul 服务器地址（默认：`localhost:8500`）
- `CONSUL_TOKEN`：Consul ACL token（可选）
- `SERVICE_NAME`：服务名称（默认：`k3-node`）
- `NODE_NAME`：节点名称（默认：系统 hostname）

**部署场景**：

1. **单节点部署**（开发/测试）：
   ```yaml
   role: one
   ```
   所有模块运行在一个进程中。

2. **多节点集群**（生产环境）：
   - Master 节点配置：
     ```yaml
     role: master
     storage:
       type: etcd
       etcd:
         endpoints:
           - http://etcd-cluster:2379
     ```
   - Node 节点配置：
     ```yaml
     role: node
     storage:
       type: etcd
       etcd:
         endpoints:
           - http://etcd-cluster:2379  # 连接到共享的 etcd
     ```

**注意事项**：

- `master` 和 `node` 模式必须使用**相同的存储后端**（mysql/etcd）才能共享资源数据
- `node` 模式需要确保 storage 已启动并可访问
- `discovery` 模块需要 Consul 服务运行（可通过环境变量配置）
- 默认情况下，`discovery` 模块会注册当前节点到 Consul 并同步到 Node 列表

### `start` - 启动完整集群

按顺序启动所有模块：**storage → controller → web**（单进程模式）。

**功能**：
- 自动拉起 MySQL/etcd 容器（如果配置为 localhost）
- 初始化存储后端（memory/mysql/etcd）
- 启动控制器管理器（Deployment、Scheduler、Runtime）
- 启动 Web 服务器（Dashboard + API Server）

**使用示例**：

```bash
# 使用默认配置（.config.yaml）
go run ./cmd/k3 start

# 指定配置文件
go run ./cmd/k3 start --config .k3/node-1/.config.yaml

# 使用环境变量指定配置
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/k3 start
```

**启动后的服务**：
- Dashboard: `http://localhost:8080/`（默认端口）
- API Server: `http://localhost:8080/api/v1/...` 和 `http://localhost:8080/apis/...`
- Health Check: `http://localhost:8080/api/healthz`

### `storage` - 仅启动存储模块

仅启动 storage 模块，保持数据库连接和容器生命周期。

**使用场景**：
- 仅需要存储后端，不需要控制器和 Web
- 调试存储连接问题
- 多进程部署时，单独管理存储

**使用示例**：

```bash
go run ./cmd/k3 storage --config .config.yaml
```

**功能**：
- 自动检测并拉起 MySQL/etcd 容器（如果配置为 localhost）
- 初始化存储连接
- 保持容器运行直到进程退出

### `controller` - 启动控制器模块

启动 storage + controller，不启动 Web 服务器。

**使用场景**：
- 仅需要控制器功能（Deployment、Scheduler、Runtime）
- 分离部署：controller 和 web 运行在不同进程

**使用示例**：

```bash
go run ./cmd/k3 controller --config .config.yaml
```

**功能**：
- 启动存储后端
- 启动控制器管理器：
  - DeploymentController（管理 Deployment 资源）
  - SchedulerController（调度 Pod 到节点）
  - RuntimeController（启动容器）
- 节点心跳上报

### `web` - 启动 Web 模块

仅启动 web 模块，不自动拉起 storage 容器，不启动 controller。

**使用场景**：
- 仅需要 Web Dashboard 和 API Server
- 分离部署：storage、controller 和 web 运行在不同进程
- 假设 storage 已经运行（通过 `cmd/k3 storage` 或 `cmd/storage` 启动）

**使用示例**：

```bash
# 终端 1：先启动 storage
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/k3 storage

# 终端 2：启动 web（连接到已运行的 storage）
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/k3 web
```

**功能**：
- 连接到已运行的存储后端（不自动拉起容器）
- 启动 Web 服务器：
  - Dashboard（实时资源看板）
  - API Server（Kubernetes 风格 REST API）

**注意事项**：
- 不会自动拉起 MySQL/etcd 容器（假设 storage 已运行）
- 需要确保 storage 后端已启动并可访问
- 如果 storage 未运行，web 服务将无法正常工作

### `apply` - 提交 Kubernetes 资源

将 Kubernetes YAML/JSON 文件提交到 apiserver（类似 `kubectl apply` 的最小实现）。

**功能特性**：
- 支持多文档 YAML（使用 `---` 分隔）
- 自动识别资源类型（Pod、Service、Deployment 等）
- 自动构建正确的 API 路径
- 支持 upsert：如果资源已存在，自动执行更新（PUT）

**支持的资源类型**：
- Core API v1: Pod、Service、ConfigMap、Secret、Node
- Apps API v1: Deployment、StatefulSet、DaemonSet

**使用示例**：

```bash
# 提交单个资源文件
go run ./cmd/k3 apply -f example/core-v1/pod.yaml

# 指定配置文件（用于读取 apiserver 地址）
go run ./cmd/k3 apply --config .config.yaml -f example/apps-v1/deployment.yaml

# 指定 apiserver 地址
go run ./cmd/k3 apply -f example/core-v1/service.yaml --server http://localhost:8080

# 提交多文档 YAML
go run ./cmd/k3 apply -f multi-resource.yaml
```

**参数说明**：
- `-f <file>`: 要提交的 YAML/JSON 文件路径（必需）
- `--config <path>`: 配置文件路径（用于读取 apiserver 端口，默认从 `.config.yaml` 读取）
- `--server <url>`: apiserver 地址（默认从配置读取，例如 `http://localhost:8080`）

**输出示例**：

```bash
$ go run ./cmd/k3 apply -f example/apps-v1/deployment.yaml
已提交 Deployment default/nginx-deployment

$ go run ./cmd/k3 apply -f example/apps-v1/deployment.yaml  # 再次提交
已更新 Deployment default/nginx-deployment
```

### `cluster create` - 创建集群配置骨架

生成多节点配置文件，便于管理多个 k3 实例。

**使用场景**：
- 本地测试多节点集群
- 生成标准化的配置文件模板

**使用示例**：

```bash
# 创建 3 个节点的配置（使用 memory 存储）
go run ./cmd/k3 cluster create --dir .k3 --nodes 3 --web-port 8080 --storage memory

# 创建单节点配置（使用 mysql 存储）
go run ./cmd/k3 cluster create --dir .k3 --nodes 1 --web-port 8080 --storage mysql

# 创建 5 个节点的配置（使用 etcd 存储）
go run ./cmd/k3 cluster create --dir .k3 --nodes 5 --web-port 8080 --storage etcd
```

**参数说明**：
- `--dir <path>`: 输出目录（默认 `.k3`）
- `--nodes <count>`: 节点数量（默认 `1`）
- `--web-port <port>`: node-1 的 web 端口（默认 `8080`），后续节点端口递增
- `--storage <type>`: storage 类型（`memory`/`mysql`/`etcd`，默认 `memory`）

**生成的目录结构**：

```
.k3/
├── node-1/
│   └── .config.yaml    # web-port: 8080
├── node-2/
│   └── .config.yaml    # web-port: 8081
└── node-3/
    └── .config.yaml    # web-port: 8082
```

**启动节点**：

```bash
# 启动 node-1
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/k3 start

# 启动 node-2（在另一个终端）
CONFIG_PATH=.k3/node-2/.config.yaml go run ./cmd/k3 start
```

### `cluster clear` - 清理集群配置和容器

删除 k3 集群配置目录以及所有关联的 Docker 容器。

**使用场景**：
- 清理测试环境
- 重置集群配置
- 删除不再需要的集群实例

**使用示例**：

```bash
# 清理默认目录 .k3 及其关联容器（会询问确认）
go run ./cmd/k3 cluster clear

# 清理指定目录
go run ./cmd/k3 cluster clear --dir .k3

# 不询问确认，直接删除
go run ./cmd/k3 cluster clear --dir .k3 --force
```

**参数说明**：
- `--dir <path>`: 要删除的集群配置目录（默认 `.k3`）
- `--force`: 不询问确认，直接删除

**清理内容**：
1. **配置目录**：删除指定的集群配置目录及其所有内容（如 `.k3/`）
2. **存储容器**：
   - `k8s_storage_mysql_mysql`（MySQL 存储容器）
   - `k8s_storage_etcd_etcd`（Etcd 存储容器）
3. **Pod 容器**：所有以 `k8s_` 开头的容器（包括 Pod 运行时容器）

**安全特性**：
- 默认会询问确认，避免误删
- 禁止删除 `.`、`/`、`..` 等危险路径
- 仅清理 k3 相关的容器（以 `k8s_` 开头）

**输出示例**：

```bash
$ go run ./cmd/k3 cluster clear --dir .k3
警告：将删除目录 .k3 及其所有内容，以及关联的 k3 容器
确认删除？(yes/no): yes
正在清理关联容器...
  已删除容器: k8s_storage_mysql_mysql
  已删除容器: k8s_default_nginx-1768728378560843000_nginx
  已删除容器: k8s_default_nginx-1768728378569414000_nginx
正在删除配置目录: .k3
✅ 清理完成：已删除 3 个容器，已删除目录 .k3
```

## 配置文件

### 配置文件路径

k3 按以下顺序查找配置文件：

1. 命令行参数 `--config <path>`
2. 环境变量 `CONFIG_PATH`
3. 默认路径 `./.config.yaml`
4. 回退路径 `./configs/config-example.yaml`（如果默认路径不存在）

### 配置文件示例

参考 `config-example.yaml`：

```yaml
debug: true

role: one  # master/node/one

web:
  port: 8080
  cors: true

log:
  level: debug
  path: ""

storage:
  type: memory   # memory/mysql/etcd（多节点共享请用 mysql/etcd）
  mysql:
    host: localhost
    port: 3306
    user: root
    password: password
    database: k3
    max_open_conns: 10
    max_idle_conns: 5
  etcd:
    endpoints:
      - http://127.0.0.1:2379
    dial_timeout: 5s
    username: ""
    password: ""

jwt:
  signing_key: secret
```

**role 配置说明**：

- `master`：控制平面节点，运行 apiserver、storage、discovery
- `node`：工作节点，运行 controller
- `one`：单节点模式，运行所有模块（默认值）

### 存储类型说明

#### `memory`（内存存储）
- **特点**：数据存储在内存中，进程退出后丢失
- **适用场景**：单进程测试、快速原型
- **限制**：多进程无法共享数据

#### `mysql`（MySQL 存储）
- **特点**：数据持久化到 MySQL，支持多进程共享
- **适用场景**：生产环境、多节点集群
- **自动管理**：如果配置为 `localhost`，k3 会自动拉起 MySQL 容器（`mysql:8.0`）

#### `etcd`（Etcd 存储）
- **特点**：数据存储在 etcd，支持多进程共享
- **适用场景**：分布式集群、高可用场景
- **自动管理**：如果配置为 `localhost`，k3 会自动拉起 etcd 容器

## 环境变量

### `CONFIG_PATH`
指定配置文件路径（等价于 `--config` 参数）。

```bash
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/k3 start
```

### `NODE_NAME`
指定节点名称（用于 controller 上报节点信息）。

```bash
NODE_NAME=my-node-1 go run ./cmd/k3 start
```

如果不设置，默认使用系统 hostname。

## 使用场景

### 场景 1：快速启动单节点集群

```bash
# 方式 1：使用 run 命令（推荐）
# 1. 创建配置文件，设置 role: one
echo "role: one" >> .config.yaml
# ... 添加其他配置

# 2. 启动集群
go run ./cmd/k3 run

# 方式 2：使用 start 命令（传统方式）
# 1. 生成配置
go run ./cmd/k3 cluster create --dir .k3 --nodes 1 --storage mysql

# 2. 启动集群
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/k3 start

# 3. 访问 Dashboard
open http://localhost:8080/
```

### 场景 2：提交 Deployment 并验证

```bash
# 1. 启动集群（后台运行）
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/k3 start &

# 2. 等待服务就绪
sleep 5

# 3. 提交 Deployment
go run ./cmd/k3 apply -f example/apps-v1/deployment.yaml

# 4. 验证 Pod 已创建
curl http://localhost:8080/api/v1/namespaces/default/pods

# 5. 验证容器已运行
docker ps --filter "name=k8s_default_nginx"
```

### 场景 3：分离部署（多进程）

**方式 1：使用 run 命令（推荐）**

```bash
# 终端 1：启动 master 节点（apiserver + storage + discovery）
go run ./cmd/k3 run --config master-config.yaml

# 终端 2：启动 node 节点（controller）
go run ./cmd/k3 run --config node-config.yaml
```

**方式 2：使用传统命令**

```bash
# 终端 1：启动 storage + controller
go run ./cmd/k3 controller --config .config.yaml

# 终端 2：启动 web（连接到已运行的 storage）
go run ./cmd/k3 web --config .config.yaml
```

**注意**：分离部署时，所有进程必须使用**相同的存储配置**（mysql/etcd），才能共享资源数据。

### 场景 4：多节点集群部署

```bash
# Master 节点（控制平面）
# master-config.yaml
role: master
storage:
  type: etcd
  etcd:
    endpoints:
      - http://etcd-cluster:2379

go run ./cmd/k3 run --config master-config.yaml

# Node 节点 1（工作节点）
# node1-config.yaml
role: node
storage:
  type: etcd
  etcd:
    endpoints:
      - http://etcd-cluster:2379  # 连接到共享的 etcd

go run ./cmd/k3 run --config node1-config.yaml

# Node 节点 2（工作节点）
# node2-config.yaml
role: node
storage:
  type: etcd
  etcd:
    endpoints:
      - http://etcd-cluster:2379

go run ./cmd/k3 run --config node2-config.yaml
```

### 场景 4：批量提交资源

创建一个包含多个资源的 YAML 文件：

```yaml
# multi-resource.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: default
data:
  key: value
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx:latest
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-service
  namespace: default
spec:
  selector:
    app: nginx
  ports:
    - port: 80
```

然后提交：

```bash
go run ./cmd/k3 apply -f multi-resource.yaml
```

## 常见问题

### Q: 如何停止 k3 进程？

A: 使用 `Ctrl+C`（SIGINT）或 `kill` 命令。k3 会优雅关闭：
- 停止 Web 服务器
- 停止控制器
- 关闭存储连接
- 清理自动拉起的数据库容器（如果由本进程启动）

### Q: MySQL 容器启动失败？

A: 检查：
1. Docker 是否运行：`docker info`
2. 端口 3306 是否被占用：`lsof -i :3306`
3. 查看日志中的错误信息

### Q: `apply` 命令提示 "unsupported kind"？

A: 当前支持的资源类型有限，仅支持：
- Core v1: Pod、Service、ConfigMap、Secret、Node
- Apps v1: Deployment、StatefulSet、DaemonSet

其他资源类型需要扩展 `kindToPlural` 函数。

### Q: 如何查看提交的资源？

A: 使用 curl 或浏览器访问 API：

```bash
# 查看所有 Pods
curl http://localhost:8080/api/v1/namespaces/default/pods

# 查看所有 Deployments
curl http://localhost:8080/apis/apps/v1/namespaces/default/deployments

# 或访问 Dashboard
open http://localhost:8080/
```

### Q: 多节点配置如何共享数据？

A: 使用 `mysql` 或 `etcd` 存储类型，所有节点配置相同的数据库地址：

```yaml
storage:
  type: mysql
  mysql:
    host: 192.168.1.100  # 共享的 MySQL 服务器
    port: 3306
    user: root
    password: password
    database: kubernetes
```

**注意**：不要使用 `memory` 存储，因为各进程内存不共享。

## 相关文档

- [项目 README](../readme.md) - 项目总体介绍
- [example/README.md](../../example/README.md) - Kubernetes 资源示例
- [docs/example.md](../../docs/example.md) - 最小运行示例
