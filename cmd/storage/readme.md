# cmd/storage

`cmd/storage` 是 k3 的**存储服务模块**，负责启动和管理存储后端（memory/mysql/etcd），为其他模块提供数据持久化能力。

## 功能

- **存储后端管理**：支持三种存储类型（memory、mysql、etcd）
- **自动容器管理**：当配置指向 `localhost` 时，自动拉起 MySQL/etcd Docker 容器
- **优雅停机**：退出时自动清理存储连接和由本进程启动的容器
- **连接重试**：MySQL 容器启动后自动重试连接，确保数据库完全就绪

## 启动方式

在项目根目录执行：

```bash
go run ./cmd/storage
```

默认读取 `.config.yaml`；若不存在，会回退读取 `configs/config-example.yaml`。也可以显式指定：

```bash
CONFIG_PATH=config-example.yaml go run ./cmd/storage
```

## 配置说明

### 存储类型

支持三种存储类型：

1. **memory**（内存存储）
   - 数据存储在内存中，进程退出后丢失
   - 适用于单进程测试、快速原型
   - 无需额外配置

2. **mysql**（MySQL 存储）
   - 数据持久化到 MySQL 数据库
   - 支持多进程共享数据
   - 适用于生产环境、多节点集群

3. **etcd**（Etcd 存储）
   - 数据存储在 etcd 分布式键值存储
   - 支持分布式部署和高可用
   - 适用于大规模生产环境

### 自动容器管理

当存储配置指向 `localhost`（或 `127.0.0.1`、`::1`）时，存储模块会：

1. **自动检测容器运行时**：优先使用 Docker，如果不可用则跳过自动启动
2. **检查容器是否已存在**：如果容器已在运行，则不会重复启动
3. **自动拉起容器**：
   - MySQL：使用 `mysql:8.0` 镜像
   - Etcd：使用 `quay.io/coreos/etcd:v3.5.0` 镜像
4. **连接重试**：MySQL 容器启动后，会重试连接（最多 45 秒），确保数据库完全就绪
5. **自动清理**：进程退出时，如果容器由本进程启动，会自动停止并删除

### 配置示例

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

## 使用场景

### 1. 独立存储服务

在多进程部署中，可以单独运行存储服务：

```bash
# 终端 1：启动存储服务
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/storage

# 终端 2：启动控制器（连接到同一存储）
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/controller

# 终端 3：启动 Web 服务（连接到同一存储）
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/web
```

### 2. 集成到 k3 命令

`cmd/k3` 的 `start`、`storage`、`controller`、`web` 子命令都会自动启动存储服务。

## 日志输出

启动成功后会输出存储类型和连接信息：

```
正在启动存储服务 (类型: mysql)...
MySQL 存储已连接: localhost:3306/k3
存储服务启动完成
```

## 注意事项

- **存储类型选择**：
  - 单进程测试：使用 `memory`
  - 多进程/生产环境：使用 `mysql` 或 `etcd`
- **容器自动管理**：
  - 仅当配置指向 `localhost` 时才会自动拉起容器
  - 如果容器已存在，不会重复启动
  - 如果未检测到 Docker，会跳过自动启动（允许用户手动启动数据库）
- **数据持久化**：
  - `memory` 类型数据不持久化，进程退出后丢失
  - `mysql` 和 `etcd` 类型数据持久化，进程退出后数据保留
- **优雅停机**：
  - 收到 SIGINT/SIGTERM/SIGQUIT 信号时，会优雅关闭存储连接
  - 如果容器由本进程启动，会自动清理容器

## 架构设计

```
Storage Service
├── 存储后端初始化
│   ├── MemoryStore (内存)
│   ├── MySQLStore (MySQL + GORM)
│   └── EtcdStore (Etcd v3)
├── 容器自动管理
│   ├── 检测容器运行时 (Docker/Podman/Containerd/CRI-O)
│   ├── 检查容器状态
│   ├── 自动拉起容器 (MySQL/Etcd)
│   └── 连接重试机制
└── 生命周期管理
    ├── 启动时初始化存储
    └── 退出时清理连接和容器
```
