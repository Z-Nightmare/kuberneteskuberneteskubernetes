package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const TraceIDKey = "traceid"

// TraceIDMiddleware 定义中间件，用于生成和注入 traceID
func TraceIDMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 生成唯一的 traceID
		traceID := uuid.New().String()

		// 将 traceID 添加到请求上下文中
		c.Set("traceID", traceID)

		// 将 traceID 添加到日志上下文
		logger = logger.With(zap.String(TraceIDKey, traceID))

		// 记录收到请求的日志
		startTime := time.Now()
		logger.Info("Incoming request",
			zap.String("method", c.Request.Method),
			zap.String("url", c.Request.URL.Path),
		)

		// 处理请求
		c.Next()

		// 请求结束后记录响应信息
		latency := time.Since(startTime)
		statusCode := c.Writer.Status()
		logger.Info("Completed request",
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
		)
	}
}
