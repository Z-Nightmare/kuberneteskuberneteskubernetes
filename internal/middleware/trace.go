package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const TraceIDKey = "traceid"

// TraceIDMiddleware 定义中间件，用于生成和注入 traceID (Gin版本，保留用于兼容)
func TraceIDMiddleware(logger *zap.Logger) func(*fiber.Ctx) error {
	return TraceIDMiddlewareFiber(logger)
}

// TraceIDMiddlewareFiber 定义中间件，用于生成和注入 traceID (Fiber版本)
func TraceIDMiddlewareFiber(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 生成唯一的 traceID
		traceID := uuid.New().String()

		// 将 traceID 添加到请求上下文中
		c.Locals("traceID", traceID)

		// 将 traceID 添加到日志上下文
		loggerWithTrace := logger.With(zap.String(TraceIDKey, traceID))

		// 记录收到请求的日志
		startTime := time.Now()
		loggerWithTrace.Info("Incoming request",
			zap.String("method", c.Method()),
			zap.String("url", c.Path()),
		)

		// 处理请求
		err := c.Next()

		// 请求结束后记录响应信息
		latency := time.Since(startTime)
		statusCode := c.Response().StatusCode()
		loggerWithTrace.Info("Completed request",
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
		)

		return err
	}
}
