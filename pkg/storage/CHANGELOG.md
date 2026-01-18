# Changelog - Storage Layer

## 2025-01-18 - 多存储后端支持

### 新增功能

1. **MySQL 存储实现** (`pkg/storage/mysql.go`)
   - 基于 GORM 和 MySQL 的持久化存储
   - 自动创建数据库表结构
   - 支持连接池配置
   - 完整的 CRUD 操作支持
   - Watch 事件通知机制

2. **Etcd 存储实现** (`pkg/storage/etcd.go`)
   - 基于 etcd client v3 的分布式存储
   - 支持 etcd 原生的 watch 机制
   - 高性能的键值存储
   - 支持分布式部署
   - 自动监听 etcd 变更事件

3. **存储工厂** (`pkg/storage/factory.go`)
   - 统一的存储创建接口
   - 根据配置自动选择存储实现
   - 支持动态切换存储类型

4. **配置支持** (`internal/core/config/config.go`)
   - 添加 `StorageConfig` 配置结构
   - 支持 MySQL 配置参数
   - 支持 etcd 配置参数
   - 配置文件示例更新

### 配置示例

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

### 存储实现对比

| 特性 | Memory | MySQL | Etcd |
|------|--------|-------|------|
| 持久化 | ❌ | ✅ | ✅ |
| 分布式 | ❌ | ❌ | ✅ |
| 性能 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| 事务支持 | ❌ | ✅ | ✅ |
| Watch 性能 | ⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| 适用场景 | 开发/测试 | 中小规模生产 | 大规模生产 |

### 技术实现

- **MySQL**: 使用 GORM ORM 框架，自动迁移表结构
- **Etcd**: 使用 etcd client v3，支持原生 watch 机制
- **工厂模式**: 统一的存储接口，便于扩展
- **配置驱动**: 通过配置文件选择存储类型

### 数据库表结构

MySQL 存储使用 `resource_records` 表，包含：
- 资源标识（Group、Version、Kind、Namespace、Name）
- 资源数据（JSON 格式）
- 资源版本（resourceVersion）
- 时间戳（created_at、updated_at）

### Etcd 键结构

- 命名空间资源: `/kubernetes/{group}/{version}/{kind}/{namespace}/{name}`
- 集群资源: `/kubernetes/{group}/{version}/{kind}/{name}`

### 使用方式

存储类型通过配置文件选择，应用启动时自动创建对应的存储实例：

```go
// 在 pkg/apiserver/module.go 中
store, err := storage.NewStore(cfg.Storage)
if err != nil {
    panic(fmt.Sprintf("Failed to create store: %v", err))
}
```

### 测试

- ✅ Memory Store 所有测试通过
- ✅ 存储工厂功能正常
- ✅ 配置解析正常

### 未来改进

- [ ] 支持数据迁移工具
- [ ] 支持存储后端健康检查
- [ ] 支持存储后端故障转移
- [ ] 支持存储后端性能监控
- [ ] 支持存储后端连接池监控
