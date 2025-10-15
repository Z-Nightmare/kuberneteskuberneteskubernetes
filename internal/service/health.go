package service

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"zeusro.com/hermes/internal/core/config"
	"zeusro.com/hermes/internal/core/logprovider"
	"zeusro.com/hermes/internal/core/webprovider"
)

func NewHealthService(gin webprovider.MyGinEngine, l logprovider.Logger,
	config config.Config) HealthService {
	return HealthService{
		gin:    gin,
		l:      l,
		config: config,
	}
}

type HealthService struct {
	gin    webprovider.MyGinEngine
	l      logprovider.Logger
	config config.Config
}

func (s HealthService) Check(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusOK, struct {
		Code int `json:"code"`
	}{200})
}
