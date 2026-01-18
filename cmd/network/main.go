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
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/network"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

func main() {
	fs := flag.NewFlagSet("network", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	cfgPath := fs.String("config", "", "配置文件路径（等价于环境变量 CONFIG_PATH）")
	listen := fs.String("listen", ":7946", "health server 监听地址（同时作为 mDNS 广播端口）")
	service := fs.String("service", "_k3._tcp", "mDNS service name")
	domain := fs.String("domain", "local.", "mDNS domain (通常为 local.)")
	nodeName := fs.String("node-name", "", "节点名称（默认 NODE_NAME 或 hostname）")
	peerTTL := fs.Duration("peer-ttl", 90*time.Second, "peer 过期时间（超过则标记 NotReady）")
	registerSelf := fs.Bool("register-self", true, "同时把当前节点也注册到 store（若 controller 已上报该节点，则不会覆盖）")

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	applyConfigFlag(*cfgPath)

	modules := fx.Options(
		core.CoreModule,
		fx.Provide(
			bootstrap.ProvideDBContainerHandle,
			bootstrap.ProvideStore,
			func() network.Settings {
				return network.Settings{
					ListenAddr:   *listen,
					Service:      *service,
					Domain:       *domain,
					PeerTTL:      *peerTTL,
					NodeName:     *nodeName,
					RegisterSelf: *registerSelf,
				}
			},
			network.NewService,
		),
	)

	app := fx.New(modules,
		fx.WithLogger(func() fxevent.Logger {
			logger := logprovider.GetLogger()
			return logger.GetFxLogger()
		}),
		fx.Invoke(StartNetworkService),
	)

	if err := app.Start(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "network 启动失败: %v\n", err)
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
	fmt.Println("network 已优雅停机")
}

func StartNetworkService(lc fx.Lifecycle, svc *network.Service) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return svc.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return svc.Stop(ctx)
		},
	})
}

