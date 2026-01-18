package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/controller"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/config"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

func main() {
	modules := fx.Options(
		core.CoreModule,
		controller.Module,
		fx.Provide(
			storage.NewStore,
		),
	)

	app := fx.New(modules,
		fx.WithLogger(func() fxevent.Logger {
			logger := logprovider.GetLogger()
			return logger.GetFxLogger()
		}),
		fx.Invoke(StartControllerManager))

	if err := app.Start(context.Background()); err != nil {
		fmt.Println("Fx 启动失败: ", err)
		os.Exit(1)
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

func StartControllerManager(
	lc fx.Lifecycle,
	cm *controller.ControllerManager,
	config config.Config,
	l logprovider.Logger,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			l.Info("正在启动控制器管理器...")

			// 启动控制器管理器
			if err := cm.Start(ctx); err != nil {
				return err
			}

			// 启动节点心跳上报
			go cm.StartNodeHeartbeat(ctx)

			l.Info("控制器管理器启动完成")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			l.Info("正在关闭控制器管理器...")
			return cm.Stop(ctx)
		},
	})
}
