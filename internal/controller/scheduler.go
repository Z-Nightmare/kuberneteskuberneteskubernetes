package controller

import (
	"context"
	"fmt"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchedulerController 实现 Pod 调度功能
type SchedulerController struct {
	store  storage.Store
	logger logprovider.Logger
	stopCh chan struct{}
}

// NewSchedulerController 创建 Scheduler 控制器
func NewSchedulerController(store storage.Store, logger logprovider.Logger) *SchedulerController {
	return &SchedulerController{
		store:  store,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// Name 返回控制器名称
func (sc *SchedulerController) Name() string {
	return "SchedulerController"
}

// Start 启动 Scheduler 控制器
func (sc *SchedulerController) Start(ctx context.Context) error {
	sc.logger.Info("启动 Scheduler 控制器...")

	podGVK := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	// 监听 Pod 资源变化
	watchCh, err := sc.store.Watch(podGVK, "", "")
	if err != nil {
		return fmt.Errorf("无法监听 Pod 资源: %w", err)
	}

	// 启动处理循环
	go sc.processPods(ctx, watchCh)

	// 处理现有的未调度 Pod
	if err := sc.syncPendingPods(ctx); err != nil {
		sc.logger.Error("同步待调度 Pod 失败: ", err.Error())
	}

	return nil
}

// Stop 停止 Scheduler 控制器
func (sc *SchedulerController) Stop(ctx context.Context) error {
	sc.logger.Info("停止 Scheduler 控制器...")
	close(sc.stopCh)
	return nil
}

// syncPendingPods 同步待调度的 Pod
func (sc *SchedulerController) syncPendingPods(ctx context.Context) error {
	podGVK := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	pods, err := sc.store.List(podGVK, "")
	if err != nil {
		return err
	}

	sc.logger.Infof("发现 %d 个 Pod，检查待调度状态...", len(pods))

	for _, obj := range pods {
		if pod, ok := obj.(*corev1.Pod); ok {
			if pod.Spec.NodeName == "" && pod.Status.Phase == corev1.PodPending {
				sc.logger.Infof("发现待调度 Pod: %s/%s", pod.Namespace, pod.Name)
				if err := sc.schedulePod(ctx, pod); err != nil {
					sc.logger.Error("调度 Pod 失败: ", pod.Name, " error: ", err.Error())
				}
			}
		}
	}

	return nil
}

// processPods 处理 Pod 事件
func (sc *SchedulerController) processPods(ctx context.Context, watchCh <-chan storage.ResourceEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-sc.stopCh:
			return
		case event, ok := <-watchCh:
			if !ok {
				sc.logger.Warn("Pod watch 通道已关闭")
				return
			}

			switch event.Type {
			case storage.EventAdded, storage.EventModified:
				if pod, ok := event.Object.(*corev1.Pod); ok {
					// 只处理未调度的 Pod
					if pod.Spec.NodeName == "" && pod.Status.Phase == corev1.PodPending {
						sc.logger.Infof("发现待调度 Pod: %s/%s", pod.Namespace, pod.Name)
						if err := sc.schedulePod(ctx, pod); err != nil {
							sc.logger.Error("调度 Pod 失败: ", pod.Name, " error: ", err.Error())
						}
					}
				}
			}
		}
	}
}

// schedulePod 调度 Pod 到节点
func (sc *SchedulerController) schedulePod(ctx context.Context, pod *corev1.Pod) error {
	// 获取所有可用节点
	nodeGVK := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Node",
	}

	nodes, err := sc.store.List(nodeGVK, "")
	if err != nil {
		return fmt.Errorf("获取节点列表失败: %w", err)
	}

	if len(nodes) == 0 {
		return fmt.Errorf("没有可用的节点")
	}

	// 简单的调度策略：选择第一个可用节点
	// 实际应该实现更复杂的调度算法（资源检查、亲和性等）
	var selectedNode *corev1.Node
	for _, obj := range nodes {
		if node, ok := obj.(*corev1.Node); ok {
			// 检查节点是否就绪
			if sc.isNodeReady(node) {
				selectedNode = node
				break
			}
		}
	}

	if selectedNode == nil {
		return fmt.Errorf("没有可用的就绪节点")
	}

	sc.logger.Infof("将 Pod %s/%s 调度到节点 %s", pod.Namespace, pod.Name, selectedNode.Name)

	// 更新 Pod，设置节点名称
	pod.Spec.NodeName = selectedNode.Name
	pod.Status.Phase = corev1.PodPending // Pod 已调度但还未运行
	pod.Status.Conditions = []corev1.PodCondition{
		{
			Type:               corev1.PodScheduled,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "Scheduled",
			Message:            fmt.Sprintf("Successfully assigned %s/%s to %s", pod.Namespace, pod.Name, selectedNode.Name),
		},
	}

	// 更新 Pod
	podGVK := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	if err := sc.store.Update(podGVK, pod); err != nil {
		return fmt.Errorf("更新 Pod 失败: %w", err)
	}

	sc.logger.Infof("Pod %s/%s 已成功调度到节点 %s", pod.Namespace, pod.Name, selectedNode.Name)
	return nil
}

// isNodeReady 检查节点是否就绪
func (sc *SchedulerController) isNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
