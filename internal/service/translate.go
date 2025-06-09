package service

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"zeusro.com/hermes/function/web/translate/model"
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
	var request model.TranslateRequest
	// city :=
	city := request.Location.GuessCity(s.config.Cities, s.config.MinimumDeviationDistance)
	if city == nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, struct {
			Code int `json:"code"`
		}{200})
	}

	ctx.AbortWithStatusJSON(http.StatusOK, struct {
		Code int `json:"code"`
	}{200})
}
