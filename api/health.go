package api

import (
	"github.com/gin-gonic/gin"
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

	r.gin.Gin.GET("/index", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	index := r.gin.Gin.Group("/api")
	{
		//http://localhost:8080/api/health
		index.OPTIONS("health", r.s.Check)
		index.GET("health", r.s.Check)
		index.OPTIONS("healthz", r.s.Check)
		index.GET("healthz", r.s.Check)
	}

}
