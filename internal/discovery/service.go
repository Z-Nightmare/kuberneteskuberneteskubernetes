package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/controller"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
	"github.com/hashicorp/consul/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// Settings 配置结构
type Settings struct {
	// ConsulAddress Consul 服务器地址
	ConsulAddress string
	// ConsulToken Consul ACL token（可选）
	ConsulToken string
	// ServiceName 服务名称
	ServiceName string
	// ServiceID 服务 ID（默认使用 NodeName）
	ServiceID string
	// ServiceTags 服务标签
	ServiceTags []string
	// ServicePort 服务端口
	ServicePort int
	// HealthCheckInterval 健康检查间隔
	HealthCheckInterval time.Duration
	// HealthCheckTimeout 健康检查超时
	HealthCheckTimeout time.Duration
	// DeregisterCriticalServiceAfter 服务不健康后多久注销
	DeregisterCriticalServiceAfter time.Duration
	// NodeName 节点名称
	NodeName string
	// RegisterSelf 是否将当前节点注册到 store
	RegisterSelf bool
	// WatchInterval 从 Consul 发现服务并同步到 store 的间隔
	WatchInterval time.Duration
	// AutoStartConsul 如果 Consul 不可用，是否自动启动 Consul 容器（仅当 ConsulAddress 指向 localhost 时生效）
	AutoStartConsul bool
}

// ConsulContainerHandle 记录由本进程"自动拉起"的 Consul 容器信息
type ConsulContainerHandle struct {
	Runtime controller.ContainerRuntime
	Pod     *corev1.Pod
	Started bool // 是否由本进程拉起
}

// Service 服务发现服务
type Service struct {
	store        storage.Store
	logger       logprovider.Logger
	settings     Settings
	consulClient *api.Client

	mu        sync.Mutex
	cancelBg  context.CancelFunc
	serviceID string

	httpServer *http.Server

	// consulContainer 如果 Consul 容器由本进程启动，记录容器信息
	consulContainer *ConsulContainerHandle
}

// NewService 创建服务发现服务
func NewService(store storage.Store, logger logprovider.Logger, settings Settings) (*Service, error) {
	// 设置默认值
	if strings.TrimSpace(settings.ConsulAddress) == "" {
		settings.ConsulAddress = "localhost:8500"
	}
	if strings.TrimSpace(settings.ServiceName) == "" {
		settings.ServiceName = "k3-node"
	}
	if strings.TrimSpace(settings.NodeName) == "" {
		settings.NodeName = defaultNodeName(logger)
	}
	if settings.ServiceID == "" {
		settings.ServiceID = fmt.Sprintf("%s-%s", settings.ServiceName, settings.NodeName)
	}
	if settings.ServicePort == 0 {
		settings.ServicePort = 7946
	}
	if settings.HealthCheckInterval <= 0 {
		settings.HealthCheckInterval = 10 * time.Second
	}
	if settings.HealthCheckTimeout <= 0 {
		settings.HealthCheckTimeout = 3 * time.Second
	}
	if settings.DeregisterCriticalServiceAfter <= 0 {
		settings.DeregisterCriticalServiceAfter = 30 * time.Second
	}
	if settings.WatchInterval <= 0 {
		settings.WatchInterval = 15 * time.Second
	}

	// 创建 Consul 客户端
	config := api.DefaultConfig()
	config.Address = settings.ConsulAddress
	if settings.ConsulToken != "" {
		config.Token = settings.ConsulToken
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("创建 Consul 客户端失败: %w", err)
	}

	return &Service{
		store:        store,
		logger:       logger,
		settings:     settings,
		consulClient: client,
		serviceID:    settings.ServiceID,
	}, nil
}

