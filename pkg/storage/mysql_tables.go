package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"gorm.io/gorm"
)

// BaseResource 是所有资源表的基础结构
type BaseResource struct {
	ID              uint           `gorm:"primaryKey"`
	Name            string         `gorm:"index;size:255;not null"`
	Namespace       string         `gorm:"index;size:255"`
	UID             string         `gorm:"uniqueIndex;size:255"`
	ResourceVersion string         `gorm:"index;size:255"`
	Labels          string         `gorm:"type:json"` // JSON 格式存储 labels
	Annotations     string         `gorm:"type:json"` // JSON 格式存储 annotations
	CreatedAt       time.Time      `gorm:"index"`
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

// PodResource Pod 资源表
type PodResource struct {
	BaseResource
	Spec   string `gorm:"type:json"` // PodSpec 的 JSON
	Status string `gorm:"type:json"` // PodStatus 的 JSON
}

// DeploymentResource Deployment 资源表
type DeploymentResource struct {
	BaseResource
	Replicas          *int32  `gorm:"type:int"`
	ReplicasAvailable *int32  `gorm:"type:int"`
	ReplicasReady     *int32  `gorm:"type:int"`
	ReplicasUpdated   *int32  `gorm:"type:int"`
	Strategy          string  `gorm:"size:50"` // RollingUpdate, Recreate
	Spec              string  `gorm:"type:json"` // DeploymentSpec 的 JSON
	Status            string  `gorm:"type:json"` // DeploymentStatus 的 JSON
}

// ServiceResource Service 资源表
type ServiceResource struct {
	BaseResource
	Type      string `gorm:"size:50;index"` // ClusterIP, NodePort, LoadBalancer, ExternalName
	ClusterIP string `gorm:"size:50"`
	Ports     string `gorm:"type:json"` // ServicePort 数组的 JSON
	Spec      string `gorm:"type:json"` // ServiceSpec 的 JSON
	Status    string `gorm:"type:json"` // ServiceStatus 的 JSON
}

// ConfigMapResource ConfigMap 资源表
type ConfigMapResource struct {
	BaseResource
	Data      string `gorm:"type:json"` // Data 字段的 JSON
	BinaryData string `gorm:"type:json"` // BinaryData 字段的 JSON
}

// SecretResource Secret 资源表
type SecretResource struct {
	BaseResource
	Type     string `gorm:"size:50;index"`
	Data     string `gorm:"type:json"` // Data 字段的 JSON（base64 编码的值）
	StringData string `gorm:"type:json"` // StringData 字段的 JSON
}

// NodeResource Node 资源表
type NodeResource struct {
	BaseResource
	Phase   string `gorm:"size:50;index"` // NodePhase: Pending, Running, Terminated
	Spec    string `gorm:"type:json"`      // NodeSpec 的 JSON
	Status  string `gorm:"type:json"`      // NodeStatus 的 JSON
}

// tableName 根据 GVK 生成表名
func tableName(gvk schema.GroupVersionKind) string {
	group := gvk.Group
	if group == "" {
		group = "core"
	}
	// 表名格式: k8s_{group}_{version}_{kind}
	// 例如: k8s_apps_v1_deployment, k8s_core_v1_pod
	kind := strings.ToLower(gvk.Kind)
	return fmt.Sprintf("k8s_%s_%s_%s", strings.ReplaceAll(group, ".", "_"), gvk.Version, kind)
}

// getTableModel 根据 GVK 获取对应的表模型
func getTableModel(gvk schema.GroupVersionKind) interface{} {
	// 根据资源类型返回对应的模型
	switch gvk.Kind {
	case "Pod":
		if gvk.Group == "" && gvk.Version == "v1" {
			return &PodResource{}
		}
	case "Deployment":
		if gvk.Group == "apps" && gvk.Version == "v1" {
			return &DeploymentResource{}
		}
	case "Service":
		if gvk.Group == "" && gvk.Version == "v1" {
			return &ServiceResource{}
		}
	case "ConfigMap":
		if gvk.Group == "" && gvk.Version == "v1" {
			return &ConfigMapResource{}
		}
	case "Secret":
		if gvk.Group == "" && gvk.Version == "v1" {
			return &SecretResource{}
		}
	case "Node":
		if gvk.Group == "" && gvk.Version == "v1" {
			return &NodeResource{}
		}
	}
	// 默认返回基础资源（用于未知资源类型）
	return &BaseResource{}
}

// ensureTable 确保表存在，如果不存在则创建
func (s *MySQLStore) ensureTable(gvk schema.GroupVersionKind) error {
	tableName := tableName(gvk)
	model := getTableModel(gvk)
	
	// 检查表是否存在
	var count int64
	if err := s.db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", tableName).Scan(&count).Error; err != nil {
		return fmt.Errorf("failed to check table existence: %w", err)
	}

	if count == 0 {
		// 表不存在，创建表
		if err := s.db.Table(tableName).AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to create table %s: %w", tableName, err)
		}
	}

	return nil
}

