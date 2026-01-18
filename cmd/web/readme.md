# cmd/web

`cmd/web` 是 k3 的 **可视化 Web 模块**，用于把当前进程内 `storage.Store` 中的资源以页面形式展示，并通过 **WebSocket** 实时推送 Nodes / Pods 的变化。

## 功能

- **实时看板**：动态展示 Node、Pod 列表与计数
- **推送机制**：服务端监听 `store.Watch(Node/Pod)`，有事件即推送最新快照到前端
- **前端样式**：使用 [Tailwind CSS](https://github.com/tailwindlabs/tailwindcss)

## 启动方式

在项目根目录执行：

```bash
go run ./cmd/web
```

默认读取 `.config.yaml`；若不存在，会回退读取 `configs/config-example.yaml`。也可以显式指定：

```bash
CONFIG_PATH=.config.yaml go run ./cmd/web
```

启动后访问：

- 页面：`http://localhost:<port>/`（默认 `8080`）

## 路由与协议

- **Dashboard 页面**
  - `GET /`（同 `GET /dashboard`）

- **WebSocket（资源快照推送）**
  - `GET /ws/resources`
  - 消息格式：JSON，`type = "snapshot"`，包含 `nodes[]`、`pods[]`、`counts`

## 数据来源说明（重要）

本项目的看板 **展示的是 k3 自己的“资源存储（Store）”**：

- Node：由 `internal/controller` 的节点上报/心跳逻辑写入
- Pod：由 `internal/controller` 的 scheduler / runtime 逻辑更新状态写入

也就是说，默认并不直接连接真实 Kubernetes 集群；而是展示该进程“当前持有的资源视图”。

## 生产化建议

当前前端为了开箱可用采用了 Tailwind CDN（`https://cdn.tailwindcss.com`），适合开发/演示。
生产环境建议改为 **Tailwind CLI/PostCSS 构建**，将产物打包为本地静态文件再由服务端提供。

