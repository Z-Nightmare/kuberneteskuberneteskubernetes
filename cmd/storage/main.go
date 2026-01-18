package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/bootstrap"
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
		fx.Provide(
			bootstrap.ProvideDBContainerHandle,
			bootstrap.ProvideStore,
		),
	)

	app := fx.New(modules,
		fx.WithLogger(func() fxevent.Logger {
			logger := logprovider.GetLogger()
			return logger.GetFxLogger()
		}),
		fx.Invoke(StartStorageService))

	if err := app.Start(context.Background()); err != nil {
		fmt.Printf("存储服务启动失败: %v\n", err)
		os.Exit(1)
	}

	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			fmt.Printf("存储服务停止失败: %v\n", err)
		}
	}()

	GracefulShutdown(func() {
		fmt.Println("清理存储资源...")
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
	fmt.Println("存储服务已优雅停机")
}

// StartStorageService 启动存储服务
func StartStorageService(
	lc fx.Lifecycle,
	store storage.Store,
	handle *bootstrap.DBContainerHandle,
	cfg config.Config,
	l logprovider.Logger,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			storageType := cfg.Storage.Type
			l.Info(fmt.Sprintf("正在启动存储服务 (类型: %s)...", storageType))

			// 根据存储类型输出不同的日志信息
			switch storageType {
			case "mysql":
				l.Info(fmt.Sprintf("MySQL 存储已连接: %s:%d/%s",
					cfg.Storage.MySQL.Host,
					cfg.Storage.MySQL.Port,
					cfg.Storage.MySQL.Database))
			case "etcd":
				l.Info(fmt.Sprintf("Etcd 存储已连接: %v", cfg.Storage.Etcd.Endpoints))
			case "memory":
				l.Info("内存存储已初始化")
			default:
				l.Warn(fmt.Sprintf("未知的存储类型: %s", storageType))
			}

			l.Info("存储服务启动完成")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			l.Info("正在关闭存储服务...")

			// 尝试关闭存储连接
			if closer, ok := store.(interface{ Close() error }); ok {
				if err := closer.Close(); err != nil {
					l.Error(fmt.Sprintf("关闭存储连接失败: %v", err))
					return err
				}
				l.Info("存储连接已关闭")
			}

			// 如果 DB 容器由本进程拉起，则在退出时清理
			if handle != nil && handle.Started && handle.Runtime != nil && handle.Pod != nil {
				l.Infof("清理由本进程拉起的存储后端容器 (runtime=%s)...", handle.Runtime.Name())
				if err := handle.Runtime.StopContainer(context.Background(), handle.Pod); err != nil {
					l.Warnf("清理容器失败: %v", err)
				} else {
					l.Info("存储后端容器已清理")
				}
			}

			l.Info("存储服务已关闭")
			return nil
		},
	})
}
