# change.md

## 2026-01-18

- 新增 `cmd/network` 的 `export` 子命令：一次性导出局域网邻居表（ARP/neighbor table）为 JSON/YAML（IP + 名称 best-effort）。
- 更新 `cmd/network/readme.md`：补充 `export` 的用法与参数说明。
- 新增 `internal/network/service_test.go`：补充 `internal/network/service.go` 的基础单元测试，并已在本地通过 `go test ./...`。
- **破坏性变更**：简化 `cmd/network export` —— 不再做端口扫描/主动探测，改为读取系统 ARP/neighbor 表导出（仅输出同局域网内“已被系统学到”的邻居：IP + 名称 best-effort）。
- `cmd/network export` 新增 `--format cmd`：按行输出 `ip name mac`，便于命令行查看/复制。

