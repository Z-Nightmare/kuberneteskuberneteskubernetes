package api

import (
	"github.com/gin-gonic/gin"
	"zeusro.com/hermes/internal/core/logprovider"
	"zeusro.com/hermes/internal/core/webprovider"
	"zeusro.com/hermes/internal/service"
)

type IndexRoutes struct {
	logger logprovider.Logger
	gin    webprovider.MyGinEngine
	health service.HealthService
	// m middleware.JWTMiddleware
}

func NewIndexRoutes(logger logprovider.Logger, gin webprovider.MyGinEngine, s service.HealthService) IndexRoutes {
	return IndexRoutes{
		logger: logger,
		gin:    gin,
		health: s,
	}
}

func (r IndexRoutes) SetUp() {

	r.gin.Gin.GET("/index", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	r.gin.Gin.POST("/hermes", func(c *gin.Context) {

	})

	index := r.gin.Gin.Group("/api")
	{
		//http://localhost:8080/api/health
		index.OPTIONS("health", r.health.Check)
		index.GET("health", r.health.Check)
		index.OPTIONS("healthz", r.health.Check)
		index.GET("healthz", r.health.Check)
	}

}
