package service

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"zeusro.com/hermes/internal/core/config"
	"zeusro.com/hermes/internal/core/logprovider"
	"zeusro.com/hermes/internal/core/webprovider"
)

func NewTranslateService(gin webprovider.MyGinEngine, l logprovider.Logger,
	config config.Config) TranslateService {
	return TranslateService{
		gin:    gin,
		l:      l,
		config: config,
	}
}

type TranslateService struct {
	gin    webprovider.MyGinEngine
	l      logprovider.Logger
	config config.Config
}

func (s TranslateService) Translate(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusOK, struct {
		Code int `json:"code"`
	}{200})
}
