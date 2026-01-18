package webprovider

import (
	"github.com/gofiber/fiber/v2/middleware/cors"
	"zeusro.com/hermes/internal/core/config"
	"zeusro.com/hermes/internal/core/logprovider"
)

type CorsMiddleware struct {
	fiber  FiberEngine
	logger logprovider.Logger
	config config.Config
}

func NewCorsMiddleware(logger logprovider.Logger,
	fiber FiberEngine,
	config config.Config) CorsMiddleware {
	return CorsMiddleware{
		fiber:  fiber,
		logger: logger,
		config: config,
	}
}

func (m CorsMiddleware) SetUp() {
	if !m.config.Gin.CORS {
		m.logger.Info("未开启CORS")
		return
	}

	m.fiber.App.Use(cors.New(cors.Config{
		AllowCredentials: true,
		AllowOrigins:     "*",
		AllowHeaders:     "*",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,HEAD,OPTIONS",
	}))

	m.logger.Info("已配置CORS")
}
