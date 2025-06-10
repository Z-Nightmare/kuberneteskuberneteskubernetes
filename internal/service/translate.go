package service

import (
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
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
	if err := ctx.ShouldBindJSON(&request); err != nil {
		// 参数校验失败
		ctx.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}
	city := request.Location.GuessCity(s.config.Cities, s.config.MinimumDeviationDistance)
	if city == nil {
		response := baseModel.NewErrorAPIResponse(time.Since(start), "无法识别城市")
		ctx.AbortWithStatusJSON(response.Code, response)
		return
	}
	err := godotenv.Load("../../.env")
	if err != nil {
		s.l.Errorf("加载环境变量失败：%v", err)
		return
	}
	//调用实际的翻译服务
	translator := translate.NewDeepSeekTranslator(os.Getenv("DEEPSEEK_API_KEY"))
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
