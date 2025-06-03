package webprovider

import (
	cors "github.com/rs/cors/wrapper/gin"

	"zeusro.com/hermes/internal/core/config"
	"zeusro.com/hermes/internal/core/logprovider"
)

type CorsMiddleware struct {
	gin    MyGinEngine
	logger logprovider.Logger
	config config.Config
}

func NewCorsMiddleware(logger logprovider.Logger,
	gin MyGinEngine,
	config config.Config) CorsMiddleware {
	return CorsMiddleware{
		gin:    gin,
		logger: logger,
		config: config,
	}
}

func (m CorsMiddleware) SetUp() {
	if !m.config.Gin.CORS {
		m.logger.Info("未开启CORS")
		return
	}

	debug := m.config.Debug
	m.gin.Gin.Use(cors.New(cors.Options{
		AllowCredentials: true,
		AllowOriginFunc:  func(origin string) bool { return true },
		AllowedHeaders:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "HEAD", "OPTIONS"},
		Debug:            debug,
	}))

	m.logger.Info("已配置CORS")
}
