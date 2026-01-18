package webprovider

import (
	"runtime/debug"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"
	"zeusro.com/hermes/internal/core/config"
	"zeusro.com/hermes/internal/core/logprovider"
	"zeusro.com/hermes/internal/middleware"
)

// FiberEngine wraps Fiber app and API router group
type FiberEngine struct {
	App *fiber.App
	Api fiber.Router
}

// NewFiberEngine creates a new Fiber engine with middleware
func NewFiberEngine(cfg config.Config) FiberEngine {
	app := fiber.New(fiber.Config{
		AppName:      "Hermes",
		ServerHeader: "Hermes",
		ErrorHandler: ErrorHandler,
	})

	zapLogger := logprovider.GetZapLogger()

	// Add trace ID middleware (already includes logging)
	app.Use(middleware.TraceIDMiddlewareFiber(zapLogger))

	// Add recovery middleware
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, e interface{}) {
			zapLogger.Error("[Recovery from panic]",
				zap.Time("time", time.Now()),
				zap.Any("error", e),
				zap.String("path", c.Path()),
				zap.String("method", c.Method()),
				zap.String("stack", string(debug.Stack())),
			)
		},
	}))

	return FiberEngine{
		App: app,
		Api: app.Group("/api"),
	}
}

// ErrorHandler is the custom error handler for Fiber
func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	return c.Status(code).JSON(fiber.Map{
		"code":    code,
		"message": err.Error(),
	})
}
