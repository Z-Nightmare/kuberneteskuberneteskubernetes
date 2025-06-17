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
	"zeusro.com/hermes/api"
	"zeusro.com/hermes/internal/core"
	"zeusro.com/hermes/internal/core/config"
	"zeusro.com/hermes/internal/core/logprovider"
	"zeusro.com/hermes/internal/core/webprovider"
	"zeusro.com/hermes/internal/service"
)

func main() {
	modules := fx.Options(
		core.CoreModule,
		// model.Module,
		// middleware.Module,
		// repository.Module,
		service.Modules,
		api.Modules)
	app := fx.New(modules,
		fx.WithLogger(func() fxevent.Logger {
			logger := logprovider.GetLogger()
			return logger.GetFxLogger()
		}),
		fx.Invoke(StartGinServer))
	// app.Run()
	if err := app.Start(context.Background()); err != nil {
		// logger.Panic("Fx 启动失败: ", err)
		fmt.Println("Fx 启动失败: ", err)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			// logger.Error("Fx 停止失败: ", err)
			fmt.Println("Fx 停止失败: ", err)
		}
	}()

	GracefulShutdown(func() {
		fmt.Println("清理资源...")
		// 这里可以添加清理逻辑，比如关闭数据库连接、释放资源等
	})

}

// GracefulShutdown 封装优雅停机逻辑
func GracefulShutdown(cleanupFuncs ...func()) {
	quit := make(chan os.Signal, 1)
	/*	syscall.SIGINT（Ctrl+C）
		syscall.SIGTERM（容器 stop / kill）
		syscall.SIGQUIT（Ctrl+\）
		syscall.SIGHUP（reload 热重启）
	*/
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit // 阻塞主线程

	// logger.Infof("收到中断信号%v，开始优雅关闭... ", sig)

	// 开始优雅退出流程（如清理资源）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup

	// 模拟多个任务，每个任务都感知 context
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

	// 启动所有任务
	wg.Add(len(tasks))
	for _, task := range tasks {
		go task(ctx)
	}
	// 等待所有任务完成或超时
	wg.Wait()
	fmt.Println("服务已优雅停机")
}

func StartGinServer(
	lc fx.Lifecycle,
	router api.Routes,
	config config.Config,
	gin webprovider.MyGinEngine,
	l logprovider.Logger,
	// model model.Models,
	// middlewares middleware.Middlewares
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// model.AutoMigrate()
			// middlewares.SetUp()
			router.SetUp()
			go func() {
				l.Infof("正在启动gin服务器 http://localhost:%v/translate", config.Gin.Port)
				err := gin.Gin.Run(fmt.Sprintf(":%v", config.Gin.Port))
				if err != nil {
					l.Panic("无法启动服务器: ", err.Error())
					return
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			l.Info("正在关闭服务器...")
			return nil
		},
	})
}
