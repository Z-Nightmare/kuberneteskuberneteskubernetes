# change.md

## cmd/k3 run 命令：根据角色启动不同模式

2026-01-18

- 新增 `cmd/k3 run` 命令：根据配置文件中的 `role` 字段启动不同的模块组合。
  - **master 模式**：启动 apiserver、storage、discovery 模块（适用于控制平面节点）
  - **node 模式**：启动 controller 模块（适用于工作节点）
  - **one 模式**：启动 apiserver、storage、discovery、controller 模块（适用于单节点部署）
- 在 `internal/core/config/config.go` 中添加 `Role` 字段，支持从配置文件读取角色配置。
- 更新 `cmd/k3/config-example.yaml`：添加 `role` 配置示例。
- 新增三个配置示例文件：
  - `cmd/k3/config-master-example.yaml` - master 模式配置示例
  - `cmd/k3/config-node-example.yaml` - node 模式配置示例
  - `cmd/k3/config-one-example.yaml` - one 模式配置示例
- 更新 `cmd/k3/readme.md`：添加 `run` 命令的详细说明，包括使用示例、配置说明、部署场景和注意事项。
- discovery 模块配置支持通过环境变量设置（`CONSUL_ADDR`、`CONSUL_TOKEN`、`SERVICE_NAME`、`NODE_NAME`）。

## cmd/network export 子命令功能增强

2026-01-18

- 新增 `cmd/network` 的 `export` 子命令：一次性导出局域网邻居表（ARP/neighbor table）为 JSON/YAML（IP + 名称 best-effort）。
- 更新 `cmd/network/readme.md`：补充 `export` 的用法与参数说明。
- 新增 `internal/network/service_test.go`：补充 `internal/network/service.go` 的基础单元测试，并已在本地通过 `go test ./...`。
- **破坏性变更**：简化 `cmd/network export` —— 不再做端口扫描/主动探测，改为读取系统 ARP/neighbor 表导出（仅输出同局域网内"已被系统学到"的邻居：IP + 名称 best-effort）。
- `cmd/network export` 新增 `--format cmd`：按行输出 `ip name mac`，便于命令行查看/复制。

## cmd/k3 cluster clear 命令

2026-01-18

- 新增 `cmd/k3 cluster clear` 命令：删除 k3 集群配置目录以及关联的 Docker 容器。
  - 功能：自动清理集群配置目录（默认 `.k3/`）和所有以 `k8s_` 开头的容器（包括存储容器和 Pod 容器）。
  - 安全特性：默认需要确认（输入 `yes`），支持 `--force` 跳过确认，禁止删除危险路径。
  - 参数：`--dir <path>` 指定要删除的目录（默认 `.k3`），`--force` 强制删除不询问。
  - 更新 `cmd/k3/readme.md`：补充 `cluster clear` 的详细用法、参数说明和使用示例。
