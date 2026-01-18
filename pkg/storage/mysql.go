package storage

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"zeusro.com/hermes/internal/core/config"
	"zeusro.com/hermes/pkg/parser"
)

// MySQLStore 是基于 MySQL 的存储实现
type MySQLStore struct {
	db       *gorm.DB
	parser   *parser.Parser
	watchers map[string][]chan ResourceEvent
}

// NewMySQLStore 创建新的 MySQL 存储
func NewMySQLStore(cfg config.MySQLConfig) (*MySQLStore, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)

	store := &MySQLStore{
		db:       db,
		parser:   parser.NewParser(),
		watchers: make(map[string][]chan ResourceEvent),
	}

	return store, nil
}

// resourceKey 生成资源的唯一键
func (s *MySQLStore) resourceKey(gvk schema.GroupVersionKind, namespace, name string) string {
	if namespace == "" {
		return fmt.Sprintf("%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, name)
	}
	return fmt.Sprintf("%s/%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, namespace, name)
}

// watchKey 生成 watch 的键
func (s *MySQLStore) watchKey(gvk schema.GroupVersionKind, namespace string) string {
	if namespace == "" {
		return fmt.Sprintf("%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind)
	}
	return fmt.Sprintf("%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, namespace)
}

// Get 获取指定资源
func (s *MySQLStore) Get(gvk schema.GroupVersionKind, namespace, name string) (runtime.Object, error) {
	// 确保表存在
	if err := s.ensureTable(gvk); err != nil {
		return nil, err
	}

	// 根据资源类型使用不同的加载方法
	switch gvk.Kind {
	case "Pod":
		if gvk.Group == "" && gvk.Version == "v1" {
			return s.loadPod(gvk, namespace, name)
		}
	case "Deployment":
		if gvk.Group == "apps" && gvk.Version == "v1" {
			return s.loadDeployment(gvk, namespace, name)
		}
	case "Service":
		if gvk.Group == "" && gvk.Version == "v1" {
			return s.loadService(gvk, namespace, name)
		}
	case "Node":
		if gvk.Group == "" && gvk.Version == "v1" {
			return s.loadNode(gvk, namespace, name)
		}
	}

	// 通用资源加载
	return s.loadGenericResource(gvk, namespace, name)
}

// List 列出所有资源
func (s *MySQLStore) List(gvk schema.GroupVersionKind, namespace string) ([]runtime.Object, error) {
	// 确保表存在
	if err := s.ensureTable(gvk); err != nil {
		return nil, err
	}

	tableName := tableName(gvk)
	var bases []BaseResource

	query := s.db.Table(tableName)
	// Node 资源没有 namespace，忽略 namespace 参数
	if gvk.Kind != "Node" || gvk.Group != "" || gvk.Version != "v1" {
		if namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}
	}

	if err := query.Find(&bases).Error; err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	var objects []runtime.Object
	for _, base := range bases {
		obj, err := s.Get(gvk, base.Namespace, base.Name)
		if err != nil {
			continue
		}
		objects = append(objects, obj)
	}

	return objects, nil
}

// Create 创建资源
func (s *MySQLStore) Create(gvk schema.GroupVersionKind, obj runtime.Object) error {
	meta, err := getObjectMeta(obj)
	if err != nil {
		return err
	}

	namespace := meta.GetNamespace()
	name := meta.GetName()

	// 确保表存在
	if err := s.ensureTable(gvk); err != nil {
		return err
	}

	// 检查资源是否已存在（Node 资源允许更新，所以跳过检查）
	tableName := tableName(gvk)
	if gvk.Kind != "Node" || gvk.Group != "" || gvk.Version != "v1" {
		var count int64
		query := s.db.Table(tableName).Where("name = ? AND namespace = ?", name, namespace)
		if err := query.Count(&count).Error; err == nil && count > 0 {
			return fmt.Errorf("resource already exists: %s/%s", namespace, name)
		}
	}

	// 设置 resourceVersion
	resourceVersion := fmt.Sprintf("%d", time.Now().UnixNano())
	if meta.GetResourceVersion() == "" {
		meta.SetResourceVersion(resourceVersion)
	} else {
		resourceVersion = meta.GetResourceVersion()
	}

	// 设置创建时间
	if meta.GetCreationTimestamp().Time.IsZero() {
		meta.SetCreationTimestamp(metav1.NewTime(time.Now()))
	}

	// 设置 UID
	if meta.GetUID() == "" {
		meta.SetUID(types.UID(fmt.Sprintf("uid-%d", time.Now().UnixNano())))
	}

	// 根据资源类型使用不同的保存方法
	switch gvk.Kind {
	case "Pod":
		if gvk.Group == "" && gvk.Version == "v1" {
			if pod, ok := obj.(*corev1.Pod); ok {
				if err := s.savePod(pod); err != nil {
					return fmt.Errorf("failed to save pod: %w", err)
				}
				// 通知 watchers
				s.notifyWatchers(gvk, namespace, ResourceEvent{
					Type:   EventAdded,
					Object: obj,
				})
				return nil
			}
		}
	case "Deployment":
		if gvk.Group == "apps" && gvk.Version == "v1" {
			if deployment, ok := obj.(*appsv1.Deployment); ok {
				if err := s.saveDeployment(deployment); err != nil {
					return fmt.Errorf("failed to save deployment: %w", err)
				}
				// 通知 watchers
				s.notifyWatchers(gvk, namespace, ResourceEvent{
					Type:   EventAdded,
					Object: obj,
				})
				return nil
			}
		}
	case "Service":
		if gvk.Group == "" && gvk.Version == "v1" {
			if service, ok := obj.(*corev1.Service); ok {
				if err := s.saveService(service); err != nil {
					return fmt.Errorf("failed to save service: %w", err)
				}
				// 通知 watchers
				s.notifyWatchers(gvk, namespace, ResourceEvent{
					Type:   EventAdded,
					Object: obj,
				})
				return nil
			}
		}
	case "Node":
		if gvk.Group == "" && gvk.Version == "v1" {
			if node, ok := obj.(*corev1.Node); ok {
				if err := s.saveNode(node); err != nil {
					return fmt.Errorf("failed to save node: %w", err)
				}
				// 通知 watchers
				s.notifyWatchers(gvk, namespace, ResourceEvent{
					Type:   EventAdded,
					Object: obj,
				})
				return nil
			}
		}
	}

	// 通用资源保存
	if err := s.saveGenericResource(gvk, obj); err != nil {
		return fmt.Errorf("failed to save generic resource: %w", err)
	}

	// 通知 watchers
	s.notifyWatchers(gvk, namespace, ResourceEvent{
		Type:   EventAdded,
		Object: obj,
	})

	return nil
}

// Update 更新资源
func (s *MySQLStore) Update(gvk schema.GroupVersionKind, obj runtime.Object) error {
	meta, err := getObjectMeta(obj)
	if err != nil {
		return err
	}

	namespace := meta.GetNamespace()
	name := meta.GetName()

	// 确保表存在
	if err := s.ensureTable(gvk); err != nil {
		return err
	}

	// 获取旧资源
	oldObj, err := s.Get(gvk, namespace, name)
	if err != nil {
		return fmt.Errorf("resource not found: %w", err)
	}

	// 更新 resourceVersion
	resourceVersion := fmt.Sprintf("%d", time.Now().UnixNano())
	meta.SetResourceVersion(resourceVersion)

	// 先删除旧资源，再创建新资源（简化实现）
	tableName := tableName(gvk)
	query := s.db.Table(tableName)
	// Node 资源没有 namespace
	if gvk.Kind == "Node" && gvk.Group == "" && gvk.Version == "v1" {
		query = query.Where("name = ?", name)
	} else {
		query = query.Where("name = ? AND namespace = ?", name, namespace)
	}
	if err := query.Delete(&BaseResource{}).Error; err != nil {
		return fmt.Errorf("failed to delete old resource: %w", err)
	}

	// 重新创建资源
	if err := s.Create(gvk, obj); err != nil {
		return fmt.Errorf("failed to create updated resource: %w", err)
	}

	// 通知 watchers
	s.notifyWatchers(gvk, namespace, ResourceEvent{
		Type:   EventModified,
		Object: obj,
		OldObj: oldObj,
	})

	return nil
}

// Delete 删除资源
func (s *MySQLStore) Delete(gvk schema.GroupVersionKind, namespace, name string) error {
	// 获取资源（用于返回和通知）
	obj, err := s.Get(gvk, namespace, name)
	if err != nil {
		return fmt.Errorf("resource not found: %w", err)
	}

	// 删除资源
	tableName := tableName(gvk)
	query := s.db.Table(tableName)
	// Node 资源没有 namespace
	if gvk.Kind == "Node" && gvk.Group == "" && gvk.Version == "v1" {
		query = query.Where("name = ?", name)
	} else {
		query = query.Where("name = ? AND namespace = ?", name, namespace)
	}
	if err := query.Delete(&BaseResource{}).Error; err != nil {
		return fmt.Errorf("failed to delete resource: %w", err)
	}

	// 通知 watchers
	s.notifyWatchers(gvk, namespace, ResourceEvent{
		Type:   EventDeleted,
		Object: obj,
	})

	return nil
}

// Watch 监听资源变更
func (s *MySQLStore) Watch(gvk schema.GroupVersionKind, namespace string, resourceVersion string) (<-chan ResourceEvent, error) {
	watchKey := s.watchKey(gvk, namespace)
	ch := make(chan ResourceEvent, 100)

	// 注册 watcher
	if s.watchers[watchKey] == nil {
		s.watchers[watchKey] = make([]chan ResourceEvent, 0)
	}
	s.watchers[watchKey] = append(s.watchers[watchKey], ch)

	return ch, nil
}

// notifyWatchers 通知所有 watchers
func (s *MySQLStore) notifyWatchers(gvk schema.GroupVersionKind, namespace string, event ResourceEvent) {
	watchKey := s.watchKey(gvk, namespace)
	watchers := s.watchers[watchKey]

	// 也通知全局 watchers
	if namespace != "" {
		globalKey := s.watchKey(gvk, "")
		watchers = append(watchers, s.watchers[globalKey]...)
	}

	for _, ch := range watchers {
		select {
		case ch <- event:
		default:
			// 如果通道已满，跳过
		}
	}
}

// Close 关闭 MySQL 连接
func (s *MySQLStore) Close() error {
	if s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}
	return sqlDB.Close()
}
