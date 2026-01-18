package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/api"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/bootstrap"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/controller"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/config"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/webprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/service"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/apiserver"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/parser"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "start":
		os.Exit(cmdStartAll(os.Args[2:]))
	case "storage":
		os.Exit(cmdStorage(os.Args[2:]))
	case "controller":
		os.Exit(cmdController(os.Args[2:]))
	case "web":
		os.Exit(cmdWeb(os.Args[2:]))
	case "apply":
		os.Exit(cmdApply(os.Args[2:]))
	case "cluster":
		os.Exit(cmdCluster(os.Args[2:]))
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(strings.TrimSpace(`
Usage:
  k3 <command> [flags]

Commands:
  start                 按顺序启动 storage -> controller -> web（单进程）
  storage               仅启动 storage（包含按需拉起 mysql/etcd 容器）
  controller            启动 storage + controller
  web                   启动 storage + web
  apply                 将 Kubernetes YAML/JSON 提交到 apiserver（最小 apply 子集）
  cluster create        创建 k3 集群配置骨架（多节点配置文件）
  cluster clear         删除 k3 集群配置目录以及关联的容器

Flags:
  --config <path>       指定配置文件路径（默认: ./.config.yaml；也支持环境变量 CONFIG_PATH）
`))
}

func commonFlags(fs *flag.FlagSet) *string {
	return fs.String("config", "", "配置文件路径（等价于环境变量 CONFIG_PATH）")
}

func applyConfigFlag(configPath string) {
	if strings.TrimSpace(configPath) == "" {
		return
	}
	_ = os.Setenv("CONFIG_PATH", configPath)
}

func newFxLogger() fxevent.Logger {
	logger := logprovider.GetLogger()
	return logger.GetFxLogger()
}

func fxApp(modules fx.Option, invoke any) *fx.App {
	opts := fx.Options(
		modules,
	)
	return fx.New(
		opts,
		fx.WithLogger(func() fxevent.Logger { return newFxLogger() }),
		fx.Invoke(invoke),
	)
}

