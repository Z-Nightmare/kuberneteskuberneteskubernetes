package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"zeusro.com/gotemplate/api"
)

// 中间件：日志 + traceID
func LoggerWithTraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		traceID := uuid.New().String()
		c.Set("traceID", traceID)

		// 继续执行
		c.Next()

		// 记录日志
		duration := time.Since(start)
		log.Printf("[TraceID:%s] %s %s %d %s",
			traceID,
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			duration,
		)
	}
}

func main() {
	//prepare work
	// for i := 0; i < 2; i++ {
	// 	// 初始化 ClickHouse
	// 	err := initClickHouse()
	// 	if err != nil {
	// 		time.Sleep(time.Second * 3)
	// 	} else {
	// 		break
	// 	}

	// }

	r := gin.Default()

	// 加载静态文件 index.html
	r.StaticFile("/", "./static/index.html")

	// 使用自定义日志+traceID中间件
	r.Use(LoggerWithTraceID())
	Route(r)

	// 启动服务
	// 定义 HTTP 服务器
	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	// 启动服务（放在 goroutine 里，非阻塞）
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("listen: %s\n", err)
		}
	}()
	fmt.Println("服务已启动，监听 8080 端口")

	GracefulShutdown(srv, func() {
		//清理工作1
	}, func() {
		//清理工作2
	})
	//清理工作完成后会关闭http连接
	fmt.Println("服务已优雅退出")
}

func Route(r *gin.Engine) {

	// 路由 /shit → 委托给 Shit 方法
	// r.POST("/shit", Shit)

	r.GET("/ping", api.Health)

	r.Any("/health", api.Health)
	r.Any("/healthz", api.Health)
	// methods := []string{"GET", "POST", "PUT", "OPTIONS"}
	// for _, m := range methods {
	// 	r.Handle(m, "/health", api.Health)
	// 	r.Handle(m, "/healthz", api.Health)
	// }

}

// GracefulShutdown 封装优雅停机逻辑
func GracefulShutdown(srv *http.Server, cleanupFuncs ...func()) {
	quit := make(chan os.Signal, 1)

	/*	syscall.SIGINT（Ctrl+C）
		syscall.SIGTERM（容器 stop / kill）
		syscall.SIGQUIT（Ctrl+\）
		syscall.SIGHUP（reload 热重启）
	*/
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	sig := <-quit // 阻塞，等待信号
	fmt.Printf("收到信号: %s，开始优雅停机...\n", sig)

	// 执行用户自定义清理函数
	for _, fn := range cleanupFuncs {
		fn()
	}

	// 设定超时时间，5 秒内优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("优雅停机失败: %v\n", err)
	} else {
		fmt.Println("服务已优雅停机")
	}
}
