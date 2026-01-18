package storage

import (
	"fmt"

	"zeusro.com/hermes/internal/core/config"
)

// NewStore 根据配置创建存储实例
func NewStore(cfg config.StorageConfig) (Store, error) {
	switch cfg.Type {
	case "memory":
		return NewMemoryStore(), nil
	case "mysql":
		return NewMySQLStore(cfg.MySQL)
	case "etcd":
		return NewEtcdStore(cfg.Etcd)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
	}
}
