package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"zeusro.com/hermes/internal/core/config"
	"zeusro.com/hermes/pkg/parser"
)

// EtcdStore 是基于 etcd 的存储实现
type EtcdStore struct {
	client   *clientv3.Client
	parser   *parser.Parser
	watchers map[string][]chan ResourceEvent
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewEtcdStore 创建新的 etcd 存储
func NewEtcdStore(cfg config.EtcdConfig) (*EtcdStore, error) {
	dialTimeout := 5 * time.Second
	if cfg.DialTimeout != "" {
		var err error
		dialTimeout, err = time.ParseDuration(cfg.DialTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid dial_timeout: %w", err)
		}
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: dialTimeout,
		Username:    cfg.Username,
		Password:    cfg.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	store := &EtcdStore{
		client:   client,
		parser:   parser.NewParser(),
		watchers: make(map[string][]chan ResourceEvent),
		ctx:      ctx,
		cancel:   cancel,
	}

	// 启动 watch 监听器
	go store.startWatcher()

	return store, nil
}

// resourceKey 生成 etcd 中的资源键
func (s *EtcdStore) resourceKey(gvk schema.GroupVersionKind, namespace, name string) string {
	if namespace == "" {
		return fmt.Sprintf("/kubernetes/%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, name)
	}
	return fmt.Sprintf("/kubernetes/%s/%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, namespace, name)
}

// watchKey 生成 watch 的键前缀
func (s *EtcdStore) watchKey(gvk schema.GroupVersionKind, namespace string) string {
	if namespace == "" {
		return fmt.Sprintf("/kubernetes/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind)
	}
	return fmt.Sprintf("/kubernetes/%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, namespace)
}

// Get 获取指定资源
func (s *EtcdStore) Get(gvk schema.GroupVersionKind, namespace, name string) (runtime.Object, error) {
	key := s.resourceKey(gvk, namespace, name)

	resp, err := s.client.Get(context.Background(), key)
	if err != nil {
		return nil, fmt.Errorf("failed to get from etcd: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("resource not found: %s/%s", namespace, name)
	}

	// 解析数据
	obj, _, err := s.parser.ParseYAML(resp.Kvs[0].Value)
	if err != nil {
		return nil, fmt.Errorf("failed to parse resource data: %w", err)
	}

	return obj, nil
}

// List 列出所有资源
func (s *EtcdStore) List(gvk schema.GroupVersionKind, namespace string) ([]runtime.Object, error) {
	prefix := s.watchKey(gvk, namespace)

	resp, err := s.client.Get(context.Background(), prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list from etcd: %w", err)
	}

	var objects []runtime.Object
	for _, kv := range resp.Kvs {
		obj, _, err := s.parser.ParseYAML(kv.Value)
		if err != nil {
			continue
		}

		// 如果指定了 namespace，过滤
		if namespace != "" {
			meta, err := getObjectMeta(obj)
			if err == nil && meta.GetNamespace() != namespace {
				continue
			}
		}

		objects = append(objects, obj)
	}

	return objects, nil
}

// Create 创建资源
func (s *EtcdStore) Create(gvk schema.GroupVersionKind, obj runtime.Object) error {
	meta, err := getObjectMeta(obj)
	if err != nil {
		return err
	}

	namespace := meta.GetNamespace()
	name := meta.GetName()
	key := s.resourceKey(gvk, namespace, name)

	// 检查资源是否已存在
	resp, err := s.client.Get(context.Background(), key)
	if err != nil {
		return fmt.Errorf("failed to check resource existence: %w", err)
	}

	if len(resp.Kvs) > 0 {
		return fmt.Errorf("resource already exists: %s/%s", namespace, name)
	}

	// 序列化对象
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
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

	// 保存到 etcd
	_, err = s.client.Put(context.Background(), key, string(data))
	if err != nil {
		return fmt.Errorf("failed to put to etcd: %w", err)
	}

	// 通知 watchers
	s.notifyWatchers(gvk, namespace, ResourceEvent{
		Type:   EventAdded,
		Object: obj,
	})

	return nil
}

// Update 更新资源
func (s *EtcdStore) Update(gvk schema.GroupVersionKind, obj runtime.Object) error {
	meta, err := getObjectMeta(obj)
	if err != nil {
		return err
	}

	namespace := meta.GetNamespace()
	name := meta.GetName()
	key := s.resourceKey(gvk, namespace, name)

	// 获取旧资源
	resp, err := s.client.Get(context.Background(), key)
	if err != nil {
		return fmt.Errorf("failed to get old resource: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return fmt.Errorf("resource not found: %s/%s", namespace, name)
	}

	oldObj, _, err := s.parser.ParseYAML(resp.Kvs[0].Value)
	if err != nil {
		return fmt.Errorf("failed to parse old resource: %w", err)
	}

	// 序列化新对象
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}

	// 更新 resourceVersion
	resourceVersion := fmt.Sprintf("%d", time.Now().UnixNano())
	meta.SetResourceVersion(resourceVersion)

	// 更新 etcd
	_, err = s.client.Put(context.Background(), key, string(data))
	if err != nil {
		return fmt.Errorf("failed to update etcd: %w", err)
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
func (s *EtcdStore) Delete(gvk schema.GroupVersionKind, namespace, name string) error {
	key := s.resourceKey(gvk, namespace, name)

	// 获取资源（用于返回和通知）
	resp, err := s.client.Get(context.Background(), key)
	if err != nil {
		return fmt.Errorf("failed to get resource: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return fmt.Errorf("resource not found: %s/%s", namespace, name)
	}

	obj, _, err := s.parser.ParseYAML(resp.Kvs[0].Value)
	if err != nil {
		return fmt.Errorf("failed to parse resource: %w", err)
	}

	// 删除资源
	_, err = s.client.Delete(context.Background(), key)
	if err != nil {
		return fmt.Errorf("failed to delete from etcd: %w", err)
	}

	// 通知 watchers
	s.notifyWatchers(gvk, namespace, ResourceEvent{
		Type:   EventDeleted,
		Object: obj,
	})

	return nil
}

// Watch 监听资源变更
func (s *EtcdStore) Watch(gvk schema.GroupVersionKind, namespace string, resourceVersion string) (<-chan ResourceEvent, error) {
	watchKey := s.watchKey(gvk, namespace)
	ch := make(chan ResourceEvent, 100)

	// 注册 watcher
	if s.watchers[watchKey] == nil {
		s.watchers[watchKey] = make([]chan ResourceEvent, 0)
	}
	s.watchers[watchKey] = append(s.watchers[watchKey], ch)

	return ch, nil
}

// startWatcher 启动 etcd watch 监听器
func (s *EtcdStore) startWatcher() {
	// 监听所有 /kubernetes 前缀的键
	watchChan := s.client.Watch(s.ctx, "/kubernetes", clientv3.WithPrefix())

	for watchResp := range watchChan {
		for _, event := range watchResp.Events {
			// 解析对象
			obj, _, err := s.parser.ParseYAML(event.Kv.Value)
			if err != nil {
				continue
			}

			// 从对象获取 GVK 和 namespace
			gvk := obj.GetObjectKind().GroupVersionKind()
			meta, err := getObjectMeta(obj)
			if err != nil {
				continue
			}
			namespace := meta.GetNamespace()

			// 确定事件类型
			var eventType EventType
			switch event.Type {
			case clientv3.EventTypePut:
				if event.IsCreate() {
					eventType = EventAdded
				} else {
					eventType = EventModified
				}
			case clientv3.EventTypeDelete:
				eventType = EventDeleted
			default:
				continue
			}

			// 通知 watchers
			s.notifyWatchers(gvk, namespace, ResourceEvent{
				Type:   eventType,
				Object: obj,
			})
		}
	}
}

// notifyWatchers 通知所有 watchers
func (s *EtcdStore) notifyWatchers(gvk schema.GroupVersionKind, namespace string, event ResourceEvent) {
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

// Close 关闭 etcd 连接
func (s *EtcdStore) Close() error {
	s.cancel()
	return s.client.Close()
}
