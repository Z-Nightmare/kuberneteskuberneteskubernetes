package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// DeploymentController 管理 Deployment 资源
type DeploymentController struct {
	store  storage.Store
	logger logprovider.Logger
	stopCh chan struct{}
}

// NewDeploymentController 创建 Deployment 控制器
func NewDeploymentController(store storage.Store, logger logprovider.Logger) *DeploymentController {
	return &DeploymentController{
		store:  store,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// Name 返回控制器名称
func (dc *DeploymentController) Name() string {
	return "DeploymentController"
}

// Start 启动 Deployment 控制器
func (dc *DeploymentController) Start(ctx context.Context) error {
	dc.logger.Info("启动 Deployment 控制器...")

	gvk := schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}

	// 监听 Deployment 资源变化
	watchCh, err := dc.store.Watch(gvk, "", "")
	if err != nil {
		return fmt.Errorf("无法监听 Deployment 资源: %w", err)
	}

	// 启动处理循环
	go dc.processDeployments(ctx, watchCh)

	// 处理现有的 Deployment
	if err := dc.syncExistingDeployments(ctx); err != nil {
		dc.logger.Error("同步现有 Deployment 失败: ", err.Error())
	}

	return nil
}

// Stop 停止 Deployment 控制器
func (dc *DeploymentController) Stop(ctx context.Context) error {
	dc.logger.Info("停止 Deployment 控制器...")
	close(dc.stopCh)
	return nil
}

// syncExistingDeployments 同步现有的 Deployment
func (dc *DeploymentController) syncExistingDeployments(ctx context.Context) error {
	gvk := schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}

	deployments, err := dc.store.List(gvk, "")
	if err != nil {
		return err
	}

	dc.logger.Infof("发现 %d 个现有 Deployment，开始同步...", len(deployments))

	for _, obj := range deployments {
		if deployment, ok := obj.(*appsv1.Deployment); ok {
			if err := dc.syncDeployment(ctx, deployment); err != nil {
				dc.logger.Error("同步 Deployment 失败: ", deployment.Name, " error: ", err.Error())
			}
		}
	}

	return nil
}

// processDeployments 处理 Deployment 事件
func (dc *DeploymentController) processDeployments(ctx context.Context, watchCh <-chan storage.ResourceEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-dc.stopCh:
			return
		case event, ok := <-watchCh:
			if !ok {
				dc.logger.Warn("Deployment watch 通道已关闭")
				return
			}

			switch event.Type {
			case storage.EventAdded, storage.EventModified:
				if deployment, ok := event.Object.(*appsv1.Deployment); ok {
					dc.logger.Infof("处理 Deployment 事件: %s/%s (%s)", deployment.Namespace, deployment.Name, event.Type)
					if err := dc.syncDeployment(ctx, deployment); err != nil {
						dc.logger.Error("同步 Deployment 失败: ", deployment.Name, " error: ", err.Error())
					}
				}
			case storage.EventDeleted:
				if deployment, ok := event.Object.(*appsv1.Deployment); ok {
					dc.logger.Infof("删除 Deployment: %s/%s", deployment.Namespace, deployment.Name)
					// 可以在这里清理相关的 Pod
				}
			}
		}
	}
}

// syncDeployment 同步 Deployment（确保 Pod 数量正确）
func (dc *DeploymentController) syncDeployment(ctx context.Context, deployment *appsv1.Deployment) error {
	replicas := int32(1)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	// 获取当前 Deployment 的 Pod
	podGVK := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	// 查找属于该 Deployment 的 Pod
	allPods, err := dc.store.List(podGVK, deployment.Namespace)
	if err != nil {
		return err
	}

	selector := deploymentSelectorLabels(deployment)

	var deploymentPods []*corev1.Pod
	for _, obj := range allPods {
		if pod, ok := obj.(*corev1.Pod); ok {
			// MySQLStore 目前只持久化 labels/annotations，OwnerReferences 可能不会被完整恢复。
			// 因此这里优先用 selector.matchLabels 关联 Pod，避免重复创建。
			if labelsMatchAll(pod.Labels, selector) || hasDeploymentOwnerRef(pod, deployment.Name) {
				deploymentPods = append(deploymentPods, pod)
			}
		}
	}

	currentReplicas := int32(len(deploymentPods))
	dc.logger.Infof("Deployment %s/%s: 期望副本数=%d, 当前副本数=%d",
		deployment.Namespace, deployment.Name, replicas, currentReplicas)

	// 如果 Pod 数量不足，创建新的 Pod
	if currentReplicas < replicas {
		needed := replicas - currentReplicas
		dc.logger.Infof("需要创建 %d 个 Pod", needed)

		for i := int32(0); i < needed; i++ {
			pod := dc.createPodForDeployment(deployment, currentReplicas+i)
			if err := dc.store.Create(podGVK, pod); err != nil {
				dc.logger.Error("创建 Pod 失败: ", err.Error())
				continue
			}
			dc.logger.Infof("创建 Pod: %s/%s", pod.Namespace, pod.Name)
		}
	}

	// 如果 Pod 数量过多，删除多余的 Pod（简化实现，实际应该更智能）
	if currentReplicas > replicas {
		excess := currentReplicas - replicas
		dc.logger.Infof("需要删除 %d 个 Pod", excess)

		for i := int32(0); i < excess && i < int32(len(deploymentPods)); i++ {
			pod := deploymentPods[i]
			if err := dc.store.Delete(podGVK, pod.Namespace, pod.Name); err != nil {
				dc.logger.Error("删除 Pod 失败: ", err.Error())
				continue
			}
			dc.logger.Infof("删除 Pod: %s/%s", pod.Namespace, pod.Name)
		}
	}

	return nil
}

func deploymentSelectorLabels(deploy *appsv1.Deployment) map[string]string {
	if deploy == nil {
		return map[string]string{}
	}
	if deploy.Spec.Selector != nil && len(deploy.Spec.Selector.MatchLabels) > 0 {
		return deploy.Spec.Selector.MatchLabels
	}
	if len(deploy.Spec.Template.Labels) > 0 {
		return deploy.Spec.Template.Labels
	}
	if len(deploy.Labels) > 0 {
		return deploy.Labels
	}
	return map[string]string{}
}

func labelsMatchAll(labels map[string]string, selector map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if labels == nil {
			return false
		}
		if labels[k] != v {
			return false
		}
	}
	return true
}

func hasDeploymentOwnerRef(pod *corev1.Pod, deployName string) bool {
	if pod == nil || deployName == "" {
		return false
	}
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "Deployment" && ref.Name == deployName {
			return true
		}
	}
	return false
}

// createPodForDeployment 为 Deployment 创建 Pod
func (dc *DeploymentController) createPodForDeployment(deployment *appsv1.Deployment, index int32) *corev1.Pod {
	podName := fmt.Sprintf("%s-%d", deployment.Name, time.Now().UnixNano())

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: deployment.Namespace,
			Labels:    deployment.Spec.Template.Labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
					UID:        deployment.UID,
					Controller: func() *bool { b := true; return &b }(),
				},
			},
			CreationTimestamp: metav1.Now(),
		},
		Spec: deployment.Spec.Template.Spec,
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	// 设置 UID
	if pod.UID == "" {
		pod.UID = types.UID(fmt.Sprintf("pod-%s-%d", podName, time.Now().UnixNano()))
	}

	return pod
}
