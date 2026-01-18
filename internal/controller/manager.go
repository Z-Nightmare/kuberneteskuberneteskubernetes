package controller

import (
	"context"
	"os"
	"time"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/config"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// ControllerManager 管理所有控制器
type ControllerManager struct {
	store       storage.Store
	logger      logprovider.Logger
	config      config.Config
	nodeName    string
	controllers []Controller
}

// Controller 是控制器的接口
type Controller interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Name() string
}

// NewControllerManager 创建控制器管理器
func NewControllerManager(
	store storage.Store,
	logger logprovider.Logger,
	config config.Config,
) *ControllerManager {
	// 获取节点名称（优先使用环境变量，否则使用主机名）
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			logger.Warn("无法获取主机名，使用默认节点名: node-1")
			nodeName = "node-1"
		} else {
			nodeName = hostname
		}
	}

	cm := &ControllerManager{
		store:    store,
		logger:   logger,
		config:   config,
		nodeName: nodeName,
	}

	// 注册所有控制器
	cm.registerControllers()

	return cm
}

// registerControllers 注册所有控制器
func (cm *ControllerManager) registerControllers() {
	// 注册 Pod 控制器（优先注册，负责 Pod 生命周期管理）
	podController := NewPodController(cm.store, cm.logger)
	cm.controllers = append(cm.controllers, podController)

	// 注册 Deployment 控制器
	deploymentController := NewDeploymentController(cm.store, cm.logger)
	cm.controllers = append(cm.controllers, deploymentController)

	// 注册 Scheduler 控制器
	schedulerController := NewSchedulerController(cm.store, cm.logger)
	cm.controllers = append(cm.controllers, schedulerController)

	// 注册容器运行时控制器
	runtimeController, err := NewRuntimeController(cm.store, cm.logger)
	if err != nil {
		cm.logger.Warnf("无法创建容器运行时控制器: %v", err)
		cm.logger.Warn("容器运行时功能将不可用")
	} else {
		cm.controllers = append(cm.controllers, runtimeController)
		cm.logger.Infof("容器运行时控制器已注册: %s", runtimeController.Name())
	}
}

// Start 启动控制器管理器
func (cm *ControllerManager) Start(ctx context.Context) error {
	cm.logger.Info("启动控制器管理器...")

	// 首先上报当前节点
	if err := cm.reportNode(ctx); err != nil {
		cm.logger.Error("上报节点失败: ", err.Error())
		return err
	}

	// 启动所有控制器
	for _, controller := range cm.controllers {
		cm.logger.Infof("启动控制器: %s", controller.Name())
		go func(c Controller) {
			if err := c.Start(ctx); err != nil {
				cm.logger.Error("控制器启动失败: ", c.Name(), " error: ", err.Error())
			}
		}(controller)
	}

	cm.logger.Info("控制器管理器启动完成")
	return nil
}

// Stop 停止控制器管理器
func (cm *ControllerManager) Stop(ctx context.Context) error {
	cm.logger.Info("停止控制器管理器...")
	for _, controller := range cm.controllers {
		if err := controller.Stop(ctx); err != nil {
			cm.logger.Error("停止控制器失败: ", controller.Name(), " error: ", err.Error())
		}
	}
	return nil
}

// reportNode 上报当前节点信息
func (cm *ControllerManager) reportNode(ctx context.Context) error {
	cm.logger.Infof("上报节点: %s", cm.nodeName)

	// 获取节点信息
	node := &corev1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: cm.nodeName,
			Labels: map[string]string{
				"kubernetes.io/hostname": cm.nodeName,
			},
			CreationTimestamp: metav1.Now(),
		},
		Status: corev1.NodeStatus{
			Phase: corev1.NodeRunning,
			Conditions: []corev1.NodeCondition{
				{
					Type:               corev1.NodeReady,
					Status:             corev1.ConditionTrue,
					LastHeartbeatTime:  metav1.Now(),
					LastTransitionTime: metav1.Now(),
					Reason:             "KubeletReady",
					Message:            "kubelet is posting ready status",
				},
			},
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeHostName,
					Address: cm.nodeName,
				},
			},
		},
	}

	// 设置 UID
	if node.UID == "" {
		node.UID = types.UID("node-" + cm.nodeName)
	}

	// 获取 Node 的 GVK
	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Node",
	}

	// 检查节点是否已存在
	existingNode, err := cm.store.Get(gvk, "", cm.nodeName)
	if err != nil {
		// 节点不存在，创建新节点
		cm.logger.Infof("创建新节点: %s", cm.nodeName)
		if err := cm.store.Create(gvk, node); err != nil {
			return err
		}
	} else {
		// 节点已存在，更新节点信息
		cm.logger.Infof("更新节点: %s", cm.nodeName)
		// 更新心跳时间
		if existingNodeNode, ok := existingNode.(*corev1.Node); ok {
			node.Status.Conditions = existingNodeNode.Status.Conditions
			// 更新心跳时间
			for i := range node.Status.Conditions {
				if node.Status.Conditions[i].Type == corev1.NodeReady {
					node.Status.Conditions[i].LastHeartbeatTime = metav1.Now()
				}
			}
		}
		if err := cm.store.Update(gvk, node); err != nil {
			return err
		}
	}

	cm.logger.Infof("节点上报成功: %s", cm.nodeName)
	return nil
}

// StartNodeHeartbeat 启动节点心跳上报
func (cm *ControllerManager) StartNodeHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := cm.reportNode(ctx); err != nil {
				cm.logger.Error("节点心跳上报失败: ", err.Error())
			}
		}
	}
}