// Start 启动服务发现服务
func (s *Service) Start(ctx context.Context) error {
	bgCtx, cancel := context.WithCancel(context.Background())
	s.cancelBg = cancel

	// 如果启用自动启动 Consul，确保 Consul 容器运行
	if s.settings.AutoStartConsul {
		if err := s.ensureConsulRunning(ctx); err != nil {
			s.logger.Warnf("自动启动 Consul 容器失败: %v，将继续尝试连接现有 Consul", err)
		}
	}

	// 启动健康检查 HTTP 服务器
	if err := s.startHealthServer(); err != nil {
		return fmt.Errorf("启动健康检查服务器失败: %w", err)
	}

	// 注册当前服务到 Consul
	if err := s.registerService(bgCtx); err != nil {
		return fmt.Errorf("注册服务到 Consul 失败: %w", err)
	}

	// 如果启用，将当前节点注册到 store
	if s.settings.RegisterSelf {
		if err := s.registerSelfNode(); err != nil {
			s.logger.Warnf("注册当前节点到 store 失败: %v", err)
		}
		// 启动心跳循环
		go s.heartbeatLoop(bgCtx)
	}

	// 启动服务发现循环
	go s.discoveryLoop(bgCtx)

	s.logger.Infof("discovery: 已启动 (consul=%s, service=%s, node=%s, health=%s:%d)",
		s.settings.ConsulAddress, s.settings.ServiceName, s.settings.NodeName,
		"0.0.0.0", s.settings.ServicePort)
	return nil
}

// Stop 停止服务发现服务
func (s *Service) Stop(ctx context.Context) error {
	if s.cancelBg != nil {
		s.cancelBg()
	}

	// 停止健康检查服务器
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.logger.Warnf("停止健康检查服务器失败: %v", err)
		}
	}

	// 从 Consul 注销服务
	if err := s.deregisterService(ctx); err != nil {
		s.logger.Warnf("从 Consul 注销服务失败: %v", err)
	}

	// 如果 Consul 容器是由本进程启动的，停止它
	if s.consulContainer != nil && s.consulContainer.Started {
		if s.consulContainer.Runtime != nil && s.consulContainer.Pod != nil {
			if err := s.consulContainer.Runtime.StopContainer(ctx, s.consulContainer.Pod); err != nil {
				s.logger.Warnf("停止 Consul 容器失败: %v", err)
			} else {
				s.logger.Infof("已停止 Consul 容器: %s", s.consulContainer.Pod.Name)
			}
		}
	}

	s.logger.Info("discovery: 已停止")
	return nil
}

// startHealthServer 启动健康检查 HTTP 服务器
func (s *Service) startHealthServer() error {
	addr := fmt.Sprintf(":%d", s.settings.ServicePort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听地址失败: %w", err)
	}

	s.httpServer = &http.Server{
		Handler: s.healthMux(),
	}

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Warnf("健康检查服务器停止: %v", err)
		}
	}()

	return nil
}

// healthMux 创建健康检查 HTTP 处理器
func (s *Service) healthMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		localIPs := localIPv4s()
		info := map[string]interface{}{
			"node":      s.settings.NodeName,
			"service":   s.settings.ServiceName,
			"serviceID": s.serviceID,
			"port":      s.settings.ServicePort,
			"pid":       os.Getpid(),
			"addrs":     localIPs,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(info)
	})
	return mux
}

// registerService 注册服务到 Consul
func (s *Service) registerService(ctx context.Context) error {
	// 获取本地 IP 地址
	localIPs := localIPv4s()
	if len(localIPs) == 0 {
		return fmt.Errorf("无法获取本地 IP 地址")
	}
	serviceAddress := localIPs[0].String()

	// 构建健康检查
	healthCheck := &api.AgentServiceCheck{
		Interval:                       s.settings.HealthCheckInterval.String(),
		Timeout:                        s.settings.HealthCheckTimeout.String(),
		DeregisterCriticalServiceAfter: s.settings.DeregisterCriticalServiceAfter.String(),
		HTTP:                           fmt.Sprintf("http://%s:%d/healthz", serviceAddress, s.settings.ServicePort),
	}

	// 构建服务注册信息
	registration := &api.AgentServiceRegistration{
		ID:      s.serviceID,
		Name:    s.settings.ServiceName,
		Tags:    s.settings.ServiceTags,
		Port:    s.settings.ServicePort,
		Address: serviceAddress,
		Check:   healthCheck,
		Meta: map[string]string{
			"node": s.settings.NodeName,
			"pid":  strconv.Itoa(os.Getpid()),
		},
	}

	// 注册服务
	if err := s.consulClient.Agent().ServiceRegister(registration); err != nil {
		return fmt.Errorf("注册服务失败: %w", err)
	}

	s.logger.Infof("已注册服务到 Consul: %s (ID: %s, Address: %s:%d)",
		s.settings.ServiceName, s.serviceID, serviceAddress, s.settings.ServicePort)
	return nil
}

