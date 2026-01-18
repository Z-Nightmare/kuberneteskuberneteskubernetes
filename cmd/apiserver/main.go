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
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/config"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/webprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/service"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/apiserver"
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
		service.Modules,
		api.Modules,
		apiserver.Module,
	)

	app := fx.New(modules,
		fx.WithLogger(func() fxevent.Logger {
			logger := logprovider.GetLogger()
			return logger.GetFxLogger()
		}),
		fx.Invoke(StartFiberServer),
	)

	if err := app.Start(context.Background()); err != nil {
		fmt.Println("Fx 启动失败: ", err)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			fmt.Println("Fx 停止失败: ", err)
		}
	}()

	GracefulShutdown(func() {
		fmt.Println("清理资源...")
	})
}

// GracefulShutdown 封装优雅停机逻辑
func GracefulShutdown(cleanupFuncs ...func()) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit // 阻塞主线程

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup

	tasks := []func(context.Context){}
	for i, cleanFn := range cleanupFuncs {
		tasks = append(tasks, func(ctx context.Context) {
			defer wg.Done()
			cleanFn()
			select {
			case <-ctx.Done():
				fmt.Printf("清理任务 %d 超时/取消\n", i+1)
			default:
				fmt.Printf("清理任务 %d 完成\n", i+1)
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

func StartFiberServer(
	lc fx.Lifecycle,
	router api.Routes,
	config config.Config,
	fiber webprovider.FiberEngine,
	l logprovider.Logger,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			router.SetUp()
			go func() {
				l.Infof("正在启动Fiber服务器 http://localhost:%v/translate", config.Gin.Port)
				err := fiber.App.Listen(fmt.Sprintf(":%v", config.Gin.Port))
				if err != nil {
					l.Panic("无法启动服务器: ", err.Error())
					return
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			l.Info("正在关闭服务器...")
			return fiber.App.Shutdown()
		},
	})
}
