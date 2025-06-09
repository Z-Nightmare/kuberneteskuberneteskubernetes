package service

import (
	"time"

	"github.com/gin-gonic/gin"
	"zeusro.com/hermes/function/web/translate"
	"zeusro.com/hermes/function/web/translate/model"
	"zeusro.com/hermes/internal/core/config"
	"zeusro.com/hermes/internal/core/logprovider"
	"zeusro.com/hermes/internal/core/webprovider"
	baseModel "zeusro.com/hermes/model"
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
	start := time.Now()
	var request model.TranslateRequest
	city := request.Location.GuessCity(s.config.Cities, s.config.MinimumDeviationDistance)
	if city == nil {
		response := baseModel.NewErrorAPIResponse(time.Since(start), "无法识别城市")
		ctx.AbortWithStatusJSON(response.Code, response)
		return
	}
	//调用实际的翻译服务
	translator := translate.DeepSeekTranslator{}
	// cost time.Duration, output string, err error
	_, output, err := translator.Translate(request.Text, city.Language)
	if err != nil {
		response := baseModel.NewErrorAPIResponse(time.Since(start), err.Error())
		ctx.AbortWithStatusJSON(response.Code, response)
		return
	}
	response := baseModel.NewSuccessAPIResponse(time.Since(start), output)
	ctx.AbortWithStatusJSON(response.Code, response)
}
