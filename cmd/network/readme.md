# network 模块

`cmd/network` 是一个“局域网节点发现/注册”模块：它通过 **mDNS（zeroconf）** 在同一局域网内广播自己并发现其他节点，然后将发现到的节点以 **`corev1.Node`** 的形式写入共享的 **`storage.Store`**（etcd/mysql/memory），从而让 controller / scheduler / dashboard 能看到“当前有哪些节点”。

## 目标与边界

- **目标**：自动发现局域网内运行了 `cmd/network` 的节点，并把它们注册到 node 列表（`v1/Node`）。
- **边界**：
  - 不做“全网段 IP 扫描”；当前只做 **mDNS 广播/发现 + TCP 探测**。
  - 不负责真正节点的 kubelet 上报；若 controller 已上报节点，本模块不会覆盖。

## 实现概览

核心实现位于 `internal/network/service.go`，整体流程如下：

1. **启动本地 health server**
   - 监听 `--listen`（默认 `:7946`）
   - 提供：
     - `GET /healthz`：返回 `ok`
     - `GET /info`：返回本节点信息（node/port/pid/addrs 等）

2. **mDNS 广播（Advertise）**
   - 使用 `zeroconf.Register(instance, service, domain, port, txt, ifaces)`
   - 默认：
     - service：`_k3._tcp`
     - domain：`local.`
   - TXT 里带上 `node=...`、`port=...`、`pid=...`

3. **mDNS 发现（Browse）**
   - 使用 `zeroconf.NewResolver().Browse(...)` 订阅局域网内同 service 的条目
   - 每次收到 entry：
     - 解析 peer 的 instance/TXT 得到 peer `nodeName`
     - 收集 peer 的 `AddrIPv4/AddrIPv6` + `port`
     - 触发一次 “upsert Node 到 store”

4. **写入 Node（Upsert 到 storage.Store）**
   - 以 `schema.GroupVersionKind{Version:"v1", Kind:"Node"}` 写入
   - 只管理带有标签 `k3.network/managed=true` 的 Node：
     - 如果 store 里已经存在同名 Node，但 **不是 managed**，则 **不覆盖**（避免和 controller 的真实上报冲突）
     - 如果不存在，则创建一个 managed Node
   - Node 字段策略（简化）：
     - `metadata.name = peer node name`
     - `status.addresses`：hostname + 内网 IP（v4 优先，过滤 link-local v6）
     - `status.conditions[NodeReady]`：基于探测结果设置 Ready/NotReady
     - `metadata.annotations`：记录 `k3.network/lastSeen`、`k3.network/port`、`k3.network/pid`

5. **存活探测 + 过期处理**
   - 定期对已知 peer 做 TCP 探测（连 `peer_ip:peer_port`）
   - 探测成功：标记 Ready，并刷新 lastSeen
   - 超过 `--peer-ttl`（默认 90s）仍不可达：标记 NotReady（仅 managed 节点）

## 启动方式

### 1) 使用默认配置启动

```bash
go run ./cmd/network
```

### 2) 指定配置文件（连接同一份 store 才能共享节点列表）

```bash
go run ./cmd/network --config .config.yaml
```

## 参数说明

- `--config <path>`：配置文件路径（等价于环境变量 `CONFIG_PATH`），用于连接 store（etcd/mysql/memory）
- `--listen <addr>`：health server 监听地址，同时作为 mDNS 广播端口（默认 `:7946`）
- `--service <name>`：mDNS service 名（默认 `_k3._tcp`）
- `--domain <name>`：mDNS domain（默认 `local.`）
- `--node-name <name>`：本机节点名（默认取 `NODE_NAME` 环境变量，否则 hostname）
- `--peer-ttl <duration>`：peer 过期时间（默认 90s；超时标记 NotReady）
- `--register-self`：是否也把本机注册为 Node（默认 true；若 controller 已上报该节点，不会覆盖）

## 注意事项

- 若使用 `storage.type=memory`，各进程内存不共享，无法形成“多节点视角”；要共享 node 列表请使用 `mysql/etcd`。
- mDNS 通常要求节点在同一二层网络/同一广播域；跨网段需要额外机制（后续可以扩展为 CIDR 扫描或中心注册）。

