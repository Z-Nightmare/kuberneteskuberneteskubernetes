# Kubernetes API Server

这是一个基于 Kubernetes API server 架构实现的 RESTful API 服务器，支持 watch 和 write API。

## 功能特性

- ✅ **Write API**: 支持 Create、Update、Patch、Delete 操作
- ✅ **Read API**: 支持 Get、List 操作
- ✅ **Watch API**: 支持 Server-Sent Events (SSE) 实时监听资源变更
- ✅ **多资源类型支持**: Pod、Deployment、Service、ConfigMap、Secret、StatefulSet、DaemonSet 等
- ✅ **Namespace 支持**: 支持命名空间隔离的资源管理
- ✅ **内存存储**: 基于内存的高性能存储实现

## API 端点

### Core API v1

#### Pods
- `GET /api/v1/pods` - 列出所有 Pods
- `GET /api/v1/pods/:name` - 获取指定 Pod
- `POST /api/v1/pods` - 创建 Pod
- `PUT /api/v1/pods/:name` - 更新 Pod
- `PATCH /api/v1/pods/:name` - 部分更新 Pod
- `DELETE /api/v1/pods/:name` - 删除 Pod
- `GET /api/v1/watch/pods` - 监听 Pod 变更

#### Namespaced Pods
- `GET /api/v1/namespaces/:namespace/pods` - 列出指定命名空间的 Pods
- `GET /api/v1/namespaces/:namespace/pods/:name` - 获取指定命名空间的 Pod
- `POST /api/v1/namespaces/:namespace/pods` - 在指定命名空间创建 Pod
- `PUT /api/v1/namespaces/:namespace/pods/:name` - 更新指定命名空间的 Pod
- `PATCH /api/v1/namespaces/:namespace/pods/:name` - 部分更新 Pod
- `DELETE /api/v1/namespaces/:namespace/pods/:name` - 删除 Pod
- `GET /api/v1/watch/namespaces/:namespace/pods` - 监听指定命名空间的 Pod 变更

类似地，还支持 Services、ConfigMaps、Secrets 等资源。

### Apps API v1

#### Deployments
- `GET /apis/apps/v1/deployments` - 列出所有 Deployments
- `GET /apis/apps/v1/deployments/:name` - 获取指定 Deployment
- `POST /apis/apps/v1/deployments` - 创建 Deployment
- `PUT /apis/apps/v1/deployments/:name` - 更新 Deployment
- `PATCH /apis/apps/v1/deployments/:name` - 部分更新 Deployment
- `DELETE /apis/apps/v1/deployments/:name` - 删除 Deployment
- `GET /apis/apps/v1/watch/deployments` - 监听 Deployment 变更

类似地，还支持 StatefulSets、DaemonSets 等资源。

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
        "image": "nginx:1.21"
      }]
    }
  }'
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
        "image": "nginx:1.22"
      }]
    }
  }'
```

### 部分更新 Pod (PATCH)

```bash
curl -X PATCH http://localhost:8080/api/v1/namespaces/default/pods/nginx-pod \
  -H "Content-Type: application/json" \
  -d '{
    "spec": {
      "containers": [{
        "name": "nginx",
        "image": "nginx:1.23"
      }]
    }
  }'
```

### 删除 Pod

```bash
curl -X DELETE http://localhost:8080/api/v1/namespaces/default/pods/nginx-pod
```

### Watch Pod 变更

```bash
curl http://localhost:8080/api/v1/watch/namespaces/default/pods
```

Watch API 使用 Server-Sent Events (SSE) 格式返回事件流：

```
data: {"type":"ADDED","object":{...}}

data: {"type":"MODIFIED","object":{...}}

data: {"type":"DELETED","object":{...}}
```

### Watch 查询参数

- `resourceVersion`: 指定从哪个资源版本开始监听
- `timeoutSeconds`: 设置超时时间（秒）

示例：
```bash
curl "http://localhost:8080/api/v1/watch/pods?resourceVersion=100&timeoutSeconds=300"
```

## 事件类型

Watch API 支持以下事件类型：

- `ADDED`: 资源被创建
- `MODIFIED`: 资源被更新
- `DELETED`: 资源被删除
- `BOOKMARK`: 书签事件（用于标记特定资源版本）

## 存储实现

当前使用内存存储 (`MemoryStore`)，所有数据存储在内存中。重启服务后数据会丢失。

### 存储特性

- 线程安全：使用 `sync.RWMutex` 保护并发访问
- 资源版本管理：自动生成和管理 `resourceVersion`
- Watch 支持：支持多个客户端同时监听资源变更
- 命名空间隔离：支持命名空间级别的资源隔离

## 技术实现

- **存储层**: `pkg/storage` - 基于内存的资源存储实现
- **API 层**: `pkg/apiserver` - RESTful API handlers
- **路由**: 使用 Gin 框架注册 Kubernetes 风格的 API 路由
- **解析器**: 使用 `pkg/parser` 解析 YAML/JSON 格式的资源定义

## 与 Kubernetes 的兼容性

- ✅ 支持 Kubernetes 原生资源类型定义
- ✅ 兼容 Kubernetes API 路径格式
- ✅ 支持 Kubernetes 风格的 JSON/YAML 资源定义
- ✅ 支持 resourceVersion 管理
- ✅ 支持命名空间隔离

## 限制

- 当前使用内存存储，数据不持久化
- 不支持 etcd 等外部存储后端
- 不支持认证和授权
- 不支持 admission controllers
- 不支持多版本 API 转换

## 未来改进

- [ ] 支持 etcd 等持久化存储
- [ ] 实现认证和授权机制
- [ ] 支持 admission controllers
- [ ] 支持多版本 API 转换
- [ ] 实现 watch cache 优化
- [ ] 支持 label selector 和 field selector
