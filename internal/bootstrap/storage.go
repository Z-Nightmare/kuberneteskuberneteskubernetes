package bootstrap

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/controller"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/config"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
)

// DBContainerHandle 记录由本进程“自动拉起”的数据库容器信息。
//
// - Runtime: 当前检测到并用于拉起容器的运行时实现（目前真正可用的是 DockerRuntime）
// - Pod: 复用已有 runtime 控制器的 “Pod 结构 -> docker run 参数” 映射来描述要运行的容器
// - Started: 标记容器是否由本进程启动（用于退出时是否清理）
type DBContainerHandle struct {
	Runtime controller.ContainerRuntime
	Pod     *corev1.Pod
	Started bool // 是否由本进程拉起
}

// ProvideDBContainerHandle 会根据配置与当前宿主机容器运行时环境，按需拉起 MySQL/Etcd 容器。
//
// 约束（避免误操作）：
// - 仅当配置指向本机地址（localhost/127.0.0.1/::1）时才会尝试拉起容器
// - 若容器已在运行则不会重复启动
// - 若未检测到可用运行时，会降级跳过（允许用户自己提前启动数据库）
func ProvideDBContainerHandle(cfg config.Config, l logprovider.Logger) (*DBContainerHandle, error) {
	storageType := strings.ToLower(strings.TrimSpace(cfg.Storage.Type))

	switch storageType {
	case "mysql":
		if !isLocalHost(cfg.Storage.MySQL.Host) {
			return &DBContainerHandle{}, nil
		}
		if cfg.Storage.MySQL.Port <= 0 {
			return &DBContainerHandle{}, nil
		}

		pod := buildMySQLPod(cfg)
		readyAddr := net.JoinHostPort(cfg.Storage.MySQL.Host, strconv.Itoa(cfg.Storage.MySQL.Port))
		return ensureContainerRunningAndWait(l, pod, readyAddr)

	case "etcd":
		endpoint, readyAddr, ok := firstLocalEtcdEndpoint(cfg.Storage.Etcd.Endpoints)
		if !ok {
			return &DBContainerHandle{}, nil
		}
		pod, err := buildEtcdPodFromEndpoint(endpoint)
		if err != nil {
			l.Warnf("Etcd endpoint 解析失败，跳过自动拉起容器: %v", err)
			return &DBContainerHandle{}, nil
		}
		return ensureContainerRunningAndWait(l, pod, readyAddr)

	default:
		return &DBContainerHandle{}, nil
	}
}

// ProvideStore 创建存储实现。
//
// 注意：这里依赖注入了 DBContainerHandle（即使未使用），是为了确保初始化顺序：
// 先拉起/等待 DB 容器就绪，再 NewStore() 连接数据库。
func ProvideStore(cfg config.Config, _ *DBContainerHandle) (storage.Store, error) {
	return storage.NewStore(cfg.Storage)
}

// ensureContainerRunningAndWait 负责：
// - 检测容器运行时
// - 若目标容器未运行则启动
// - 等待指定 TCP 端口就绪（表示服务可连接）
//
// readyAddr 一般为 "host:port"（如 "127.0.0.1:3306" / "127.0.0.1:2379"）。
func ensureContainerRunningAndWait(l logprovider.Logger, pod *corev1.Pod, readyAddr string) (*DBContainerHandle, error) {
	detector := controller.NewRuntimeDetector(l)
	runtime, err := detector.DetectRuntime()
	if err != nil {
		// best-effort：如果用户本机已经有 MySQL/Etcd 进程在跑，不强制要求运行时
		l.Warnf("未检测到可用容器运行时，跳过自动拉起容器: %v", err)
		return &DBContainerHandle{}, nil
	}

	status, _ := runtime.GetContainerStatus(context.Background(), pod)
	if status.Running {
		l.Infof("检测到目标容器已在运行 (runtime=%s, status=%s)，跳过拉起", runtime.Name(), status.Status)
		return &DBContainerHandle{Runtime: runtime, Pod: pod, Started: false}, nil
	}

	// 清理同名旧容器（Exited 等），避免 docker run --name 冲突
	_ = runtime.StopContainer(context.Background(), pod)

	l.Infof("准备通过 %s 拉起容器以提供存储后端...", runtime.Name())
	if err := runtime.StartContainer(context.Background(), pod); err != nil {
		return nil, err
	}

	if err := waitForTCP(readyAddr, 60*time.Second); err != nil {
		_ = runtime.StopContainer(context.Background(), pod)
		return nil, err
	}

	l.Infof("存储后端容器已就绪: %s", readyAddr)
	return &DBContainerHandle{Runtime: runtime, Pod: pod, Started: true}, nil
}

