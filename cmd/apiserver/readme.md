# cmd/apiserver

`cmd/apiserver` 是 k3 的**Kubernetes API Server 模块**，提供符合 Kubernetes API 规范的 RESTful API 服务，支持资源的 CRUD 操作和 Watch 功能。

## 功能

- **RESTful API**：提供符合 Kubernetes API 规范的 RESTful 接口
- **多资源类型支持**：支持 Pod、Deployment、Service、ConfigMap、Secret、StatefulSet、DaemonSet、Node 等
- **命名空间支持**：支持命名空间级别的资源管理和隔离
- **Watch API**：支持 Server-Sent Events (SSE) 实时监听资源变更
- **存储集成**：支持多种存储后端（memory、mysql、etcd）
- **自动容器管理**：当配置指向 `localhost` 时，自动拉起 MySQL/etcd 容器

## 启动方式

在项目根目录执行：

```bash
go run ./cmd/apiserver
```

默认读取 `.config.yaml`；若不存在，会回退读取 `configs/config-example.yaml`。也可以显式指定：

```bash
CONFIG_PATH=config-example.yaml go run ./cmd/apiserver
```

启动后访问：

- API Server：`http://localhost:<port>/api/v1/...`（默认端口 `8080`）
- Dashboard：`http://localhost:<port>/`（如果启用了 dashboard 路由）

## API 端点

### Core API v1

#### Pods
- `GET /api/v1/pods` - 列出所有 Pods
- `GET /api/v1/pods/:name` - 获取指定 Pod
- `POST /api/v1/pods` - 创建 Pod
- `PUT /api/v1/pods/:name` - 更新 Pod
- `PATCH /api/v1/pods/:name` - 部分更新 Pod
- `DELETE /api/v1/pods/:name` - 删除 Pod
- `GET /api/v1/watch/pods` - 监听 Pod 变更（SSE）

#### Namespaced Pods
- `GET /api/v1/namespaces/:namespace/pods` - 列出指定命名空间的 Pods
- `GET /api/v1/namespaces/:namespace/pods/:name` - 获取指定命名空间的 Pod
- `POST /api/v1/namespaces/:namespace/pods` - 在指定命名空间创建 Pod
- `PUT /api/v1/namespaces/:namespace/pods/:name` - 更新指定命名空间的 Pod
- `PATCH /api/v1/namespaces/:namespace/pods/:name` - 部分更新 Pod
- `DELETE /api/v1/namespaces/:namespace/pods/:name` - 删除 Pod
- `GET /api/v1/watch/namespaces/:namespace/pods` - 监听指定命名空间的 Pod 变更

类似地，还支持 **Services**、**ConfigMaps**、**Secrets**、**Nodes** 等资源。

### Apps API v1

#### Deployments
- `GET /apis/apps/v1/deployments` - 列出所有 Deployments
- `GET /apis/apps/v1/deployments/:name` - 获取指定 Deployment
- `POST /apis/apps/v1/deployments` - 创建 Deployment
- `PUT /apis/apps/v1/deployments/:name` - 更新 Deployment
- `PATCH /apis/apps/v1/deployments/:name` - 部分更新 Deployment
- `DELETE /apis/apps/v1/deployments/:name` - 删除 Deployment
- `GET /apis/apps/v1/watch/deployments` - 监听 Deployment 变更

#### Namespaced Deployments
- `GET /apis/apps/v1/namespaces/:namespace/deployments` - 列出指定命名空间的 Deployments
- `GET /apis/apps/v1/namespaces/:namespace/deployments/:name` - 获取指定命名空间的 Deployment
- `POST /apis/apps/v1/namespaces/:namespace/deployments` - 在指定命名空间创建 Deployment
- `PUT /apis/apps/v1/namespaces/:namespace/deployments/:name` - 更新指定命名空间的 Deployment
- `PATCH /apis/apps/v1/namespaces/:namespace/deployments/:name` - 部分更新 Deployment
- `DELETE /apis/apps/v1/namespaces/:namespace/deployments/:name` - 删除 Deployment
- `GET /apis/apps/v1/watch/namespaces/:namespace/deployments` - 监听指定命名空间的 Deployment 变更

类似地，还支持 **StatefulSets**、**DaemonSets** 等资源。

## 使用示例

### 创建 Pod

```bash
curl -X POST http://localhost:8080/api/v1/namespaces/default/pods \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
      "name": "nginx-pod",
      "namespace": "default"
    },
    "spec": {
      "containers": [{
        "name": "nginx",
        "image": "nginx:latest"
      }]
    }
  }'
```

### 创建 Pod（YAML 格式）

```bash
curl -X POST http://localhost:8080/api/v1/namespaces/default/pods \
  -H "Content-Type: application/yaml" \
  --data-binary @pod.yaml
```

### 获取 Pod

```bash
curl http://localhost:8080/api/v1/namespaces/default/pods/nginx-pod
```

### 列出所有 Pods

```bash
curl http://localhost:8080/api/v1/namespaces/default/pods
```

### 更新 Pod

```bash
curl -X PUT http://localhost:8080/api/v1/namespaces/default/pods/nginx-pod \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
      "name": "nginx-pod",
      "namespace": "default"
    },
    "spec": {
      "containers": [{
        "name": "nginx",
        "image": "nginx:1.21"
      }]
    }
  }'
```

### 删除 Pod