// deregisterService 从 Consul 注销服务
func (s *Service) deregisterService(ctx context.Context) error {
	if err := s.consulClient.Agent().ServiceDeregister(s.serviceID); err != nil {
		return fmt.Errorf("注销服务失败: %w", err)
	}
	s.logger.Infof("已从 Consul 注销服务: %s", s.serviceID)
	return nil
}

// discoveryLoop 服务发现循环
func (s *Service) discoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(s.settings.WatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.discoverAndSyncServices(ctx); err != nil {
				s.logger.Warnf("服务发现失败: %v", err)
			}
		}
	}
}

// discoverAndSyncServices 发现服务并同步到 store
func (s *Service) discoverAndSyncServices(ctx context.Context) error {
	// 从 Consul 获取所有服务
	services, _, err := s.consulClient.Catalog().Service(s.settings.ServiceName, "", nil)
	if err != nil {
		return fmt.Errorf("获取服务列表失败: %w", err)
	}

	s.logger.Debugf("从 Consul 发现 %d 个服务实例", len(services))

	// 将服务同步到 store 作为 Node 资源
	for _, svc := range services {
		if err := s.syncServiceToNode(svc); err != nil {
			s.logger.Warnf("同步服务到 Node 失败: %s: %v", svc.ServiceID, err)
		}
	}

	return nil
}

// syncServiceToNode 将 Consul 服务同步为 Kubernetes Node
func (s *Service) syncServiceToNode(svc *api.CatalogService) error {
	// 从 Meta 或 ServiceID 中提取节点名称
	nodeName := svc.ServiceMeta["node"]
	if nodeName == "" {
		// 从 ServiceID 中提取（格式：service-name-node-name）
		parts := strings.Split(svc.ServiceID, "-")
		if len(parts) > 1 {
			nodeName = strings.Join(parts[1:], "-")
		} else {
			nodeName = svc.ServiceID
		}
	}

	// 解析服务地址
	var addresses []corev1.NodeAddress
	if svc.ServiceAddress != "" {
		addresses = append(addresses, corev1.NodeAddress{
			Type:    corev1.NodeInternalIP,
			Address: svc.ServiceAddress,
		})
	}
	if svc.Address != "" && svc.Address != svc.ServiceAddress {
		addresses = append(addresses, corev1.NodeAddress{
			Type:    corev1.NodeHostName,
			Address: svc.Address,
		})
	}

	// 检查节点健康状态
	isReady := true
	for _, check := range svc.Checks {
		if check.Status != "passing" {
			isReady = false
			break
		}
	}

	// 创建或更新 Node
	node := &corev1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				"kubernetes.io/hostname": nodeName,
				"discovery":              "consul",
			},
			CreationTimestamp: metav1.Now(),
		},
		Status: corev1.NodeStatus{
			Addresses: addresses,
			Conditions: []corev1.NodeCondition{
				{
					Type:               corev1.NodeReady,
					Status:             func() corev1.ConditionStatus { if isReady { return corev1.ConditionTrue } else { return corev1.ConditionFalse } }(),
					LastHeartbeatTime:  metav1.Now(),
					LastTransitionTime: metav1.Now(),
					Reason:             "ConsulHealthCheck",
					Message:            fmt.Sprintf("Service registered via Consul: %s", svc.ServiceID),
				},
			},
		},
	}

	// 设置 UID
	if node.UID == "" {
		node.UID = types.UID("consul-node-" + nodeName)
	}

	// 获取 Node 的 GVK
	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Node",
	}

	// 检查节点是否已存在
	existingNode, err := s.store.Get(gvk, "", nodeName)
	if err != nil {
		// 节点不存在，创建新节点
		if err := s.store.Create(gvk, node); err != nil {
			return fmt.Errorf("创建节点失败: %w", err)
		}
		s.logger.Debugf("已创建节点: %s (来自 Consul 服务: %s)", nodeName, svc.ServiceID)
	} else {
		// 节点已存在，更新节点信息
		if existingNodeNode, ok := existingNode.(*corev1.Node); ok {
			// 保留原有的条件，更新心跳时间
			node.Status.Conditions = existingNodeNode.Status.Conditions
			for i := range node.Status.Conditions {
				if node.Status.Conditions[i].Type == corev1.NodeReady {
					node.Status.Conditions[i].LastHeartbeatTime = metav1.Now()
					if isReady {
						node.Status.Conditions[i].Status = corev1.ConditionTrue
					} else {
						node.Status.Conditions[i].Status = corev1.ConditionFalse
					}
				}
			}
		}
		if err := s.store.Update(gvk, node); err != nil {
			return fmt.Errorf("更新节点失败: %w", err)
		}
		s.logger.Debugf("已更新节点: %s (来自 Consul 服务: %s)", nodeName, svc.ServiceID)
	}

	return nil
}

