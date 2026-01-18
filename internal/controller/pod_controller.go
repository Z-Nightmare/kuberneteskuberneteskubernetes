package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// PodController 管理 Pod 资源的生命周期
type PodController struct {
	store  storage.Store
	logger logprovider.Logger
	stopCh chan struct{}
}

// NewPodController 创建 Pod 控制器
func NewPodController(store storage.Store, logger logprovider.Logger) *PodController {
	return &PodController{
		store:  store,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// Name 返回控制器名称
func (pc *PodController) Name() string {
	return "PodController"
}

// Start 启动 Pod 控制器
func (pc *PodController) Start(ctx context.Context) error {
	pc.logger.Info("启动 Pod 控制器...")

	podGVK := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	// 监听 Pod 资源变化
	watchCh, err := pc.store.Watch(podGVK, "", "")
	if err != nil {
		return fmt.Errorf("无法监听 Pod 资源: %w", err)
	}

	// 启动处理循环
	go pc.processPods(ctx, watchCh)

	// 处理现有的 Pod
	if err := pc.syncExistingPods(ctx); err != nil {
		pc.logger.Error("同步现有 Pod 失败: ", err.Error())
	}

	return nil
}

// Stop 停止 Pod 控制器
func (pc *PodController) Stop(ctx context.Context) error {
	pc.logger.Info("停止 Pod 控制器...")
	close(pc.stopCh)
	return nil
}

// syncExistingPods 同步现有的 Pod
func (pc *PodController) syncExistingPods(ctx context.Context) error {
	podGVK := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	pods, err := pc.store.List(podGVK, "")
	if err != nil {
		return err
	}

	pc.logger.Infof("发现 %d 个现有 Pod，开始同步...", len(pods))

	for _, obj := range pods {
		if pod, ok := obj.(*corev1.Pod); ok {
			if err := pc.syncPod(ctx, pod); err != nil {
				pc.logger.Error("同步 Pod 失败: ", pod.Name, " error: ", err.Error())
			}
		}
	}

	return nil
}

// processPods 处理 Pod 事件
func (pc *PodController) processPods(ctx context.Context, watchCh <-chan storage.ResourceEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-pc.stopCh:
			return
		case event, ok := <-watchCh:
			if !ok {
				pc.logger.Warn("Pod watch 通道已关闭")
				return
			}

			switch event.Type {
			case storage.EventAdded:
				if pod, ok := event.Object.(*corev1.Pod); ok {
					pc.logger.Infof("处理 Pod 创建事件: %s/%s", pod.Namespace, pod.Name)
					if err := pc.handlePodCreated(ctx, pod); err != nil {
						pc.logger.Error("处理 Pod 创建失败: ", pod.Name, " error: ", err.Error())
					}
				}
			case storage.EventModified:
				if pod, ok := event.Object.(*corev1.Pod); ok {
					pc.logger.Debugf("处理 Pod 更新事件: %s/%s", pod.Namespace, pod.Name)
					if err := pc.syncPod(ctx, pod); err != nil {
						pc.logger.Error("同步 Pod 失败: ", pod.Name, " error: ", err.Error())
					}
				}
			case storage.EventDeleted:
				if pod, ok := event.Object.(*corev1.Pod); ok {
					pc.logger.Infof("处理 Pod 删除事件: %s/%s", pod.Namespace, pod.Name)
					if err := pc.handlePodDeleted(ctx, pod); err != nil {
						pc.logger.Error("处理 Pod 删除失败: ", pod.Name, " error: ", err.Error())
					}
				}
			}
		}
	}
}

// handlePodCreated 处理新创建的 Pod
func (pc *PodController) handlePodCreated(ctx context.Context, pod *corev1.Pod) error {
	// 初始化 Pod 状态
	if pod.Status.Phase == "" {
		pod.Status.Phase = corev1.PodPending
	}

	// 设置 UID（如果还没有）
	if pod.UID == "" {
		pod.UID = types.UID(fmt.Sprintf("pod-%s-%s-%d", pod.Namespace, pod.Name, time.Now().UnixNano()))
	}

	// 设置创建时间（如果还没有）
	if pod.CreationTimestamp.IsZero() {
		pod.CreationTimestamp = metav1.Now()
	}

	// 初始化 Pod 条件
	if len(pod.Status.Conditions) == 0 {
		pod.Status.Conditions = []corev1.PodCondition{
			{
				Type:               corev1.PodScheduled,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
				Reason:             "Unscheduled",
				Message:            "Pod is waiting to be scheduled",
			},
			{
				Type:               corev1.PodInitialized,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
				Reason:             "NotInitialized",
				Message:            "Pod is being initialized",
			},
			{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
				Reason:             "NotReady",
				Message:            "Pod is not ready",
			},
		}
	}

	// 初始化容器状态
	if len(pod.Status.ContainerStatuses) == 0 && len(pod.Spec.Containers) > 0 {
		pod.Status.ContainerStatuses = make([]corev1.ContainerStatus, 0, len(pod.Spec.Containers))
		for _, container := range pod.Spec.Containers {
			pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{
				Name:  container.Name,
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
				Ready: false,
			})
		}
	}

	// 更新 Pod
	podGVK := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	if err := pc.store.Update(podGVK, pod); err != nil {
		return fmt.Errorf("更新 Pod 状态失败: %w", err)
	}

	pc.logger.Infof("Pod %s/%s 已初始化，状态: %s", pod.Namespace, pod.Name, pod.Status.Phase)
	return nil
}

