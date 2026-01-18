# Storage Layer

存储层支持多种存储后端，可以通过配置文件选择使用哪种存储实现。

## 支持的存储类型

- **memory**: 内存存储（默认，数据不持久化）
- **mysql**: MySQL 数据库存储（持久化）
- **etcd**: etcd 键值存储（持久化，支持分布式）

## 配置

在 `.config.yaml` 中配置存储类型和相关参数：

```yaml
storage:
  type: memory                # memory / mysql / etcd
  mysql:
    host: localhost
    port: 3306
    user: root
    password: password
    database: kubernetes
    max_open_conns: 100
    max_idle_conns: 10
  etcd:
    endpoints:
      - http://localhost:2379
    dial_timeout: 5s
    username: ""
    password: ""
```

## 存储实现

### Memory Store

内存存储，数据存储在进程内存中，重启后数据会丢失。

**优点**:
- 性能最高
- 无需外部依赖
- 适合开发和测试

**缺点**:
- 数据不持久化
- 不支持分布式

**使用场景**:
- 开发环境
- 测试环境
- 单机部署且不需要持久化

### MySQL Store

基于 MySQL 的关系型数据库存储，数据持久化到 MySQL。

**优点**:
- 数据持久化
- 支持事务
- 易于备份和恢复
- 支持 SQL 查询

**缺点**:
- 性能相对较低
- 需要维护 MySQL 实例
- 不适合高并发写入

**使用场景**:
- 生产环境（中小规模）
- 需要数据持久化
- 需要 SQL 查询能力

**数据库表结构**:

存储层会为每种资源类型自动创建独立的表，表名格式：`k8s_{group}_{version}_{kind}`

例如：
- `k8s_core_v1_pod` - Pod 资源表
- `k8s_apps_v1_deployment` - Deployment 资源表
- `k8s_core_v1_service` - Service 资源表
- `k8s_core_v1_configmap` - ConfigMap 资源表
- `k8s_core_v1_secret` - Secret 资源表

**基础字段**（所有资源表共有）:
- `id`: 主键
- `name`: 资源名称（索引）
- `namespace`: 命名空间（索引）
- `uid`: 唯一标识符（唯一索引）
- `resource_version`: 资源版本（索引）
- `labels`: 标签（JSON 格式）
- `annotations`: 注解（JSON 格式）
- `created_at`: 创建时间（索引）
- `updated_at`: 更新时间
- `deleted_at`: 软删除时间（索引）

**资源特定字段**:

**Pod 表** (`k8s_core_v1_pod`):
- `spec`: PodSpec 的 JSON
- `status`: PodStatus 的 JSON

**Deployment 表** (`k8s_apps_v1_deployment`):
- `replicas`: 副本数
- `replicas_available`: 可用副本数
- `replicas_ready`: 就绪副本数
- `replicas_updated`: 已更新副本数
- `strategy`: 部署策略（RollingUpdate, Recreate）
- `spec`: DeploymentSpec 的 JSON
- `status`: DeploymentStatus 的 JSON

**Service 表** (`k8s_core_v1_service`):
- `type`: 服务类型（ClusterIP, NodePort, LoadBalancer, ExternalName）（索引）
- `cluster_ip`: 集群 IP
- `ports`: ServicePort 数组的 JSON
- `spec`: ServiceSpec 的 JSON
- `status`: ServiceStatus 的 JSON

**ConfigMap 表** (`k8s_core_v1_configmap`):
- `data`: Data 字段的 JSON
- `binary_data`: BinaryData 字段的 JSON

**Secret 表** (`k8s_core_v1_secret`):
- `type`: Secret 类型（索引）
- `data`: Data 字段的 JSON（base64 编码的值）
- `string_data`: StringData 字段的 JSON

对于未定义具体表结构的资源类型，会使用基础表结构，完整对象数据存储在 `annotations` 字段中（JSON 格式）。

### Etcd Store

基于 etcd 的分布式键值存储，数据持久化到 etcd。

**优点**:
- 数据持久化
- 支持分布式
- 高性能
- 支持 watch 机制（原生支持）
- Kubernetes 原生使用 etcd

