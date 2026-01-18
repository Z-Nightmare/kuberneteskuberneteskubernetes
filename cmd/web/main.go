package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/api"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/bootstrap"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/controller"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/config"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/webprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/service"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/apiserver"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

func main() {
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

	app := fx.New(modules,
		fx.WithLogger(func() fxevent.Logger {
			logger := logprovider.GetLogger()
			return logger.GetFxLogger()
		}),
		fx.Invoke(StartWebStack),
	)

	if err := app.Start(context.Background()); err != nil {
		fmt.Println("Fx 启动失败: ", err)
		os.Exit(1)
	}
	defer func() { _ = app.Stop(context.Background()) }()

	GracefulShutdown(func() {
		fmt.Println("清理资源...")
	})
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
	fmt.Println("服务已优雅停机")
}

func StartWebStack(
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

			// 启动控制器（会写入 Node/调度 Pod 等，方便 Dashboard 展示“当前资源”）
			l.Info("正在启动 Controller...")
			if err := cm.Start(ctx); err != nil {
				return err
			}
			go cm.StartNodeHeartbeat(ctx)
			l.Info("Controller 启动完成")

			// 启动 Web（包括 Dashboard + API server）
			router.SetUp()
			go func() {
				l.Infof("正在启动 Web http://localhost:%v/ (dashboard)", cfg.Gin.Port)
				if err := fiber.App.Listen(fmt.Sprintf(":%v", cfg.Gin.Port)); err != nil {
					l.Panic("无法启动服务器: ", err.Error())
				}
			}()

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
