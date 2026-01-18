package storage

import (
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// EventType 表示资源事件类型
type EventType string

const (
	EventAdded    EventType = "ADDED"
	EventModified EventType = "MODIFIED"
	EventDeleted  EventType = "DELETED"
	EventBookmark EventType = "BOOKMARK"
)

// ResourceEvent 表示资源变更事件
type ResourceEvent struct {
	Type   EventType
	Object runtime.Object
	OldObj runtime.Object // 用于 MODIFIED 事件
}

// Store 是 Kubernetes 资源的存储接口
type Store interface {
	// Get 获取指定资源
	Get(gvk schema.GroupVersionKind, namespace, name string) (runtime.Object, error)
	// List 列出所有资源（可指定 namespace）
	List(gvk schema.GroupVersionKind, namespace string) ([]runtime.Object, error)
	// Create 创建资源
	Create(gvk schema.GroupVersionKind, obj runtime.Object) error
	// Update 更新资源
	Update(gvk schema.GroupVersionKind, obj runtime.Object) error
	// Delete 删除资源
	Delete(gvk schema.GroupVersionKind, namespace, name string) error
	// Watch 监听资源变更
	Watch(gvk schema.GroupVersionKind, namespace string, resourceVersion string) (<-chan ResourceEvent, error)
}

// MemoryStore 是基于内存的存储实现
type MemoryStore struct {
	mu        sync.RWMutex
	resources map[string]map[string]runtime.Object // key: gvk-namespace-name, value: object
	watchers  map[string][]chan ResourceEvent      // key: gvk-namespace, value: watchers
	version   int64                                // 全局版本号，用于 resourceVersion
}

// NewMemoryStore 创建新的内存存储
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		resources: make(map[string]map[string]runtime.Object),
		watchers:  make(map[string][]chan ResourceEvent),
		version:   0,
	}
}

