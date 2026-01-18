package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
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
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
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
		fx.Invoke(StartK3),
	)

	if err := app.Start(context.Background()); err != nil {
		fmt.Printf("K3 启动失败: %v\n", err)
		os.Exit(1)
	}

	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			fmt.Printf("K3 停止失败: %v\n", err)
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

// StartK3 按顺序启动：storage -> controller -> web。
//
// - storage：通过 bootstrap provider 确保（必要时）先拉起 DB 容器并初始化 Store
// - controller：启动控制器管理器与节点心跳
// - web：启动 Fiber Web 服务并注册路由
func StartK3(
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
			// 1) storage：store 已经在依赖注入阶段创建成功（否则 fx 启动会失败）
			l.Infof("Storage 已就绪 (type=%s)", cfg.Storage.Type)

			// 2) controller
			l.Info("正在启动 Controller...")
			if err := cm.Start(ctx); err != nil {
				return err
			}
			go cm.StartNodeHeartbeat(ctx)
			l.Info("Controller 启动完成")

			// 3) web
			l.Info("正在启动 Web...")
			router.SetUp()
			go func() {
				l.Infof("正在启动Fiber服务器 http://localhost:%v/translate", cfg.Gin.Port)
				if err := fiber.App.Listen(fmt.Sprintf(":%v", cfg.Gin.Port)); err != nil {
					l.Panic("无法启动服务器: ", err.Error())
				}
			}()
			l.Info("Web 启动完成")

			_ = store // 显式引用，避免未来调整时被误删依赖
			return nil
		},
		OnStop: func(ctx context.Context) error {
			l.Info("正在关闭 K3...")

			// 关闭 web
			_ = fiber.App.Shutdown()

			// 关闭 controller
			_ = cm.Stop(ctx)

			// 关闭 storage
			if closer, ok := store.(interface{ Close() error }); ok {
				_ = closer.Close()
			}

			// 清理由本进程拉起的 DB 容器
			if handle != nil && handle.Started && handle.Runtime != nil && handle.Pod != nil {
				_ = handle.Runtime.StopContainer(context.Background(), handle.Pod)
			}

			l.Info("K3 已关闭")
			return nil
		},
	})
}