// cmdStartAll 启动 storage -> controller -> web（单进程）
func cmdStartAll(args []string) int {
	fs := flag.NewFlagSet("k3 start", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cfgPath := commonFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	applyConfigFlag(*cfgPath)

	modules := fx.Options(
		core.CoreModule,
		fx.Provide(
			bootstrap.ProvideDBContainerHandle,
			bootstrap.ProvideStore,
		),
		controller.Module,
		service.Modules,
		api.Modules,
		apiserver.Module,
	)

	app := fxApp(modules, StartAll)
	if err := app.Start(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		return 1
	}
	defer func() { _ = app.Stop(context.Background()) }()

	blockUntilSignal()
	return 0
}

// cmdStorage 仅启动 storage（保持 DB 连接/容器生命周期）
func cmdStorage(args []string) int {
	fs := flag.NewFlagSet("k3 storage", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cfgPath := commonFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	applyConfigFlag(*cfgPath)

	modules := fx.Options(
		core.CoreModule,
		fx.Provide(
			bootstrap.ProvideDBContainerHandle,
			bootstrap.ProvideStore,
		),
	)

	app := fxApp(modules, StartStorageOnly)
	if err := app.Start(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		return 1
	}
	defer func() { _ = app.Stop(context.Background()) }()

	blockUntilSignal()
	return 0
}

// cmdController 启动 storage + controller
func cmdController(args []string) int {
	fs := flag.NewFlagSet("k3 controller", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cfgPath := commonFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	applyConfigFlag(*cfgPath)

	modules := fx.Options(
		core.CoreModule,
		fx.Provide(
			bootstrap.ProvideDBContainerHandle,
			bootstrap.ProvideStore,
		),
		controller.Module,
	)

	app := fxApp(modules, StartControllerOnly)
	if err := app.Start(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		return 1
	}
	defer func() { _ = app.Stop(context.Background()) }()

	blockUntilSignal()
	return 0
}

// cmdWeb 启动 storage + web
func cmdWeb(args []string) int {
	fs := flag.NewFlagSet("k3 web", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cfgPath := commonFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	applyConfigFlag(*cfgPath)

	modules := fx.Options(
		core.CoreModule,
		fx.Provide(
			bootstrap.ProvideDBContainerHandle,
			bootstrap.ProvideStore,
		),
		service.Modules,
		api.Modules,
		apiserver.Module,
	)

	app := fxApp(modules, StartWebOnly)
	if err := app.Start(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		return 1
	}
	defer func() { _ = app.Stop(context.Background()) }()

	blockUntilSignal()
	return 0
}

func cmdApply(args []string) int {
	fs := flag.NewFlagSet("k3 apply", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cfgPath := commonFlags(fs)
	file := fs.String("f", "", "要提交的 YAML 文件路径（支持多文档 ---）")
	server := fs.String("server", "", "apiserver 地址（默认从配置读取，例如 http://localhost:8080）")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	applyConfigFlag(*cfgPath)

	if strings.TrimSpace(*file) == "" {
		fmt.Fprintln(os.Stderr, "缺少 -f <file>")
		return 2
	}

	cfg := config.NewFileConfig()
	base := strings.TrimSpace(*server)
	if base == "" {
		base = fmt.Sprintf("http://localhost:%d", cfg.Gin.Port)
	}
	base = strings.TrimRight(base, "/")

	p := parser.NewParser()
	objects, gvks, err := p.ParseYAMLFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析 YAML 失败: %v\n", err)
		return 1
	}
	if len(objects) == 0 {
		fmt.Fprintln(os.Stderr, "YAML 中没有可提交的资源")
		return 2
	}

	client := &http.Client{Timeout: 15 * time.Second}
	for i, obj := range objects {
		gvk := gvks[i]
		if gvk == nil {
			fmt.Fprintf(os.Stderr, "跳过第 %d 个对象：无法解析 GVK\n", i+1)
			continue
		}
		meta, ok := obj.(metav1.Object)
		if !ok {
			fmt.Fprintf(os.Stderr, "跳过第 %d 个对象：不支持的对象类型（无 metadata）\n", i+1)
			continue
		}

		path, err := apiPathFor(*gvk, meta.GetNamespace())
		if err != nil {
			fmt.Fprintf(os.Stderr, "跳过 %s/%s：%v\n", gvk.Kind, meta.GetName(), err)
			continue
		}

		body, err := parser.ToYAML(obj)
		if err != nil {
			fmt.Fprintf(os.Stderr, "序列化 %s/%s 失败: %v\n", gvk.Kind, meta.GetName(), err)
			continue
		}

		req, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(string(body)))
		if err != nil {
			fmt.Fprintf(os.Stderr, "构造请求失败: %v\n", err)
			return 1
		}
		req.Header.Set("Content-Type", "application/yaml")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "提交失败 %s/%s: %v\n", gvk.Kind, meta.GetName(), err)
			return 1
		}
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		// 如果已存在，则走一次 PUT（最小化“apply”语义）
		if resp.StatusCode == http.StatusConflict {
			updateURL := fmt.Sprintf("%s%s/%s", base, path, meta.GetName())
			req2, err := http.NewRequest(http.MethodPut, updateURL, strings.NewReader(string(body)))
			if err != nil {
				fmt.Fprintf(os.Stderr, "构造更新请求失败: %v\n", err)
				return 1
			}
			req2.Header.Set("Content-Type", "application/yaml")

			resp2, err := client.Do(req2)
			if err != nil {
				fmt.Fprintf(os.Stderr, "更新失败 %s/%s: %v\n", gvk.Kind, meta.GetName(), err)
				return 1
			}
			respBody2, _ := io.ReadAll(resp2.Body)
			_ = resp2.Body.Close()
			if resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
				fmt.Fprintf(os.Stderr, "更新失败 %s/%s: HTTP %d: %s\n", gvk.Kind, meta.GetName(), resp2.StatusCode, strings.TrimSpace(string(respBody2)))
				return 1
			}
			fmt.Printf("已更新 %s %s/%s\n", gvk.Kind, meta.GetNamespace(), meta.GetName())
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			fmt.Fprintf(os.Stderr, "提交失败 %s/%s: HTTP %d: %s\n", gvk.Kind, meta.GetName(), resp.StatusCode, strings.TrimSpace(string(respBody)))
			return 1
		}

		fmt.Printf("已提交 %s %s/%s\n", gvk.Kind, meta.GetNamespace(), meta.GetName())
	}

	return 0
}

func apiPathFor(gvk schema.GroupVersionKind, namespace string) (string, error) {
	plural, ok := kindToPlural(gvk.Kind)
	if !ok {
		return "", fmt.Errorf("unsupported kind: %s", gvk.Kind)
	}

	// cluster-scoped
	if gvk.Kind == "Node" && gvk.Group == "" && gvk.Version == "v1" {
		return fmt.Sprintf("/api/%s/%s", gvk.Version, plural), nil
	}

	// namespaced (default to "default")
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		ns = "default"
	}

	if gvk.Group == "" {
		return fmt.Sprintf("/api/%s/namespaces/%s/%s", gvk.Version, ns, plural), nil
	}
	return fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s", gvk.Group, gvk.Version, ns, plural), nil
}

func kindToPlural(kind string) (string, bool) {
	switch kind {
	case "Pod":
		return "pods", true
	case "Service":
		return "services", true
	case "ConfigMap":
		return "configmaps", true
	case "Secret":
		return "secrets", true
	case "Node":
		return "nodes", true
	case "Deployment":
		return "deployments", true
	case "StatefulSet":
		return "statefulsets", true
	case "DaemonSet":
		return "daemonsets", true
	default:
		return "", false
	}
}

