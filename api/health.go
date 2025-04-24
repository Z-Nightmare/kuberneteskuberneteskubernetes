package api

import (
	"zeusro.com/gotemplate/internal/core/logprovider"
	"zeusro.com/gotemplate/internal/core/webprovider"
	"zeusro.com/gotemplate/internal/service"
)

type IndexRoutes struct {
	logger logprovider.Logger
	gin    webprovider.MyGinEngine
	s      service.HealthService
	// m middleware.JWTMiddleware
}

func NewIndexRoutes(logger logprovider.Logger, gin webprovider.MyGinEngine, s service.HealthService) IndexRoutes {
	return IndexRoutes{
		logger: logger,
		gin:    gin,
		s:      s,
	}
}

func (r IndexRoutes) SetUp() {
	r.gin.Gin.StaticFile("/", "./static/index.html")

	// 在主 engine 上注册路由组
	index := r.gin.Gin.Group("/")
	{
		//http://localhost:8080/health
		index.Any("health", r.s.Check)
		index.Any("healthz", r.s.Check)
	}

}