**缺点**:
- 需要维护 etcd 集群
- 配置相对复杂

**使用场景**:
- 生产环境（大规模）
- 需要分布式部署
- 需要高可用性
- 与 Kubernetes 集成

**键结构**:

资源在 etcd 中的键格式：
- 命名空间资源: `/kubernetes/{group}/{version}/{kind}/{namespace}/{name}`
- 集群资源: `/kubernetes/{group}/{version}/{kind}/{name}`

## 使用示例

### 切换到 MySQL 存储

1. 更新配置文件 `.config.yaml`:

```yaml
storage:
  type: mysql
  mysql:
    host: localhost
    port: 3306
    user: root
    password: your_password
    database: kubernetes
    max_open_conns: 100
    max_idle_conns: 10
```

2. 创建数据库:

```sql
CREATE DATABASE kubernetes CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

3. 重启应用，存储层会自动创建表结构。

### 切换到 etcd 存储

1. 启动 etcd（如果还没有）:

```bash
docker run -d \
  --name etcd \
  -p 2379:2379 \
  -p 2380:2380 \
  quay.io/coreos/etcd:v3.5.0 \
  /usr/local/bin/etcd \
  --name etcd \
  --data-dir /etcd-data \
  --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://localhost:2379 \
  --listen-peer-urls http://0.0.0.0:2380 \
  --initial-advertise-peer-urls http://localhost:2380 \
  --initial-cluster etcd=http://localhost:2380 \
  --initial-cluster-token my-etcd-token \
  --initial-cluster-state new
```

2. 更新配置文件 `.config.yaml`:

```yaml
storage:
  type: etcd
  etcd:
    endpoints:
      - http://localhost:2379
    dial_timeout: 5s
    username: ""
    password: ""
```

3. 重启应用。

## 存储接口

所有存储实现都实现了 `Store` 接口：

```go
type Store interface {
    Get(gvk schema.GroupVersionKind, namespace, name string) (runtime.Object, error)
    List(gvk schema.GroupVersionKind, namespace string) ([]runtime.Object, error)
    Create(gvk schema.GroupVersionKind, obj runtime.Object) error
    Update(gvk schema.GroupVersionKind, obj runtime.Object) error
    Delete(gvk schema.GroupVersionKind, namespace, name string) error
    Watch(gvk schema.GroupVersionKind, namespace string, resourceVersion string) (<-chan ResourceEvent, error)
}
```

## 性能对比

| 存储类型 | 读取性能 | 写入性能 | 持久化 | 分布式 | 适用场景 |
|---------|---------|---------|--------|--------|---------|
| Memory  | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ❌ | ❌ | 开发/测试 |
| MySQL   | ⭐⭐⭐ | ⭐⭐ | ✅ | ❌ | 中小规模生产 |
| Etcd    | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ✅ | ✅ | 大规模生产 |

## 注意事项

1. **数据迁移**: 切换存储类型时，需要手动迁移数据（当前不支持自动迁移）

2. **Watch 机制**: 
   - Memory 和 MySQL 使用内存中的事件通道实现 watch
   - Etcd 使用 etcd 原生的 watch 机制，性能更好

3. **资源版本**: 所有存储实现都支持 resourceVersion，但实现方式不同：
   - Memory: 使用递增的整数
   - MySQL: 使用时间戳（纳秒）
   - Etcd: 使用时间戳（纳秒）

4. **并发安全**: 所有存储实现都是线程安全的，支持并发访问

5. **事务支持**: 
   - Memory: 不支持事务
   - MySQL: 支持事务
   - Etcd: 支持事务（通过 etcd 的 Txn）

## 故障排查

### MySQL 连接失败

检查：
- MySQL 服务是否运行
- 连接参数是否正确（host、port、user、password）
- 数据库是否存在
- 网络连接是否正常

### Etcd 连接失败

检查：
- etcd 服务是否运行
- endpoints 配置是否正确
- 网络连接是否正常
- 认证信息是否正确（如果启用了认证）

### 数据丢失

- Memory 存储：重启应用后数据会丢失（预期行为）
- MySQL/Etcd 存储：检查存储服务是否正常运行，数据是否被意外删除