// waitForTCP 在指定超时时间内轮询探测 addr 的 TCP 连通性。
// 成功连接并立即关闭即视为“端口就绪”，否则超时返回错误。
func waitForTCP(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("等待端口就绪超时: %s: %w", addr, lastErr)
}

// isLocalHost 判断 host 是否为本机回环地址。
// 用于限制“自动拉起容器”只作用于本机数据库场景，避免误操作远端数据库。
func isLocalHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}

// firstLocalEtcdEndpoint 从 endpoints 中挑选第一个指向本机的 endpoint，并返回：
// - endpoint: 原始 endpoint 字符串
// - readyAddr: 解析后的 "host:port"（默认端口 2379）
// - ok: 是否找到
func firstLocalEtcdEndpoint(endpoints []string) (endpoint string, readyAddr string, ok bool) {
	for _, ep := range endpoints {
		u, err := url.Parse(strings.TrimSpace(ep))
		if err != nil || u.Host == "" {
			continue
		}
		host := u.Hostname()
		if !isLocalHost(host) {
			continue
		}
		port := u.Port()
		if port == "" {
			port = "2379"
		}
		return ep, net.JoinHostPort(host, port), true
	}
	return "", "", false
}

// buildMySQLPod 构造用于拉起 MySQL 容器的 Pod 描述。
//
// 说明：
// - 该 Pod 不会进入 Kubernetes 调度，仅作为 runtime(Docker) 的输入描述
// - HostPort 使用配置中的 mysql.port，容器镜像固定为 mysql:8.0
func buildMySQLPod(cfg config.Config) *corev1.Pod {
	mysqlCfg := cfg.Storage.MySQL

	env := []corev1.EnvVar{
		{Name: "MYSQL_DATABASE", Value: mysqlCfg.Database},
	}

	// mysql 镜像至少需要 MYSQL_ROOT_PASSWORD
	if strings.TrimSpace(mysqlCfg.Password) == "" {
		env = append(env, corev1.EnvVar{Name: "MYSQL_ROOT_PASSWORD", Value: "password"})
	} else {
		env = append(env, corev1.EnvVar{Name: "MYSQL_ROOT_PASSWORD", Value: mysqlCfg.Password})
	}

	// 如果不是 root，则创建同名用户，保证 Store 连接参数可用
	if u := strings.TrimSpace(mysqlCfg.User); u != "" && u != "root" {
		env = append(env,
			corev1.EnvVar{Name: "MYSQL_USER", Value: u},
			corev1.EnvVar{Name: "MYSQL_PASSWORD", Value: mysqlCfg.Password},
		)
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "storage", Name: "mysql"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "mysql",
					Image: "mysql:8.0",
					Env:   env,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 3306,
							HostPort:      int32(mysqlCfg.Port),
						},
					},
				},
			},
		},
	}
}

// buildEtcdPodFromEndpoint 从一个 etcd endpoint 构造用于拉起 etcd 容器的 Pod 描述。
//
// 端口策略：
// - clientPort: endpoint 端口（默认 2379）
// - peerPort: 默认 2380；若 clientPort 不是 2379，则 peerPort = clientPort + 1
func buildEtcdPodFromEndpoint(endpoint string) (*corev1.Pod, error) {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return nil, err
	}
	host := u.Hostname()
	if host == "" {
		host = "localhost"
	}
	portStr := u.Port()
	if portStr == "" {
		portStr = "2379"
	}
	clientPort, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}
	peerPort := 2380
	if clientPort != 2379 {
		peerPort = clientPort + 1
	}

	args := []string{
		"--name", "etcd",
		"--data-dir", "/etcd-data",
		"--listen-client-urls", fmt.Sprintf("http://0.0.0.0:%d", clientPort),
		"--advertise-client-urls", fmt.Sprintf("http://%s:%d", host, clientPort),
		"--listen-peer-urls", fmt.Sprintf("http://0.0.0.0:%d", peerPort),
		"--initial-advertise-peer-urls", fmt.Sprintf("http://%s:%d", host, peerPort),
		"--initial-cluster", fmt.Sprintf("etcd=http://%s:%d", host, peerPort),
		"--initial-cluster-token", "hermes-etcd-token",
		"--initial-cluster-state", "new",
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "storage", Name: "etcd"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "etcd",
					Image: "quay.io/coreos/etcd:v3.5.0",
					Args:  args,
					Ports: []corev1.ContainerPort{
						{ContainerPort: int32(clientPort), HostPort: int32(clientPort)},
						{ContainerPort: int32(peerPort), HostPort: int32(peerPort)},
					},
				},
			},
		},
	}, nil
}
