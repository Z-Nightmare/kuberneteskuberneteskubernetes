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
)

// RuntimeController 容器运行时控制器，负责启动和管理容器
type RuntimeController struct {
	store   storage.Store
	logger  logprovider.Logger
	runtime ContainerRuntime
	stopCh  chan struct{}
}

// NewRuntimeController 创建容器运行时控制器
func NewRuntimeController(store storage.Store, logger logprovider.Logger) (*RuntimeController, error) {
	// 检测可用的容器运行时
	detector := NewRuntimeDetector(logger)
	runtime, err := detector.DetectRuntime()
	if err != nil {
		return nil, fmt.Errorf("无法检测容器运行时: %w", err)
	}

	return &RuntimeController{
		store:   store,
		logger:  logger,
		runtime: runtime,
		stopCh:  make(chan struct{}),
	}, nil
}

// Name 返回控制器名称
func (rc *RuntimeController) Name() string {
	return fmt.Sprintf("RuntimeController(%s)", rc.runtime.Name())
}

// Start 启动容器运行时控制器
func (rc *RuntimeController) Start(ctx context.Context) error {
	rc.logger.Infof("启动容器运行时控制器: %s", rc.runtime.Name())

	podGVK := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	// 监听 Pod 资源变化
	watchCh, err := rc.store.Watch(podGVK, "", "")
	if err != nil {
		return fmt.Errorf("无法监听 Pod 资源: %w", err)
	}

	// 启动处理循环
	go rc.processPods(ctx, watchCh)

	// 处理现有的已调度但未运行的 Pod
	if err := rc.syncPendingPods(ctx); err != nil {
		rc.logger.Error("同步待运行 Pod 失败: ", err.Error())
	}

	return nil
}

// Stop 停止容器运行时控制器
func (rc *RuntimeController) Stop(ctx context.Context) error {
	rc.logger.Info("停止容器运行时控制器...")
	close(rc.stopCh)
	return nil
}

// syncPendingPods 同步待运行的 Pod
func (rc *RuntimeController) syncPendingPods(ctx context.Context) error {
	podGVK := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	pods, err := rc.store.List(podGVK, "")
	if err != nil {
		return err
	}

	rc.logger.Infof("发现 %d 个 Pod，检查待运行状态...", len(pods))

	for _, obj := range pods {
		if pod, ok := obj.(*corev1.Pod); ok {
			// 只处理已调度到当前节点且未运行的 Pod
			if pod.Spec.NodeName != "" && pod.Status.Phase != corev1.PodRunning {
				rc.logger.Infof("发现待运行 Pod: %s/%s (节点: %s)", pod.Namespace, pod.Name, pod.Spec.NodeName)
				if err := rc.handlePod(ctx, pod); err != nil {
					rc.logger.Error("处理 Pod 失败: ", pod.Name, " error: ", err.Error())
				}
			}
		}
	}

	return nil
}

// processPods 处理 Pod 事件
func (rc *RuntimeController) processPods(ctx context.Context, watchCh <-chan storage.ResourceEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-rc.stopCh:
			return
		case event, ok := <-watchCh:
			if !ok {
				rc.logger.Warn("Pod watch 通道已关闭")
				return
			}

			switch event.Type {
			case storage.EventAdded, storage.EventModified:
				if pod, ok := event.Object.(*corev1.Pod); ok {
					// 只处理已调度到当前节点且未运行的 Pod
					if pod.Spec.NodeName != "" && pod.Status.Phase != corev1.PodRunning {
						rc.logger.Infof("处理 Pod 事件: %s/%s (%s)", pod.Namespace, pod.Name, event.Type)
						if err := rc.handlePod(ctx, pod); err != nil {
							rc.logger.Error("处理 Pod 失败: ", pod.Name, " error: ", err.Error())
						}
					}
				}
			case storage.EventDeleted:
				if pod, ok := event.Object.(*corev1.Pod); ok {
					rc.logger.Infof("删除 Pod: %s/%s", pod.Namespace, pod.Name)
					stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()
					if err := rc.runtime.StopContainer(stopCtx, pod); err != nil {
						rc.logger.Error("停止容器失败: ", err.Error())
					}
				}
			}
		}
	}
}

// handlePod 处理 Pod（启动或更新容器）
func (rc *RuntimeController) handlePod(ctx context.Context, pod *corev1.Pod) error {
	// 检查容器状态
	status, err := rc.runtime.GetContainerStatus(ctx, pod)
	if err != nil {
		rc.logger.Warnf("获取容器状态失败: %v", err)
	}

	// 如果容器未运行，启动它
	if !status.Running {
		rc.logger.Infof("启动 Pod 容器: %s/%s", pod.Namespace, pod.Name)
		startCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if err := rc.runtime.StartContainer(startCtx, pod); err != nil {
			return fmt.Errorf("启动容器失败: %w", err)
		}

		// 更新 Pod 状态
		pod.Status.Phase = corev1.PodRunning
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{
			{
				Name:  pod.Spec.Containers[0].Name,
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()}},
				Ready: true,
			},
		}
		pod.Status.Conditions = []corev1.PodCondition{
			{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "ContainersReady",
				Message:            "All containers are ready",
			},
		}

		// 更新 Pod 资源
		podGVK := schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		}

		if err := rc.store.Update(podGVK, pod); err != nil {
			return fmt.Errorf("更新 Pod 状态失败: %w", err)
		}

		rc.logger.Infof("Pod %s/%s 已成功启动", pod.Namespace, pod.Name)
	} else {
		rc.logger.Debugf("Pod %s/%s 容器已在运行", pod.Namespace, pod.Name)
	}

	return nil
}