// toBaseResource 将 runtime.Object 转换为 BaseResource
func toBaseResource(meta metav1.Object) BaseResource {
	labelsJSON, _ := json.Marshal(meta.GetLabels())
	annotationsJSON, _ := json.Marshal(meta.GetAnnotations())

	return BaseResource{
		Name:            meta.GetName(),
		Namespace:       meta.GetNamespace(),
		UID:             string(meta.GetUID()),
		ResourceVersion: meta.GetResourceVersion(),
		Labels:          string(labelsJSON),
		Annotations:     string(annotationsJSON),
		CreatedAt:       meta.GetCreationTimestamp().Time,
		UpdatedAt:       time.Now(),
	}
}

// fromBaseResource 从 BaseResource 恢复 metadata
func fromBaseResource(base BaseResource, obj runtime.Object) error {
	meta, ok := obj.(metav1.Object)
	if !ok {
		return fmt.Errorf("object does not implement metav1.Object")
	}

	meta.SetName(base.Name)
	meta.SetNamespace(base.Namespace)
	meta.SetUID(types.UID(base.UID))
	meta.SetResourceVersion(base.ResourceVersion)
	meta.SetCreationTimestamp(metav1.NewTime(base.CreatedAt))

	// 恢复 labels
	if base.Labels != "" {
		var labels map[string]string
		if err := json.Unmarshal([]byte(base.Labels), &labels); err == nil {
			meta.SetLabels(labels)
		}
	}

	// 恢复 annotations
	if base.Annotations != "" {
		var annotations map[string]string
		if err := json.Unmarshal([]byte(base.Annotations), &annotations); err == nil {
			meta.SetAnnotations(annotations)
		}
	}

	return nil
}

// savePod 保存 Pod 资源
func (s *MySQLStore) savePod(pod *corev1.Pod) error {
	tableName := tableName(pod.GroupVersionKind())
	base := toBaseResource(pod)

	specJSON, _ := json.Marshal(pod.Spec)
	statusJSON, _ := json.Marshal(pod.Status)

	resource := PodResource{
		BaseResource: base,
		Spec:         string(specJSON),
		Status:       string(statusJSON),
	}

	return s.db.Table(tableName).Create(&resource).Error
}

// loadPod 加载 Pod 资源
func (s *MySQLStore) loadPod(gvk schema.GroupVersionKind, namespace, name string) (*corev1.Pod, error) {
	tableName := tableName(gvk)
	var resource PodResource

	if err := s.db.Table(tableName).Where("name = ? AND namespace = ?", name, namespace).First(&resource).Error; err != nil {
		return nil, err
	}

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", gvk.Group, gvk.Version),
			Kind:       gvk.Kind,
		},
	}

	if err := fromBaseResource(resource.BaseResource, pod); err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(resource.Spec), &pod.Spec)
	json.Unmarshal([]byte(resource.Status), &pod.Status)

	return pod, nil
}

// saveDeployment 保存 Deployment 资源
func (s *MySQLStore) saveDeployment(deployment *appsv1.Deployment) error {
	tableName := tableName(deployment.GroupVersionKind())
	base := toBaseResource(deployment)

	specJSON, _ := json.Marshal(deployment.Spec)
	statusJSON, _ := json.Marshal(deployment.Status)

	resource := DeploymentResource{
		BaseResource: base,
		Replicas:     deployment.Spec.Replicas,
		Spec:         string(specJSON),
		Status:       string(statusJSON),
	}

	// 设置可选字段
	if deployment.Status.AvailableReplicas > 0 {
		avail := deployment.Status.AvailableReplicas
		resource.ReplicasAvailable = &avail
	}
	if deployment.Status.ReadyReplicas > 0 {
		ready := deployment.Status.ReadyReplicas
		resource.ReplicasReady = &ready
	}
	if deployment.Status.UpdatedReplicas > 0 {
		updated := deployment.Status.UpdatedReplicas
		resource.ReplicasUpdated = &updated
	}
	if deployment.Spec.Strategy.Type != "" {
		resource.Strategy = string(deployment.Spec.Strategy.Type)
	}

	return s.db.Table(tableName).Create(&resource).Error
}

// loadDeployment 加载 Deployment 资源
func (s *MySQLStore) loadDeployment(gvk schema.GroupVersionKind, namespace, name string) (*appsv1.Deployment, error) {
	tableName := tableName(gvk)
	var resource DeploymentResource

	if err := s.db.Table(tableName).Where("name = ? AND namespace = ?", name, namespace).First(&resource).Error; err != nil {
		return nil, err
	}

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", gvk.Group, gvk.Version),
			Kind:       gvk.Kind,
		},
	}

	if err := fromBaseResource(resource.BaseResource, deployment); err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(resource.Spec), &deployment.Spec)
	json.Unmarshal([]byte(resource.Status), &deployment.Status)

	// 恢复主要字段（如果 JSON 解析失败，使用数据库字段）
	if resource.Replicas != nil {
		deployment.Spec.Replicas = resource.Replicas
	}
	if resource.ReplicasAvailable != nil {
		deployment.Status.AvailableReplicas = *resource.ReplicasAvailable
	}
	if resource.ReplicasReady != nil {
		deployment.Status.ReadyReplicas = *resource.ReplicasReady
	}
	if resource.ReplicasUpdated != nil {
		deployment.Status.UpdatedReplicas = *resource.ReplicasUpdated
	}
	if resource.Strategy != "" && deployment.Spec.Strategy.Type == "" {
		deployment.Spec.Strategy.Type = appsv1.DeploymentStrategyType(resource.Strategy)
	}

	return deployment, nil
}

