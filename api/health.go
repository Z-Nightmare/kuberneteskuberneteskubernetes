package api

import (
	"zeusro.com/gotemplate/internal/core/logprovider"
	"zeusro.com/gotemplate/internal/core/webprovider"
	"zeusro.com/gotemplate/internal/service"
)

type HealthRoutes struct {
	logger logprovider.Logger
	gin    webprovider.MyGinEngine
	s      service.HealthService
	// m middleware.JWTMiddleware
}

func NewHealthRoutes(logger logprovider.Logger, gin webprovider.MyGinEngine) HealthRoutes {
	return HealthRoutes{
		logger: logger,
		gin:    gin,
	}
}

func (r HealthRoutes) SetUp() {
	admin := r.gin.Api.Group("/").Use()
	{
		admin.Any("health", r.s.Check)
		admin.Any("healthz", r.s.Check)

	}
}
