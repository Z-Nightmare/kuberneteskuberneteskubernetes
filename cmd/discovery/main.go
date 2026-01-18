package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/bootstrap"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/discovery"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

func main() {
	fs := flag.NewFlagSet("discovery", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	cfgPath := fs.String("config", "", "配置文件路径（等价于环境变量 CONFIG_PATH）")
	consulAddr := fs.String("consul-addr", "localhost:8500", "Consul 服务器地址")
	consulToken := fs.String("consul-token", "", "Consul ACL token（可选）")
	serviceName := fs.String("service-name", "k3-node", "服务名称")
	serviceID := fs.String("service-id", "", "服务 ID（默认：service-name-node-name）")
	servicePort := fs.Int("service-port", 7946, "服务端口")
	nodeName := fs.String("node-name", "", "节点名称（默认 NODE_NAME 或 hostname）")
	registerSelf := fs.Bool("register-self", true, "同时把当前节点也注册到 store")
	watchInterval := fs.Duration("watch-interval", 15*time.Second, "从 Consul 发现服务并同步到 store 的间隔")
	healthCheckInterval := fs.Duration("health-check-interval", 10*time.Second, "健康检查间隔")
	healthCheckTimeout := fs.Duration("health-check-timeout", 3*time.Second, "健康检查超时")
	deregisterAfter := fs.Duration("deregister-after", 30*time.Second, "服务不健康后多久注销")

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	applyConfigFlag(*cfgPath)

	modules := fx.Options(
		core.CoreModule,
		fx.Provide(
			bootstrap.ProvideDBContainerHandle,
			bootstrap.ProvideStore,
			func() discovery.Settings {
				return discovery.Settings{
					ConsulAddress:                      *consulAddr,
					ConsulToken:                        *consulToken,
					ServiceName:                        *serviceName,
					ServiceID:                          *serviceID,
					ServicePort:                        *servicePort,
					NodeName:                           *nodeName,
					RegisterSelf:                       *registerSelf,
					WatchInterval:                      *watchInterval,
					HealthCheckInterval:                *healthCheckInterval,
					HealthCheckTimeout:                 *healthCheckTimeout,
					DeregisterCriticalServiceAfter:     *deregisterAfter,
				}
			},
			discovery.NewService,
		),
	)

	app := fx.New(modules,
		fx.WithLogger(func() fxevent.Logger {
			logger := logprovider.GetLogger()
			return logger.GetFxLogger()
		}),
		fx.Invoke(StartDiscoveryService),
	)

	if err := app.Start(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "discovery 启动失败: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = app.Stop(context.Background()) }()

	GracefulShutdown()
}

func applyConfigFlag(configPath string) {
	if strings.TrimSpace(configPath) == "" {
		return
	}
	_ = os.Setenv("CONFIG_PATH", configPath)
}

// GracefulShutdown 封装优雅停机逻辑
func GracefulShutdown(cleanupFuncs ...func()) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup

	tasks := []func(context.Context){}
	for i, cleanFn := range cleanupFuncs {
		idx := i + 1
		fn := cleanFn
		tasks = append(tasks, func(ctx context.Context) {
			defer wg.Done()
			fn()
			select {
			case <-ctx.Done():
				fmt.Printf("清理任务 %d 超时/取消\n", idx)
			default:
				fmt.Printf("清理任务 %d 完成\n", idx)
			}
		})
	}

	wg.Add(len(tasks))
	for _, task := range tasks {
		go task(ctx)
	}
	wg.Wait()
	fmt.Println("discovery 已优雅停机")
}

func StartDiscoveryService(lc fx.Lifecycle, svc *discovery.Service) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return svc.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return svc.Stop(ctx)
		},
	})
}