// saveService 保存 Service 资源
func (s *MySQLStore) saveService(service *corev1.Service) error {
	tableName := tableName(service.GroupVersionKind())
	base := toBaseResource(service)

	specJSON, _ := json.Marshal(service.Spec)
	statusJSON, _ := json.Marshal(service.Status)
	portsJSON, _ := json.Marshal(service.Spec.Ports)

	resource := ServiceResource{
		BaseResource: base,
		Type:         string(service.Spec.Type),
		ClusterIP:    service.Spec.ClusterIP,
		Ports:        string(portsJSON),
		Spec:         string(specJSON),
		Status:       string(statusJSON),
	}

	return s.db.Table(tableName).Create(&resource).Error
}

// loadService 加载 Service 资源
func (s *MySQLStore) loadService(gvk schema.GroupVersionKind, namespace, name string) (*corev1.Service, error) {
	tableName := tableName(gvk)
	var resource ServiceResource

	if err := s.db.Table(tableName).Where("name = ? AND namespace = ?", name, namespace).First(&resource).Error; err != nil {
		return nil, err
	}

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", gvk.Group, gvk.Version),
			Kind:       gvk.Kind,
		},
	}

	if err := fromBaseResource(resource.BaseResource, service); err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(resource.Spec), &service.Spec)
	json.Unmarshal([]byte(resource.Status), &service.Status)

	// 恢复主要字段
	service.Spec.Type = corev1.ServiceType(resource.Type)
	service.Spec.ClusterIP = resource.ClusterIP
	json.Unmarshal([]byte(resource.Ports), &service.Spec.Ports)

	return service, nil
}

// saveGenericResource 保存通用资源（使用基础表结构）
func (s *MySQLStore) saveGenericResource(gvk schema.GroupVersionKind, obj runtime.Object) error {
	tableName := tableName(gvk)
	meta, err := getObjectMeta(obj)
	if err != nil {
		return err
	}

	base := toBaseResource(meta)
	
	// 将整个对象序列化为 JSON 存储在 annotations 中（作为备用）
	objJSON, _ := json.Marshal(obj)
	base.Annotations = string(objJSON)

	return s.db.Table(tableName).Create(&base).Error
}

// loadGenericResource 加载通用资源
func (s *MySQLStore) loadGenericResource(gvk schema.GroupVersionKind, namespace, name string) (runtime.Object, error) {
	tableName := tableName(gvk)
	var resource BaseResource

	if err := s.db.Table(tableName).Where("name = ? AND namespace = ?", name, namespace).First(&resource).Error; err != nil {
		return nil, err
	}

	// 从 annotations 中恢复完整对象
	if resource.Annotations != "" {
		// 尝试解析为完整对象
		obj, _, err := s.parser.ParseYAML([]byte(resource.Annotations))
		if err == nil {
			return obj, nil
		}
	}

	// 如果解析失败，返回错误
	return nil, fmt.Errorf("failed to load generic resource")
}

// saveNode 保存 Node 资源（支持创建和更新）
func (s *MySQLStore) saveNode(node *corev1.Node) error {
	tableName := tableName(node.GroupVersionKind())
	base := toBaseResource(node)

	specJSON, _ := json.Marshal(node.Spec)
	statusJSON, _ := json.Marshal(node.Status)

	resource := NodeResource{
		BaseResource: base,
		Phase:        string(node.Status.Phase),
		Spec:         string(specJSON),
		Status:       string(statusJSON),
	}

	// 检查节点是否已存在
	var existing NodeResource
	err := s.db.Table(tableName).Where("name = ?", node.Name).First(&existing).Error
	if err == nil {
		// 节点已存在，执行更新
		resource.ID = existing.ID
		resource.CreatedAt = existing.CreatedAt
		return s.db.Table(tableName).Save(&resource).Error
	}

	// 节点不存在，创建新节点
	return s.db.Table(tableName).Create(&resource).Error
}

// loadNode 加载 Node 资源
func (s *MySQLStore) loadNode(gvk schema.GroupVersionKind, namespace, name string) (*corev1.Node, error) {
	tableName := tableName(gvk)
	var resource NodeResource

	// Node 资源没有 namespace，使用空字符串查询
	if err := s.db.Table(tableName).Where("name = ?", name).First(&resource).Error; err != nil {
		return nil, err
	}

	node := &corev1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", gvk.Group, gvk.Version),
			Kind:       gvk.Kind,
		},
	}

	if err := fromBaseResource(resource.BaseResource, node); err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(resource.Spec), &node.Spec)
	json.Unmarshal([]byte(resource.Status), &node.Status)

	// 恢复 Phase
	if resource.Phase != "" {
		node.Status.Phase = corev1.NodePhase(resource.Phase)
	}

	return node, nil
}
