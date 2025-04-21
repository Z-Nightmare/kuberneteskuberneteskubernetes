package main

import (
	"log"
	"time"
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
	for i := 0; i < 2; i++ {
		// 初始化 ClickHouse
		err := initClickHouse()
		if err != nil {
			time.Sleep(time.Second * 3)
		} else {
			break
		}

	}

	r := gin.Default()

	// 加载静态文件 index.html
	r.StaticFile("/", "./static/index.html")

	// 使用自定义日志+traceID中间件
	r.Use(LoggerWithTraceID())

	// 路由 /shit → 委托给 Shit 方法
	// r.POST("/shit", Shit)

	// 启动服务
	r.Run(":8080") // 监听 8080 端口
}