// key 生成资源的唯一键
func (s *MemoryStore) key(gvk schema.GroupVersionKind, namespace, name string) string {
	if namespace == "" {
		return fmt.Sprintf("%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, name)
	}
	return fmt.Sprintf("%s/%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, namespace, name)
}

// watchKey 生成 watch 的键
func (s *MemoryStore) watchKey(gvk schema.GroupVersionKind, namespace string) string {
	if namespace == "" {
		return fmt.Sprintf("%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind)
	}
	return fmt.Sprintf("%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, namespace)
}

// getObjectMeta 获取对象的元数据
func getObjectMeta(obj runtime.Object) (metav1.Object, error) {
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return nil, fmt.Errorf("object does not implement metav1.Object")
	}
	return metaObj, nil
}

// Get 获取指定资源
func (s *MemoryStore) Get(gvk schema.GroupVersionKind, namespace, name string) (runtime.Object, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.key(gvk, namespace, name)
	nsMap, exists := s.resources[key]
	if !exists {
		return nil, fmt.Errorf("resource not found: %s", key)
	}

	obj, exists := nsMap[name]
	if !exists {
		return nil, fmt.Errorf("resource not found: %s/%s", namespace, name)
	}

	return obj, nil
}

// List 列出所有资源
func (s *MemoryStore) List(gvk schema.GroupVersionKind, namespace string) ([]runtime.Object, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []runtime.Object

	// 遍历所有资源
	for _, nsMap := range s.resources {
		for _, obj := range nsMap {
			meta, err := getObjectMeta(obj)
			if err != nil {
				continue
			}

			// 检查 GVK 是否匹配
			objGVK := obj.GetObjectKind().GroupVersionKind()
			if objGVK.Group != gvk.Group || objGVK.Version != gvk.Version || objGVK.Kind != gvk.Kind {
				continue
			}

			// 如果指定了 namespace，只返回该 namespace 的资源
			if namespace != "" && meta.GetNamespace() != namespace {
				continue
			}

			results = append(results, obj)
		}
	}

	return results, nil
}

// Create 创建资源
func (s *MemoryStore) Create(gvk schema.GroupVersionKind, obj runtime.Object) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := getObjectMeta(obj)
	if err != nil {
		return err
	}

	namespace := meta.GetNamespace()
	name := meta.GetName()
	key := s.key(gvk, namespace, name)

	// 检查资源是否已存在
	if nsMap, exists := s.resources[key]; exists {
		if _, exists := nsMap[name]; exists {
			return fmt.Errorf("resource already exists: %s/%s", namespace, name)
		}
	}

	// 设置 resourceVersion
	s.version++
	if meta.GetResourceVersion() == "" {
		meta.SetResourceVersion(fmt.Sprintf("%d", s.version))
	}

	// 设置创建时间
	creationTime := meta.GetCreationTimestamp()
	if creationTime.Time.IsZero() {
		meta.SetCreationTimestamp(metav1.NewTime(time.Now()))
	}

	// 设置 UID（如果未设置）
	if meta.GetUID() == "" {
		meta.SetUID(types.UID(fmt.Sprintf("uid-%d", s.version)))
	}

	// 存储资源
	if s.resources[key] == nil {
		s.resources[key] = make(map[string]runtime.Object)
	}
	s.resources[key][name] = obj

	// 通知 watchers
	s.notifyWatchers(gvk, namespace, ResourceEvent{
		Type:   EventAdded,
		Object: obj,
	})

	return nil
}

// Update 更新资源
func (s *MemoryStore) Update(gvk schema.GroupVersionKind, obj runtime.Object) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := getObjectMeta(obj)
	if err != nil {
		return err
	}

	namespace := meta.GetNamespace()
	name := meta.GetName()
	key := s.key(gvk, namespace, name)

	// 检查资源是否存在
	oldObj, exists := s.resources[key][name]
	if !exists {
		return fmt.Errorf("resource not found: %s/%s", namespace, name)
	}

	// 更新 resourceVersion
	s.version++
	meta.SetResourceVersion(fmt.Sprintf("%d", s.version))

	// 更新资源
	s.resources[key][name] = obj

	// 通知 watchers
	s.notifyWatchers(gvk, namespace, ResourceEvent{
		Type:   EventModified,
		Object: obj,
		OldObj: oldObj,
	})

	return nil
}

// Delete 删除资源
func (s *MemoryStore) Delete(gvk schema.GroupVersionKind, namespace, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.key(gvk, namespace, name)

	// 检查资源是否存在
	nsMap, exists := s.resources[key]
	if !exists {
		return fmt.Errorf("resource not found: %s/%s", namespace, name)
	}

	obj, exists := nsMap[name]
	if !exists {
		return fmt.Errorf("resource not found: %s/%s", namespace, name)
	}

	// 删除资源
	delete(nsMap, name)
	if len(nsMap) == 0 {
		delete(s.resources, key)
	}

	// 通知 watchers
	s.notifyWatchers(gvk, namespace, ResourceEvent{
		Type:   EventDeleted,
		Object: obj,
	})

	return nil
}

// Watch 监听资源变更
func (s *MemoryStore) Watch(gvk schema.GroupVersionKind, namespace string, resourceVersion string) (<-chan ResourceEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	watchKey := s.watchKey(gvk, namespace)
	ch := make(chan ResourceEvent, 100) // 缓冲通道

	// 注册 watcher
	if s.watchers[watchKey] == nil {
		s.watchers[watchKey] = make([]chan ResourceEvent, 0)
	}
	s.watchers[watchKey] = append(s.watchers[watchKey], ch)

	return ch, nil
}

// notifyWatchers 通知所有 watchers
func (s *MemoryStore) notifyWatchers(gvk schema.GroupVersionKind, namespace string, event ResourceEvent) {
	watchKey := s.watchKey(gvk, namespace)
	watchers := s.watchers[watchKey]

	// 也通知全局 watchers（namespace 为空）
	if namespace != "" {
		globalKey := s.watchKey(gvk, "")
		watchers = append(watchers, s.watchers[globalKey]...)
	}

	for _, ch := range watchers {
		select {
		case ch <- event:
		default:
			// 如果通道已满，跳过（避免阻塞）
		}
	}
}

// StopWatcher 停止指定的 watcher
func (s *MemoryStore) StopWatcher(gvk schema.GroupVersionKind, namespace string, ch <-chan ResourceEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	watchKey := s.watchKey(gvk, namespace)
	watchers := s.watchers[watchKey]

	for i, w := range watchers {
		if w == ch {
			// 移除 watcher
			s.watchers[watchKey] = append(watchers[:i], watchers[i+1:]...)
			close(w)
			break
		}
	}
}