func cmdCluster(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "cluster 需要子命令，例如: cluster create")
		return 2
	}

	switch args[0] {
	case "create":
		return cmdClusterCreate(args[1:])
	case "clear":
		return cmdClusterClear(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "未知 cluster 子命令: %s\n", args[0])
		return 2
	}
}

// cmdClusterCreate 创建一个“多节点配置文件”骨架，便于用命令方式管理/启动多个实例。
func cmdClusterCreate(args []string) int {
	fs := flag.NewFlagSet("k3 cluster create", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("dir", ".k3", "输出目录")
	nodes := fs.Int("nodes", 1, "节点数量")
	webPort := fs.Int("web-port", 8080, "node-1 的 web 端口")
	storageType := fs.String("storage", "memory", "storage 类型：memory/mysql/etcd")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *nodes <= 0 {
		fmt.Fprintln(os.Stderr, "--nodes 必须 > 0")
		return 2
	}

	for i := 1; i <= *nodes; i++ {
		nodeDir := filepath.Join(*dir, fmt.Sprintf("node-%d", i))
		if err := os.MkdirAll(nodeDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "创建目录失败: %v\n", err)
			return 1
		}

		cfgPath := filepath.Join(nodeDir, ".config.yaml")
		cfgContent := defaultConfigYAML(*webPort+i-1, *storageType)
		if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "写入配置失败: %v\n", err)
			return 1
		}
	}

	fmt.Printf("已生成 %d 个节点配置到 %s/\n", *nodes, *dir)
	fmt.Println("示例：启动 node-1：")
	fmt.Printf("  CONFIG_PATH=%s go run ./cmd/k3 start\n", filepath.Join(*dir, "node-1", ".config.yaml"))
	return 0
}

// cmdClusterClear 删除 k3 集群配置目录以及关联的容器
func cmdClusterClear(args []string) int {
	fs := flag.NewFlagSet("k3 cluster clear", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("dir", ".k3", "要删除的集群配置目录")
	force := fs.Bool("force", false, "不询问确认，直接删除")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	clusterDir := strings.TrimSpace(*dir)
	if clusterDir == "" {
		fmt.Fprintln(os.Stderr, "--dir 不能为空")
		return 2
	}

	// 安全检查：避免误删重要目录
	if clusterDir == "." || clusterDir == "/" || clusterDir == ".." || strings.Contains(clusterDir, "..") {
		fmt.Fprintf(os.Stderr, "错误：不允许删除目录 %s（安全限制）\n", clusterDir)
		return 1
	}

	// 确认操作
	if !*force {
		fmt.Fprintf(os.Stderr, "警告：将删除目录 %s 及其所有内容，以及关联的 k3 容器\n", clusterDir)
		fmt.Fprint(os.Stderr, "确认删除？(yes/no): ")
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(strings.TrimSpace(answer)) != "yes" {
			fmt.Println("已取消")
			return 0
		}
	}

	// 1. 停止并删除关联容器
	fmt.Println("正在清理关联容器...")
	containersCleared := clearK3Containers()

	// 2. 删除配置目录
	fmt.Printf("正在删除配置目录: %s\n", clusterDir)
	if err := os.RemoveAll(clusterDir); err != nil {
		fmt.Fprintf(os.Stderr, "删除目录失败: %v\n", err)
		return 1
	}

	fmt.Printf("✅ 清理完成：已删除 %d 个容器，已删除目录 %s\n", containersCleared, clusterDir)
	return 0
}

// clearK3Containers 清理 k3 相关的 Docker 容器
// 返回清理的容器数量
func clearK3Containers() int {
	// 检查 docker 是否可用
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Println("未找到 docker 命令，跳过容器清理")
		return 0
	}

	// 检查 docker daemon 是否运行
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		fmt.Println("Docker daemon 未运行，跳过容器清理")
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 获取所有容器名称（包括已停止的）
	listCmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{.Names}}")
	output, err := listCmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取容器列表失败: %v\n", err)
		return 0
	}

	allNames := strings.Split(strings.TrimSpace(string(output)), "\n")
	cleared := 0

	// k3 相关的容器名称模式
	// 1. 存储容器：k8s_storage_mysql_mysql, k8s_storage_etcd_etcd
	// 2. Pod 容器：k8s_{namespace}_{pod-name}_{container-name}
	matchedContainers := make(map[string]bool)

	for _, name := range allNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// 匹配 k3 容器命名规则
		if strings.HasPrefix(name, "k8s_") {
			matchedContainers[name] = true
		}
	}

	// 停止并删除匹配的容器
	for name := range matchedContainers {
		// 停止容器
		stopCmd := exec.CommandContext(ctx, "docker", "stop", name)
		if err := stopCmd.Run(); err != nil {
			// 容器可能已经停止，忽略错误
		}

		// 删除容器
		rmCmd := exec.CommandContext(ctx, "docker", "rm", name)
		if err := rmCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  删除容器 %s 失败: %v\n", name, err)
		} else {
			fmt.Printf("  已删除容器: %s\n", name)
			cleared++
		}
	}

	return cleared
}

