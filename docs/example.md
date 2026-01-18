# 最小例子

## 例子

目标：用 **k3（本项目）** 在本机跑通：

- storage 使用 **MySQL docker 镜像**（由 k3 自动拉起）
- 启动 apiserver + controller + web（dashboard）
- 用 **k3 命令提交 Deployment YAML**
- controller 监听 Deployment → 创建 Pod → 调度 → runtime 拉起容器（Docker）

### 0) 前置条件

- 已安装 Go（能 `go run`）
- Docker 可用且 daemon 在运行（能 `docker ps`）

### 1) 准备 Deployment YAML（nginx）

仓库已提供示例：`docs/nginx-deployment.yaml`

> 注意：若本机 Docker 无法从镜像仓库拉取（网络受限），请使用你本地已存在的镜像标签；该示例默认使用 `nginx:latest`。

### 2) 生成单节点配置（storage=mysql）

```bash
go run ./cmd/k3 cluster create --dir .k3 --nodes 1 --web-port 8080 --storage mysql
```

这会生成：`.k3/node-1/.config.yaml`

### 3) 启动 k3（单进程：storage -> controller -> web）

```bash
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/k3 start
```

启动后访问：

- dashboard：`http://localhost:8080/`

## 4) 提交 Deployment YAML 到 apiserver

另开一个终端执行：

```bash
CONFIG_PATH=.k3/node-1/.config.yaml go run ./cmd/k3 apply -f docs/nginx-deployment.yaml
```

## 验证事项

使用外部命令检查 Deployment 中关联的容器是否已经运行启动完毕。

### A) 验证 nginx 容器已启动（外部命令：docker）

```bash
docker ps --filter "name=k8s_default_nginx" --format "{{.Names}}\t{{.Image}}\t{{.Status}}"
```

期望：能看到 1 条（或多条）`nginx:...` 且 `Status` 为 `Up ...`。

### B) 验证 Pod 状态为 Running（apiserver）

```bash
curl http://localhost:8080/api/v1/namespaces/default/pods
```

期望：列表中存在 nginx 相关 Pod，且其 `status.phase` 为 `Running`。

## （可选）清理

```bash
curl -X DELETE http://localhost:8080/apis/apps/v1/namespaces/default/deployments/nginx
```

停止示例产生的容器（名称形如 `k8s_default_<pod>_nginx`）：

```bash
docker ps --filter "name=k8s_default_nginx" --format "{{.Names}}"
```
