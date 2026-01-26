# KubernetesKubernetesKubernetes

  Chronon：连续是不连续的特殊状态，强关系是弱关系的特殊态。


KubernetesKubernetesKubernetes（简称k3） 是一个简化的 kubernetes 集群项目。
把 kubernetes 的架构简化为 apiserver -- controller -- storage 的形式，以一种兼容 kubernetes yaml 的形式，直接在单机/集群上面运行简化版本的 kubernetes。

按照我最终的设计理念，所有的组件都可以被替换，理想架构是单一组件的失败不会影响业务的整体运行。

Kubernetes 三倍の速度。

<a href="https://tamashiiweb.com/item/12870?wovn=en" target="_blank" rel="noopener">
  <img src="docs/item_0000012870_obE77vfs_01.jpg" alt="MS-06S">
</a>

## 命令行（cmd/k3）

项目提供统一的命令入口 `cmd/k3`，用于以“命令模式”管理各大模块（storage / controller / web），以及生成集群配置骨架。

### 启动（单进程：storage -> controller -> web）

```bash
go run ./cmd/k3 start
```

指定配置文件路径（等价于环境变量 `CONFIG_PATH`）：

```bash
go run ./cmd/k3 start --config .config.yaml
```

### 分模块启动

```bash
go run ./cmd/k3 storage
go run ./cmd/k3 controller
go run ./cmd/k3 web
```

### 创建 k3 集群配置骨架

生成多节点配置文件（写入到 `.k3/node-*/.config.yaml`）：

```bash
go run ./cmd/k3 cluster create --dir .k3 --nodes 3 --web-port 8080 --storage memory
```

然后选择一个节点配置启动（示例 node-1）：

```bash
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/k3 start
```

## TODO

动态路由注册与控制器解耦。
