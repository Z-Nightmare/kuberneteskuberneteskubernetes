# Kubernetes 资源示例

本目录包含 Kubernetes 内置资源的 YAML 示例，按照 API 分组组织。

## 目录结构

```
example/
├── core-v1/          # Core API v1 资源
│   ├── pod.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   ├── secret.yaml
│   └── node.yaml
└── apps-v1/          # Apps API v1 资源
    ├── deployment.yaml
    ├── statefulset.yaml
    └── daemonset.yaml
```

## 使用方法

### 使用 k3 apply 命令提交

```bash
# 提交单个资源
go run ./cmd/k3 apply -f example/core-v1/pod.yaml

# 提交整个目录下的所有资源（需要逐个文件）
go run ./cmd/k3 apply -f example/core-v1/pod.yaml
go run ./cmd/k3 apply -f example/core-v1/service.yaml
go run ./cmd/k3 apply -f example/apps-v1/deployment.yaml
```

### 使用 curl 直接提交到 apiserver

```bash
# 提交 Pod
curl -X POST http://localhost:8080/api/v1/namespaces/default/pods \
  -H "Content-Type: application/yaml" \
  --data-binary @example/core-v1/pod.yaml

# 提交 Service
curl -X POST http://localhost:8080/api/v1/namespaces/default/services \
  -H "Content-Type: application/yaml" \
  --data-binary @example/core-v1/service.yaml

# 提交 Deployment
curl -X POST http://localhost:8080/apis/apps/v1/namespaces/default/deployments \
  -H "Content-Type: application/yaml" \
  --data-binary @example/apps-v1/deployment.yaml
```

## 资源说明

### Core API v1

#### Pod
- **文件**: `core-v1/pod.yaml`
- **说明**: 最基本的运行单元，包含一个 nginx 容器
- **特性**: 环境变量、资源限制、端口映射

#### Service
- **文件**: `core-v1/service.yaml`
- **说明**: 为 Pod 提供稳定的网络访问入口
- **类型**: ClusterIP（集群内部访问）

#### ConfigMap
- **文件**: `core-v1/configmap.yaml`
- **说明**: 存储非敏感配置数据
- **用途**: 应用配置、nginx 配置、属性文件

#### Secret
- **文件**: `core-v1/secret.yaml`
- **说明**: 存储敏感信息（密码、API 密钥等）
- **编码**: data 字段使用 base64，stringData 字段使用明文（会自动编码）

#### Node
- **文件**: `core-v1/node.yaml`
- **说明**: 集群节点资源（cluster-scoped，无 namespace）
- **注意**: 通常由 kubelet 自动创建和管理

### Apps API v1

#### Deployment
- **文件**: `apps-v1/deployment.yaml`
- **说明**: 管理无状态应用的副本集
- **特性**: 滚动更新、健康检查、资源限制

#### StatefulSet
- **文件**: `apps-v1/statefulset.yaml`
- **说明**: 管理有状态应用，提供稳定的网络标识和存储
- **特性**: 有序部署、持久化存储

#### DaemonSet
- **文件**: `apps-v1/daemonset.yaml`
- **说明**: 确保每个节点运行一个 Pod 副本
- **用途**: 日志收集、监控代理、网络插件

## 注意事项

1. **镜像标签**: 示例中使用 `nginx:latest`，实际使用时建议使用具体版本标签
2. **资源限制**: 示例中的资源限制（CPU/内存）可根据实际需求调整
3. **命名空间**: 所有示例默认使用 `default` 命名空间，可根据需要修改
4. **Secret 编码**: Secret 的 `data` 字段需要 base64 编码，可使用以下命令：
   ```bash
   echo -n 'your-value' | base64
   ```
5. **Node 资源**: Node 是集群级别资源，通常不需要手动创建，由 kubelet 自动上报

## 完整示例：部署 Web 应用

```bash
# 1. 创建 ConfigMap（应用配置）
go run ./cmd/k3 apply -f example/core-v1/configmap.yaml

# 2. 创建 Secret（敏感信息）
go run ./cmd/k3 apply -f example/core-v1/secret.yaml

# 3. 创建 Deployment（应用实例）
go run ./cmd/k3 apply -f example/apps-v1/deployment.yaml

# 4. 创建 Service（服务暴露）
go run ./cmd/k3 apply -f example/core-v1/service.yaml
```

## 验证资源

```bash
# 查看 Pods
curl http://localhost:8080/api/v1/namespaces/default/pods

# 查看 Services
curl http://localhost:8080/api/v1/namespaces/default/services

# 查看 Deployments
curl http://localhost:8080/apis/apps/v1/namespaces/default/deployments

# 查看 ConfigMaps
curl http://localhost:8080/api/v1/namespaces/default/configmaps

# 查看 Secrets
curl http://localhost:8080/api/v1/namespaces/default/secrets
```