```bash
curl -X DELETE http://localhost:8080/api/v1/namespaces/default/pods/nginx-pod
```

### Watch Pod 变更（SSE）

```bash
curl http://localhost:8080/api/v1/watch/namespaces/default/pods
```

### 创建 Deployment

```bash
curl -X POST http://localhost:8080/apis/apps/v1/namespaces/default/deployments \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
      "name": "nginx-deployment",
      "namespace": "default"
    },
    "spec": {
      "replicas": 3,
      "selector": {
        "matchLabels": {
          "app": "nginx"
        }
      },
      "template": {
        "metadata": {
          "labels": {
            "app": "nginx"
          }
        },
        "spec": {
          "containers": [{
            "name": "nginx",
            "image": "nginx:latest"
          }]
        }
      }
    }
  }'
```

## 配置说明

### 存储配置

API Server 需要配置存储后端（与 `cmd/storage` 使用相同的配置）：

```yaml
storage:
  type: mysql   # memory / mysql / etcd
  mysql:
    host: localhost      # 指向 localhost 时会自动拉起容器
    port: 3306
    user: root
    password: password
    database: k3
    max_open_conns: 10
    max_idle_conns: 5
  etcd:
    endpoints:
      - http://127.0.0.1:2379  # 指向 localhost 时会自动拉起容器
    dial_timeout: 5s
    username: ""
    password: ""
```

### Web 配置

```yaml
web:
  port: 8080    # API Server 监听端口
  cors: true     # 是否启用 CORS
```

### 自动容器管理

当存储配置指向 `localhost`（或 `127.0.0.1`、`::1`）时，API Server 会：

1. 自动检测容器运行时（优先使用 Docker）
2. 检查容器是否已存在
3. 自动拉起 MySQL/etcd 容器
4. 连接重试（MySQL 最多 45 秒）
5. 退出时自动清理容器

## 使用场景

### 1. 独立 API Server

在多进程部署中，可以单独运行 API Server：

```bash
# 终端 1：启动存储服务
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/storage

# 终端 2：启动 API Server（连接到同一存储）
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/apiserver
```

### 2. 使用 k3 apply 命令

API Server 可以与 `cmd/k3` 的 `apply` 命令配合使用：

```bash
# 启动 API Server
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/apiserver

# 在另一个终端使用 k3 apply
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/k3 apply -f deployment.yaml
```

### 3. 集成到 k3 命令

`cmd/k3` 的 `start`、`web` 子命令都会自动启动 API Server。

## 功能特性

- ✅ **Write API**: Create、Update、Patch、Delete
- ✅ **Read API**: Get、List
- ✅ **Watch API**: Server-Sent Events 实时监听
- ✅ **资源版本管理**: 自动生成和管理 resourceVersion
- ✅ **命名空间支持**: 完整的命名空间隔离
- ✅ **事件通知**: ADDED、MODIFIED、DELETED、BOOKMARK 事件类型
- ✅ **YAML/JSON 支持**: 支持 YAML 和 JSON 格式的请求体
- ✅ **GVK 验证**: 自动验证路径和请求体中的 GroupVersionKind 是否匹配
- ✅ **Namespace 自动补全**: 如果 URL 中包含 namespace 但资源体中没有，会自动补全

## 架构设计

```
API Server
├── 路由注册 (pkg/apiserver/router.go)
│   ├── Core API v1 (Pods, Services, ConfigMaps, Secrets, Nodes)
│   └── Apps API v1 (Deployments, StatefulSets, DaemonSets)
├── 请求处理 (pkg/apiserver/handler.go)
│   ├── HandleCreate (POST)
│   ├── HandleGet (GET)
│   ├── HandleList (GET)
│   ├── HandleUpdate (PUT)
│   ├── HandlePatch (PATCH)
│   ├── HandleDelete (DELETE)
│   └── HandleWatch (GET /watch/...)
├── 存储集成 (pkg/storage)
│   ├── MemoryStore
│   ├── MySQLStore
│   └── EtcdStore
└── 容器管理 (internal/bootstrap)
    ├── 自动检测运行时
    ├── 自动拉起容器
    └── 优雅清理
```

## 注意事项

- **存储依赖**：API Server 需要连接到存储后端（memory/mysql/etcd）
- **端口配置**：默认端口为 `8080`，可通过配置文件修改
- **CORS 支持**：默认启用 CORS，可通过配置文件关闭
- **YAML 解析**：支持直接提交 YAML 格式的请求，会自动解析
- **资源版本**：每次更新资源时，`resourceVersion` 会自动递增
- **Watch 超时**：Watch 连接默认不会超时，客户端断开连接后会自动清理
- **容器自动管理**：
  - 仅当配置指向 `localhost` 时才会自动拉起容器
  - 如果容器已存在，不会重复启动
  - 如果未检测到 Docker，会跳过自动启动

## 与 kubectl 的兼容性

API Server 实现了 Kubernetes API 的核心功能，但并非完全兼容 `kubectl`。主要差异：

- **认证/授权**：当前不支持认证和授权机制
- **Admission Controllers**：不支持 admission controllers
- **资源验证**：基础的资源验证，但不包括完整的 Kubernetes 验证逻辑
- **Watch 实现**：使用 SSE 而非 WebSocket，与 Kubernetes 的 Watch 实现不同

## 参考文档

- 详细的 API 文档请参考：`pkg/apiserver/README.md`
- 存储层文档请参考：`pkg/storage/README.md`