// registerSelfNode 注册当前节点到 store
func (s *Service) registerSelfNode() error {
	localIPs := localIPv4s()
	if len(localIPs) == 0 {
		return fmt.Errorf("无法获取本地 IP 地址")
	}

	node := &corev1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: s.settings.NodeName,
			Labels: map[string]string{
				"kubernetes.io/hostname": s.settings.NodeName,
				"discovery":              "consul",
			},
			CreationTimestamp: metav1.Now(),
		},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: localIPs[0].String(),
				},
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:               corev1.NodeReady,
					Status:             corev1.ConditionTrue,
					LastHeartbeatTime:  metav1.Now(),
					LastTransitionTime: metav1.Now(),
					Reason:             "ConsulRegistered",
					Message:            "Node registered via Consul discovery",
				},
			},
		},
	}

	// 设置 UID
	if node.UID == "" {
		node.UID = types.UID("consul-node-" + s.settings.NodeName)
	}

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Node",
	}

	// 检查节点是否已存在
	existingNode, err := s.store.Get(gvk, "", s.settings.NodeName)
	if err != nil {
		// 节点不存在，创建新节点
		if err := s.store.Create(gvk, node); err != nil {
			return fmt.Errorf("创建节点失败: %w", err)
		}
		s.logger.Infof("已注册当前节点到 store: %s", s.settings.NodeName)
	} else {
		// 节点已存在，更新节点信息
		if existingNodeNode, ok := existingNode.(*corev1.Node); ok {
			node.Status.Conditions = existingNodeNode.Status.Conditions
			for i := range node.Status.Conditions {
				if node.Status.Conditions[i].Type == corev1.NodeReady {
					node.Status.Conditions[i].LastHeartbeatTime = metav1.Now()
				}
			}
		}
		if err := s.store.Update(gvk, node); err != nil {
			return fmt.Errorf("更新节点失败: %w", err)
		}
		s.logger.Debugf("已更新当前节点: %s", s.settings.NodeName)
	}

	return nil
}

// heartbeatLoop 心跳循环
func (s *Service) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.registerSelfNode(); err != nil {
				s.logger.Warnf("心跳更新节点失败: %v", err)
			}
		}
	}
}

// localIPv4s 获取本地 IPv4 地址
func localIPv4s() []net.IP {
	var ips []net.IP
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ips = append(ips, ipNet.IP)
			}
		}
	}
	return ips
}

// defaultNodeName 获取默认节点名称
func defaultNodeName(logger logprovider.Logger) string {
	if name := os.Getenv("NODE_NAME"); name != "" {
		return name
	}
	hostname, err := os.Hostname()
	if err != nil {
		logger.Warn("无法获取主机名，使用默认节点名: node-1")
		return "node-1"
	}
	return hostname
}

