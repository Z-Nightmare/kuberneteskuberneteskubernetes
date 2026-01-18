package controller

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	corev1 "k8s.io/api/core/v1"
)

// ContainerRuntime 是容器运行时的接口
type ContainerRuntime interface {
	// Name 返回运行时名称
	Name() string
	// IsAvailable 检查运行时是否可用
	IsAvailable() bool
	// StartContainer 启动容器
	StartContainer(ctx context.Context, pod *corev1.Pod) error
	// StopContainer 停止容器
	StopContainer(ctx context.Context, pod *corev1.Pod) error
	// GetContainerStatus 获取容器状态
	GetContainerStatus(ctx context.Context, pod *corev1.Pod) (ContainerStatus, error)
}

// ContainerStatus 容器状态
type ContainerStatus struct {
	Running bool
	Status  string
	Message string
}

// RuntimeDetector 检测可用的容器运行时
type RuntimeDetector struct {
	logger logprovider.Logger
}

// NewRuntimeDetector 创建运行时检测器
func NewRuntimeDetector(logger logprovider.Logger) *RuntimeDetector {
	return &RuntimeDetector{
		logger: logger,
	}
}

// DetectRuntime 检测并返回可用的容器运行时
func (rd *RuntimeDetector) DetectRuntime() (ContainerRuntime, error) {
	// 优先级：Docker > Podman > Containerd > CRI-O
	runtimes := []struct {
		name     string
		detector func() (ContainerRuntime, error)
	}{
		{"Docker", rd.detectDocker},
		{"Podman", rd.detectPodman},
		{"Containerd", rd.detectContainerd},
		{"CRI-O", rd.detectCRIO},
	}

	for _, rt := range runtimes {
		runtime, err := rt.detector()
		if err == nil && runtime != nil && runtime.IsAvailable() {
			rd.logger.Infof("检测到容器运行时: %s", rt.name)
			return runtime, nil
		}
		rd.logger.Debugf("容器运行时 %s 不可用: %v", rt.name, err)
	}

	return nil, fmt.Errorf("未找到可用的容器运行时")
}

// detectDocker 检测 Docker
func (rd *RuntimeDetector) detectDocker() (ContainerRuntime, error) {
	// 检查 docker 命令是否存在
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker 命令未找到")
	}

	// 检查 docker daemon 是否运行
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker daemon 未运行: %w", err)
	}

	return NewDockerRuntime(rd.logger), nil
}

// detectPodman 检测 Podman
func (rd *RuntimeDetector) detectPodman() (ContainerRuntime, error) {
	if _, err := exec.LookPath("podman"); err != nil {
		return nil, fmt.Errorf("podman 命令未找到")
	}

	cmd := exec.Command("podman", "info")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("podman 未运行: %w", err)
	}

	return NewPodmanRuntime(rd.logger), nil
}

// detectContainerd 检测 Containerd
func (rd *RuntimeDetector) detectContainerd() (ContainerRuntime, error) {
	if _, err := exec.LookPath("ctr"); err != nil {
		return nil, fmt.Errorf("ctr 命令未找到")
	}

	return NewContainerdRuntime(rd.logger), nil
}

// detectCRIO 检测 CRI-O
func (rd *RuntimeDetector) detectCRIO() (ContainerRuntime, error) {
	if _, err := exec.LookPath("crictl"); err != nil {
		return nil, fmt.Errorf("crictl 命令未找到")
	}

	return NewCRIORuntime(rd.logger), nil
}

// DockerRuntime Docker 容器运行时实现
type DockerRuntime struct {
	logger logprovider.Logger
}

// NewDockerRuntime 创建 Docker 运行时
func NewDockerRuntime(logger logprovider.Logger) *DockerRuntime {
	return &DockerRuntime{
		logger: logger,
	}
}

// Name 返回运行时名称
func (dr *DockerRuntime) Name() string {
	return "Docker"
}

// IsAvailable 检查 Docker 是否可用
func (dr *DockerRuntime) IsAvailable() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

// StartContainer 启动容器
func (dr *DockerRuntime) StartContainer(ctx context.Context, pod *corev1.Pod) error {
	if len(pod.Spec.Containers) == 0 {
		return fmt.Errorf("Pod %s/%s 没有容器定义", pod.Namespace, pod.Name)
	}

	container := pod.Spec.Containers[0]
	containerName := fmt.Sprintf("k8s_%s_%s_%s", pod.Namespace, pod.Name, container.Name)

	// 构建 docker run 命令
	args := []string{"run", "-d", "--name", containerName}

	// 添加环境变量
	for _, env := range container.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", env.Name, env.Value))
	}

	// 添加端口映射
	for _, port := range container.Ports {
		if port.HostPort != 0 {
			args = append(args, "-p", fmt.Sprintf("%d:%d", port.HostPort, port.ContainerPort))
		} else {
			args = append(args, "-p", fmt.Sprintf("%d", port.ContainerPort))
		}
	}

	// 添加镜像
	image := container.Image
	if image == "" {
		return fmt.Errorf("容器镜像未指定")
	}

	args = append(args, image)

	// 添加命令和参数
	if len(container.Command) > 0 {
		args = append(args, container.Command...)
	}
	if len(container.Args) > 0 {
		args = append(args, container.Args...)
	}

	dr.logger.Infof("启动 Docker 容器: %s, 命令: docker %s", containerName, strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("启动容器失败: %w, 输出: %s", err, string(output))
	}

	dr.logger.Infof("容器启动成功: %s, 容器ID: %s", containerName, strings.TrimSpace(string(output)))
	return nil
}