// syncPod 同步 Pod 状态
func (pc *PodController) syncPod(ctx context.Context, pod *corev1.Pod) error {
	// 根据 Pod 的当前状态更新条件
	pc.updatePodConditions(pod)

	// 更新 Pod 阶段
	pc.updatePodPhase(pod)

	// 更新 Pod
	podGVK := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	if err := pc.store.Update(podGVK, pod); err != nil {
		return fmt.Errorf("更新 Pod 状态失败: %w", err)
	}

	return nil
}

// updatePodConditions 更新 Pod 条件
func (pc *PodController) updatePodConditions(pod *corev1.Pod) {
	now := metav1.Now()
	conditions := make(map[corev1.PodConditionType]*corev1.PodCondition)

	// 初始化条件映射
	for i := range pod.Status.Conditions {
		conditions[pod.Status.Conditions[i].Type] = &pod.Status.Conditions[i]
	}

	// 更新 Scheduled 条件
	if pod.Spec.NodeName != "" {
		if cond, exists := conditions[corev1.PodScheduled]; !exists || cond.Status != corev1.ConditionTrue {
			if cond == nil {
				cond = &corev1.PodCondition{
					Type:               corev1.PodScheduled,
					LastTransitionTime: now,
				}
				pod.Status.Conditions = append(pod.Status.Conditions, *cond)
				conditions[corev1.PodScheduled] = cond
			}
			cond.Status = corev1.ConditionTrue
			cond.Reason = "Scheduled"
			cond.Message = fmt.Sprintf("Successfully assigned %s/%s to %s", pod.Namespace, pod.Name, pod.Spec.NodeName)
			cond.LastTransitionTime = now
		}
	}

	// 更新 Initialized 条件
	if pod.Spec.NodeName != "" {
		if cond, exists := conditions[corev1.PodInitialized]; !exists || cond.Status != corev1.ConditionTrue {
			if cond == nil {
				cond = &corev1.PodCondition{
					Type:               corev1.PodInitialized,
					LastTransitionTime: now,
				}
				pod.Status.Conditions = append(pod.Status.Conditions, *cond)
				conditions[corev1.PodInitialized] = cond
			}
			cond.Status = corev1.ConditionTrue
			cond.Reason = "Initialized"
			cond.Message = "Pod has been initialized"
			cond.LastTransitionTime = now
		}
	}

	// 更新 Ready 条件
	allContainersReady := true
	for _, status := range pod.Status.ContainerStatuses {
		if !status.Ready {
			allContainersReady = false
			break
		}
	}

	if cond, exists := conditions[corev1.PodReady]; !exists || cond.Status != corev1.ConditionTrue {
		if cond == nil {
			cond = &corev1.PodCondition{
				Type:               corev1.PodReady,
				LastTransitionTime: now,
			}
			pod.Status.Conditions = append(pod.Status.Conditions, *cond)
			conditions[corev1.PodReady] = cond
		}
		if allContainersReady && pod.Status.Phase == corev1.PodRunning {
			cond.Status = corev1.ConditionTrue
			cond.Reason = "ContainersReady"
			cond.Message = "All containers are ready"
		} else {
			cond.Status = corev1.ConditionFalse
			cond.Reason = "ContainersNotReady"
			cond.Message = "Not all containers are ready"
		}
		cond.LastTransitionTime = now
	}
}

// updatePodPhase 更新 Pod 阶段
func (pc *PodController) updatePodPhase(pod *corev1.Pod) {
	// 如果 Pod 已经被删除，设置为 Terminating
	if pod.DeletionTimestamp != nil {
		if pod.Status.Phase != corev1.PodFailed && pod.Status.Phase != corev1.PodSucceeded {
			pod.Status.Phase = corev1.PodRunning // 在删除过程中保持 Running，直到容器停止
		}
		return
	}

	// 根据容器状态更新阶段
	if len(pod.Status.ContainerStatuses) == 0 {
		if pod.Spec.NodeName == "" {
			pod.Status.Phase = corev1.PodPending
		} else {
			pod.Status.Phase = corev1.PodPending
		}
		return
	}

	// 检查所有容器的状态
	hasRunning := false
	hasWaiting := false
	allTerminated := true

	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Running != nil {
			hasRunning = true
			allTerminated = false
		} else if status.State.Waiting != nil {
			hasWaiting = true
			allTerminated = false
		} else if status.State.Terminated != nil {
			// 有终止的容器，但需要检查是否全部终止
		} else {
			allTerminated = false
		}
	}

	// 根据容器状态确定 Pod 阶段
	if allTerminated {
		// 检查退出码
		allSucceeded := true
		for _, status := range pod.Status.ContainerStatuses {
			if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
				allSucceeded = false
				break
			}
		}
		if allSucceeded {
			pod.Status.Phase = corev1.PodSucceeded
		} else {
			pod.Status.Phase = corev1.PodFailed
		}
	} else if hasRunning {
		pod.Status.Phase = corev1.PodRunning
	} else if hasWaiting {
		pod.Status.Phase = corev1.PodPending
	} else if pod.Spec.NodeName == "" {
		pod.Status.Phase = corev1.PodPending
	} else {
		pod.Status.Phase = corev1.PodPending
	}
}

// handlePodDeleted 处理 Pod 删除
func (pc *PodController) handlePodDeleted(ctx context.Context, pod *corev1.Pod) error {
	pc.logger.Infof("Pod %s/%s 已被删除，清理相关资源", pod.Namespace, pod.Name)
	// 这里可以添加清理逻辑，比如清理相关的 Service、ConfigMap 等
	return nil
}