// isLocalHost 判断 host 是否为本机回环地址
func isLocalHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	return h == "localhost" || h == "127.0.0.1" || h == "::1" || h == ""
}

// parseConsulAddress 解析 Consul 地址，返回 host 和 port
func parseConsulAddress(addr string) (host string, port int) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "localhost", 8500
	}

	// 处理格式：host:port
	parts := strings.Split(addr, ":")
	if len(parts) == 2 {
		host = parts[0]
		if p, err := strconv.Atoi(parts[1]); err == nil {
			port = p
		} else {
			port = 8500
		}
	} else {
		host = addr
		port = 8500
	}

	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 8500
	}

	return host, port
}

// buildConsulPod 构造用于拉起 Consul 容器的 Pod 描述
func buildConsulPod(consulAddress string) *corev1.Pod {
	_, port := parseConsulAddress(consulAddress)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "discovery",
			Name:      "consul",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "consul",
					Image: "consul:1.17",
					Args: []string{
						"agent",
						"-dev",
						"-client", "0.0.0.0",
						"-ui",
					},
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 8500,
							HostPort:      int32(port),
							Name:          "http",
							Protocol:      corev1.ProtocolTCP,
						},
						{
							ContainerPort: 8600,
							HostPort:      8600,
							Name:          "dns",
							Protocol:      corev1.ProtocolUDP,
						},
					},
				},
			},
		},
	}
}

// ensureConsulRunning 确保 Consul 容器运行
// 仅当 ConsulAddress 指向 localhost 且 AutoStartConsul 为 true 时才会启动容器
func (s *Service) ensureConsulRunning(ctx context.Context) error {
	consulHost, port := parseConsulAddress(s.settings.ConsulAddress)

	// 安全检查：仅当 Consul 地址指向本机时才自动启动容器
	if !isLocalHost(consulHost) {
		s.logger.Debugf("Consul 地址 %s 不是本机地址，跳过自动启动容器", consulHost)
		return nil
	}

	// 检查 Consul 是否已经可用
	readyAddr := net.JoinHostPort(consulHost, strconv.Itoa(port))
	if s.checkConsulAvailable(readyAddr) {
		s.logger.Infof("Consul 已在运行: %s", readyAddr)
		return nil
	}

	// 检测容器运行时
	detector := controller.NewRuntimeDetector(s.logger)
	runtime, err := detector.DetectRuntime()
	if err != nil {
		return fmt.Errorf("未检测到可用容器运行时: %w", err)
	}

	// 构建 Consul Pod
	pod := buildConsulPod(s.settings.ConsulAddress)

	// 检查容器是否已在运行
	status, _ := runtime.GetContainerStatus(ctx, pod)
	if status.Running {
		s.logger.Infof("检测到 Consul 容器已在运行 (runtime=%s, status=%s)，跳过拉起", runtime.Name(), status.Status)
		s.consulContainer = &ConsulContainerHandle{
			Runtime: runtime,
			Pod:     pod,
			Started: false,
		}
		return nil
	}

	// 清理同名旧容器
	_ = runtime.StopContainer(ctx, pod)

	s.logger.Infof("准备通过 %s 拉起 Consul 容器...", runtime.Name())
	if err := runtime.StartContainer(ctx, pod); err != nil {
		return fmt.Errorf("启动 Consul 容器失败: %w", err)
	}

	// 等待 Consul 就绪
	if err := s.waitForConsul(readyAddr, 60*time.Second); err != nil {
		_ = runtime.StopContainer(ctx, pod)
		return fmt.Errorf("等待 Consul 就绪超时: %w", err)
	}

	s.logger.Infof("Consul 容器已就绪: %s", readyAddr)
	s.consulContainer = &ConsulContainerHandle{
		Runtime: runtime,
		Pod:     pod,
		Started: true,
	}

	return nil
}

// checkConsulAvailable 检查 Consul 是否可用
func (s *Service) checkConsulAvailable(addr string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://%s/v1/status/leader", addr)
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// waitForConsul 等待 Consul 就绪
func (s *Service) waitForConsul(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.checkConsulAvailable(addr) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("等待 Consul 就绪超时: %s", addr)
}
