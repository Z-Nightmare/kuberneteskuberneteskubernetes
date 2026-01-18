# 控制器模块

控制器模块实现了 Kubernetes 控制平面的核心功能，包括：

- **节点管理**：自动上报当前节点信息到存储
- **Pod 控制器**：管理 Pod 资源的生命周期，初始化 Pod 状态和条件
- **Deployment 控制器**：监听 Deployment 资源变化，自动创建/删除 Pod
- **Scheduler 控制器**：为 Pod 分配节点
- **容器运行时控制器**：自动检测并使用容器运行时启动容器

## 功能特性

### 1. 节点上报

控制器启动时会自动：
- 获取当前节点的主机名（或使用 `NODE_NAME` 环境变量）
- 创建或更新 Node 资源到存储
- 定期发送心跳更新节点状态

### 2. Pod 控制器

- 监听 Pod 资源的创建、更新、删除事件
- 初始化新创建的 Pod：
  - 设置初始状态为 `Pending`
  - 初始化 Pod 条件（Scheduled、Initialized、Ready）
  - 初始化容器状态
  - 设置 UID 和创建时间
- 管理 Pod 状态转换：
  - 根据容器状态更新 Pod 阶段（Pending、Running、Succeeded、Failed）
  - 更新 Pod 条件（Scheduled、Initialized、Ready）
- 处理 Pod 删除和清理

### 3. Deployment 控制器

- 监听 Deployment 资源的创建、更新、删除事件
- 根据 `spec.replicas` 自动创建或删除 Pod
- 维护 Pod 数量与期望副本数一致

### 4. Scheduler 控制器

- 监听 Pod 资源变化
- 为未调度的 Pod（`spec.nodeName` 为空）分配节点
- 简单的调度策略：选择第一个可用的就绪节点

### 5. 容器运行时控制器

- **自动检测容器运行时**：启动时自动检测环境中可用的容器运行时
- **优先级顺序**：Docker > Podman > Containerd > CRI-O
- **容器管理**：
  - 监听已调度到当前节点的 Pod
  - 自动启动容器（使用检测到的运行时）
  - 更新 Pod 状态为 Running
  - 处理 Pod 删除时停止容器

#### 支持的容器运行时

1. **Docker**（优先，已完整实现）
   - 自动检测 `docker` 命令和 Docker daemon
   - 支持启动、停止、查询容器状态
   - 支持环境变量、端口映射等配置

2. **Podman**（占位符，待实现）
   - 检测逻辑已实现
   - 容器操作待实现

3. **Containerd**（占位符，待实现）
   - 检测逻辑已实现
   - 容器操作待实现

4. **CRI-O**（占位符，待实现）
   - 检测逻辑已实现
   - 容器操作待实现

## 使用方法

### 启动控制器

```bash
# 使用默认配置
go run cmd/controller/main.go

# 指定节点名称
NODE_NAME=my-node go run cmd/controller/main.go
```

### 配置

控制器使用项目的统一配置文件 `.config.yaml`，需要配置存储后端：

```yaml
storage:
  type: mysql  # 或 memory / etcd
  mysql:
    host: localhost
    port: 3306
    user: root
    password: password
    database: kubernetes
```

## 架构设计

```
ControllerManager
├── PodController         (管理 Pod 生命周期，初始化状态)
├── DeploymentController  (监听 Deployment，创建 Pod)
├── SchedulerController   (调度 Pod 到节点)
├── RuntimeController     (启动容器，管理容器生命周期)
└── Node Heartbeat        (定期上报节点状态)
```

### 容器运行时检测流程

```
启动时检测
  ├─> 检查 Docker (docker info)
  │   └─> 可用 → 使用 DockerRuntime
  ├─> 检查 Podman (podman info)
  │   └─> 可用 → 使用 PodmanRuntime
  ├─> 检查 Containerd (ctr 命令)
  │   └─> 可用 → 使用 ContainerdRuntime
  └─> 检查 CRI-O (crictl 命令)
      └─> 可用 → 使用 CRIORuntime
```

## 扩展

### 添加新的控制器

1. 实现 `Controller` 接口：

```go
type MyController struct {
    store  storage.Store
    logger logprovider.Logger
}

func (c *MyController) Start(ctx context.Context) error {
    // 启动逻辑
}

func (c *MyController) Stop(ctx context.Context) error {
    // 停止逻辑
}

func (c *MyController) Name() string {
    return "MyController"
}
```

2. 在 `ControllerManager.registerControllers()` 中注册：

```go
func (cm *ControllerManager) registerControllers() {
    myController := NewMyController(cm.store, cm.logger)
    cm.controllers = append(cm.controllers, myController)
}
```

## 注意事项

- Node 资源没有 namespace，存储时会忽略 namespace 字段
- 当前调度器实现较为简单，生产环境需要更复杂的调度算法
- Deployment 控制器会维护 Pod 数量，但不会处理 Pod 的更新（需要 ReplicaSet 控制器）
- **容器运行时要求**：
  - 优先使用 Docker，确保 Docker daemon 正在运行
  - 如果 Docker 不可用，会自动尝试其他运行时
  - 如果所有运行时都不可用，容器运行时控制器将无法启动，但其他控制器仍可正常工作
- **Docker 运行时特性**：
  - 容器命名格式：`k8s_{namespace}_{pod-name}_{container-name}`
  - 支持环境变量、端口映射等基本配置
  - 容器状态会同步到 Pod 状态