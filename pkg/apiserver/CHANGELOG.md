# Changelog - Kubernetes API Server

## 2025-01-18 - Kubernetes API Server 实现

### 新增功能

1. **存储层** (`pkg/storage`)
   - 基于内存的资源存储实现 (`MemoryStore`)
   - 支持 CRUD 操作（Create、Read、Update、Delete）
   - 支持资源版本管理（resourceVersion）
   - 支持 Watch 事件通知机制
   - 线程安全的并发访问控制
   - 命名空间隔离支持

2. **API Server** (`pkg/apiserver`)
   - RESTful API handlers 实现
   - 支持 Kubernetes 风格的 API 路径
   - 支持所有 HTTP verbs（GET、POST、PUT、PATCH、DELETE）
   - Watch API 实现（Server-Sent Events）
   - 自动解析 GroupVersionKind
   - 支持命名空间和资源名称解析

3. **路由注册** (`pkg/apiserver/router.go`)
   - 自动注册 Core API v1 路由（Pods、Services、ConfigMaps、Secrets）
   - 自动注册 Apps API v1 路由（Deployments、StatefulSets、DaemonSets）
   - 支持命名空间级别的资源路由
   - 支持 Watch 路由

### API 端点

#### Core API v1
- Pods: `/api/v1/pods`, `/api/v1/namespaces/:namespace/pods`
- Services: `/api/v1/services`, `/api/v1/namespaces/:namespace/services`
- ConfigMaps: `/api/v1/configmaps`, `/api/v1/namespaces/:namespace/configmaps`
- Secrets: `/api/v1/secrets`, `/api/v1/namespaces/:namespace/secrets`

#### Apps API v1
- Deployments: `/apis/apps/v1/deployments`, `/apis/apps/v1/namespaces/:namespace/deployments`
- StatefulSets: `/apis/apps/v1/statefulsets`, `/apis/apps/v1/namespaces/:namespace/statefulsets`
- DaemonSets: `/apis/apps/v1/daemonsets`, `/apis/apps/v1/namespaces/:namespace/daemonsets`

### 功能特性

- ✅ **Write API**: Create、Update、Patch、Delete
- ✅ **Read API**: Get、List
- ✅ **Watch API**: Server-Sent Events 实时监听
- ✅ **资源版本管理**: 自动生成和管理 resourceVersion
- ✅ **命名空间支持**: 完整的命名空间隔离
- ✅ **事件通知**: ADDED、MODIFIED、DELETED、BOOKMARK 事件类型

### 技术实现

- **存储**: 基于内存的线程安全存储实现
- **API**: 使用 Gin 框架实现 RESTful API
- **Watch**: 使用 Server-Sent Events (SSE) 实现事件流
- **解析**: 集成 `pkg/parser` 解析 YAML/JSON 资源定义
- **类型**: 使用 Kubernetes 官方类型定义（k8s.io/api）

### 测试覆盖

- ✅ Create/Get 操作测试
- ✅ Update 操作测试
- ✅ Delete 操作测试
- ✅ List 操作测试
- ✅ Watch 事件监听测试

### 集成

- 已集成到主应用 (`cmd/apiserver/main.go`)
- 使用 fx 依赖注入框架
- 自动注册到 Gin 路由

### 限制

- 当前使用内存存储，数据不持久化
- 不支持 etcd 等外部存储
- 不支持认证和授权
- 不支持 admission controllers

### 未来计划

- [ ] 支持 etcd 持久化存储
- [ ] 实现认证和授权机制
- [ ] 支持 admission controllers
- [ ] 实现 watch cache 优化
- [ ] 支持 label selector 和 field selector
- [ ] 支持分页和限制查询
