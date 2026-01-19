# cmd/controller

`cmd/controller` 是 k3 的**控制器模块**，负责运行 Kubernetes 控制平面的核心控制器，管理集群资源的生命周期。

## 功能

- **节点管理**：自动上报当前节点信息到存储，定期发送心跳
- **Pod 控制器**：管理 Pod 资源的生命周期，初始化 Pod 状态和条件
- **Deployment 控制器**：监听 Deployment 资源变化，自动创建/删除 Pod
- **Scheduler 控制器**：为 Pod 分配节点
- **容器运行时控制器**：自动检测并使用容器运行时启动容器

## 启动方式

在项目根目录执行：

```bash
go run ./cmd/controller
```

默认读取 `.config.yaml`；若不存在，会回退读取 `configs/config-example.yaml`。也可以显式指定：

```bash
CONFIG_PATH=config-example.yaml go run ./cmd/controller
```

### 指定节点名称

通过环境变量 `NODE_NAME` 指定节点名称（否则使用系统 hostname）：

```bash
NODE_NAME=my-node-1 go run ./cmd/controller
```

## 控制器说明

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
- 使用 `selector.matchLabels` 识别属于该 Deployment 的 Pod

### 4. Scheduler 控制器

- 监听 Pod 资源变化
- 为未调度的 Pod（`spec.nodeName` 为空）分配节点
- 简单的调度策略：选择第一个可用的就绪节点
- 更新 Pod 的 `spec.nodeName` 字段

### 5. 容器运行时控制器

- **自动检测容器运行时**：启动时自动检测环境中可用的容器运行时
- **优先级顺序**：Docker > Podman > Containerd > CRI-O
- **容器管理**：
  - 监听已调度到当前节点的 Pod
  - 自动启动容器（使用检测到的运行时）
  - 更新 Pod 状态为 Running
  - 处理 Pod 删除时停止容器
  - 支持超时机制（启动 2 分钟，停止 30 秒）

#### 支持的容器运行时

1. **Docker**（优先，已完整实现）
   - 自动检测 `docker` 命令和 Docker daemon
   - 支持启动、停止、查询容器状态
   - 支持环境变量、端口映射等配置
   - 容器命名格式：`k8s_{namespace}_{pod-name}_{container-name}`

2. **Podman**（占位符，待实现）
   - 检测逻辑已实现
   - 容器操作待实现

3. **Containerd**（占位符，待实现）
   - 检测逻辑已实现
   - 容器操作待实现

4. **CRI-O**（占位符，待实现）
   - 检测逻辑已实现
   - 容器操作待实现

## 配置说明

控制器需要配置存储后端（与 `cmd/storage` 使用相同的配置）：

```yaml
storage:
  type: mysql   # memory / mysql / etcd
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
```

**注意**：控制器模块**不包含**存储后端的自动容器管理功能。如果需要自动拉起 MySQL/etcd 容器，请先运行 `cmd/storage` 或使用 `cmd/k3` 的集成命令。

## 使用场景

### 1. 独立控制器服务

在多进程部署中，可以单独运行控制器服务：

```bash
# 终端 1：启动存储服务
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/storage

# 终端 2：启动控制器（连接到同一存储）
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/controller
```

### 2. 多节点集群

在不同节点上运行控制器，连接到共享存储：

```bash
# 节点 1
NODE_NAME=node-1 CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/controller

# 节点 2
NODE_NAME=node-2 CONFIG_PATH=.k3/node-2/.config.yaml go run ./cmd/controller
```

### 3. 集成到 k3 命令

`cmd/k3` 的 `start`、`controller`、`web` 子命令都会自动启动控制器。

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

## 工作流程示例

1. **创建 Deployment**：
   ```
   API Server → 存储 → Deployment 控制器检测到新 Deployment
   → 根据 replicas 创建 Pod → Pod 控制器初始化 Pod 状态
   ```

2. **调度 Pod**：
   ```
   Pod 控制器创建 Pod → Scheduler 控制器检测到未调度的 Pod
   → 选择节点并更新 spec.nodeName
   ```

3. **启动容器**：
   ```
   Scheduler 分配节点 → Runtime 控制器检测到已调度到本节点的 Pod
   → 启动容器 → 更新 Pod 状态为 Running
   ```

## 注意事项

- **存储依赖**：控制器需要连接到存储后端（memory/mysql/etcd）
- **节点名称**：通过 `NODE_NAME` 环境变量指定，否则使用系统 hostname
- **容器运行时**：
  - 优先使用 Docker，确保 Docker daemon 正在运行
  - 如果 Docker 不可用，会自动尝试其他运行时
  - 如果所有运行时都不可用，容器运行时控制器将无法启动，但其他控制器仍可正常工作
- **调度策略**：当前调度器实现较为简单，生产环境需要更复杂的调度算法
- **Deployment 控制器**：
  - 会维护 Pod 数量，但不会处理 Pod 的更新（需要 ReplicaSet 控制器）
  - 使用 `selector.matchLabels` 识别 Pod，不依赖 `OwnerReferences`
- **超时保护**：容器启动和停止操作都有超时保护，避免无限等待

## 扩展

### 添加新的控制器

1. 实现 `Controller` 接口（参考 `internal/controller/README.md`）
2. 在 `ControllerManager.registerControllers()` 中注册

详细说明请参考 `internal/controller/README.md`。