// StopContainer 停止容器
func (dr *DockerRuntime) StopContainer(ctx context.Context, pod *corev1.Pod) error {
	if len(pod.Spec.Containers) == 0 {
		return fmt.Errorf("Pod %s/%s 没有容器定义", pod.Namespace, pod.Name)
	}

	container := pod.Spec.Containers[0]
	containerName := fmt.Sprintf("k8s_%s_%s_%s", pod.Namespace, pod.Name, container.Name)

	dr.logger.Infof("停止 Docker 容器: %s", containerName)

	// 先停止容器
	cmd := exec.CommandContext(ctx, "docker", "stop", containerName)
	if err := cmd.Run(); err != nil {
		dr.logger.Warnf("停止容器失败（可能已停止）: %v", err)
	}

	// 删除容器
	cmd = exec.CommandContext(ctx, "docker", "rm", containerName)
	if err := cmd.Run(); err != nil {
		dr.logger.Warnf("删除容器失败（可能已删除）: %v", err)
	}

	return nil
}

// GetContainerStatus 获取容器状态
func (dr *DockerRuntime) GetContainerStatus(ctx context.Context, pod *corev1.Pod) (ContainerStatus, error) {
	if len(pod.Spec.Containers) == 0 {
		return ContainerStatus{}, fmt.Errorf("Pod %s/%s 没有容器定义", pod.Namespace, pod.Name)
	}

	container := pod.Spec.Containers[0]
	containerName := fmt.Sprintf("k8s_%s_%s_%s", pod.Namespace, pod.Name, container.Name)

	// 检查容器是否存在
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", fmt.Sprintf("name=^%s$", containerName), "--format", "{{.Status}}")
	output, err := cmd.Output()
	if err != nil {
		return ContainerStatus{Running: false, Status: "Unknown"}, nil
	}

	statusStr := strings.TrimSpace(string(output))
	running := strings.Contains(statusStr, "Up")

	return ContainerStatus{
		Running: running,
		Status:  statusStr,
		Message: statusStr,
	}, nil
}

// PodmanRuntime Podman 容器运行时实现（占位符）
type PodmanRuntime struct {
	logger logprovider.Logger
}

// NewPodmanRuntime 创建 Podman 运行时
func NewPodmanRuntime(logger logprovider.Logger) *PodmanRuntime {
	return &PodmanRuntime{logger: logger}
}

func (pr *PodmanRuntime) Name() string {
	return "Podman"
}

func (pr *PodmanRuntime) IsAvailable() bool {
	cmd := exec.Command("podman", "info")
	return cmd.Run() == nil
}

func (pr *PodmanRuntime) StartContainer(ctx context.Context, pod *corev1.Pod) error {
	return fmt.Errorf("Podman 运行时尚未实现")
}

func (pr *PodmanRuntime) StopContainer(ctx context.Context, pod *corev1.Pod) error {
	return fmt.Errorf("Podman 运行时尚未实现")
}

func (pr *PodmanRuntime) GetContainerStatus(ctx context.Context, pod *corev1.Pod) (ContainerStatus, error) {
	return ContainerStatus{}, fmt.Errorf("Podman 运行时尚未实现")
}

// ContainerdRuntime Containerd 容器运行时实现（占位符）
type ContainerdRuntime struct {
	logger logprovider.Logger
}

// NewContainerdRuntime 创建 Containerd 运行时
func NewContainerdRuntime(logger logprovider.Logger) *ContainerdRuntime {
	return &ContainerdRuntime{logger: logger}
}

func (cr *ContainerdRuntime) Name() string {
	return "Containerd"
}

func (cr *ContainerdRuntime) IsAvailable() bool {
	return true
}

func (cr *ContainerdRuntime) StartContainer(ctx context.Context, pod *corev1.Pod) error {
	return fmt.Errorf("Containerd 运行时尚未实现")
}

func (cr *ContainerdRuntime) StopContainer(ctx context.Context, pod *corev1.Pod) error {
	return fmt.Errorf("Containerd 运行时尚未实现")
}

func (cr *ContainerdRuntime) GetContainerStatus(ctx context.Context, pod *corev1.Pod) (ContainerStatus, error) {
	return ContainerStatus{}, fmt.Errorf("Containerd 运行时尚未实现")
}

// CRIORuntime CRI-O 容器运行时实现（占位符）
type CRIORuntime struct {
	logger logprovider.Logger
}

// NewCRIORuntime 创建 CRI-O 运行时
func NewCRIORuntime(logger logprovider.Logger) *CRIORuntime {
	return &CRIORuntime{logger: logger}
}

func (crio *CRIORuntime) Name() string {
	return "CRI-O"
}

func (crio *CRIORuntime) IsAvailable() bool {
	return true
}

func (crio *CRIORuntime) StartContainer(ctx context.Context, pod *corev1.Pod) error {
	return fmt.Errorf("CRI-O 运行时尚未实现")
}

func (crio *CRIORuntime) StopContainer(ctx context.Context, pod *corev1.Pod) error {
	return fmt.Errorf("CRI-O 运行时尚未实现")
}

func (crio *CRIORuntime) GetContainerStatus(ctx context.Context, pod *corev1.Pod) (ContainerStatus, error) {
	return ContainerStatus{}, fmt.Errorf("CRI-O 运行时尚未实现")
}