func defaultConfigYAML(port int, storageType string) string {
	// 基于项目现有配置结构生成一个最小可运行配置；用户可按需修改。
	return fmt.Sprintf(strings.TrimSpace(`
debug: true
web:
  port: %d
  cors: true
log:
  level: debug
  path: logs/app.log
storage:
  type: %s
  mysql:
    host: localhost
    port: 3306
    user: root
    password: password
    database: kubernetes
    max_open_conns: 100
    max_idle_conns: 10
  etcd:
    endpoints:
      - http://localhost:2379
    dial_timeout: 5s
    username: ""
    password: ""
jwt:
  signing_key: secret
minimum_deviation_distance: 666
output: console
cities: []
`)+"\n", port, storageType)
}

func blockUntilSignal() {
	ch := make(chan os.Signal, 1)
	signalNotify(ch)
	<-ch
}

func signalNotify(ch chan<- os.Signal) {
	// 使用单独函数，便于后续扩展（如 Windows 信号差异）
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
}

// StartAll 按顺序启动：storage -> controller -> web。
func StartAll(
	lc fx.Lifecycle,
	handle *bootstrap.DBContainerHandle,
	store storage.Store,
	cm *controller.ControllerManager,
	router api.Routes,
	cfg config.Config,
	fiber webprovider.FiberEngine,
	l logprovider.Logger,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			l.Infof("Storage 已就绪 (type=%s)", cfg.Storage.Type)

			l.Info("正在启动 Controller...")
			if err := cm.Start(ctx); err != nil {
				return err
			}
			go cm.StartNodeHeartbeat(ctx)
			l.Info("Controller 启动完成")

			l.Info("正在启动 Web...")
			router.SetUp()
			go func() {
				l.Infof("正在启动Fiber服务器 http://localhost:%v/translate", cfg.Gin.Port)
				if err := fiber.App.Listen(fmt.Sprintf(":%v", cfg.Gin.Port)); err != nil {
					l.Panic("无法启动服务器: ", err.Error())
				}
			}()
			l.Info("Web 启动完成")

			_ = store
			_ = handle
			return nil
		},
		OnStop: func(ctx context.Context) error {
			_ = fiber.App.Shutdown()
			_ = cm.Stop(ctx)
			if closer, ok := store.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
			if handle != nil && handle.Started && handle.Runtime != nil && handle.Pod != nil {
				_ = handle.Runtime.StopContainer(context.Background(), handle.Pod)
			}
			return nil
		},
	})
}

// StartStorageOnly 仅启动 storage（保持 store/容器生命周期）。
func StartStorageOnly(
	lc fx.Lifecycle,
	handle *bootstrap.DBContainerHandle,
	store storage.Store,
	cfg config.Config,
	l logprovider.Logger,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			l.Infof("Storage 已就绪 (type=%s)", cfg.Storage.Type)
			_ = store
			_ = handle
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if closer, ok := store.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
			if handle != nil && handle.Started && handle.Runtime != nil && handle.Pod != nil {
				_ = handle.Runtime.StopContainer(context.Background(), handle.Pod)
			}
			return nil
		},
	})
}

// StartControllerOnly 启动 controller（依赖 storage）。
func StartControllerOnly(
	lc fx.Lifecycle,
	cm *controller.ControllerManager,
	cfg config.Config,
	l logprovider.Logger,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			l.Info("正在启动 Controller...")
			if err := cm.Start(ctx); err != nil {
				return err
			}
			go cm.StartNodeHeartbeat(ctx)
			l.Info("Controller 启动完成")
			_ = cfg
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return cm.Stop(ctx)
		},
	})
}

// StartWebOnly 启动 web（依赖 storage）。
func StartWebOnly(
	lc fx.Lifecycle,
	router api.Routes,
	cfg config.Config,
	fiber webprovider.FiberEngine,
	l logprovider.Logger,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			router.SetUp()
			go func() {
				l.Infof("正在启动Fiber服务器 http://localhost:%v/translate", cfg.Gin.Port)
				if err := fiber.App.Listen(fmt.Sprintf(":%v", cfg.Gin.Port)); err != nil {
					l.Panic("无法启动服务器: ", err.Error())
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = shutdownCtx
			return fiber.App.Shutdown()
		},
	})
}
