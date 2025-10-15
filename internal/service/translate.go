package service

import (
	"fmt"
	"os"
	"sync"
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

func exportResponseToFile(lang string, response baseModel.APIResponse) error {
	filename := fmt.Sprintf("output/%s.md", lang)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%s\n\n%vs\n", time.Now().Format(time.RFC3339), response.Message)
	return err
}

// Translate 多种语言长文本翻译时可以选择触发
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
	format := s.config.OutputFormat
	responses := make([]baseModel.APIResponse, 0)
	code := 200
	if format == "console" {
		// 调用实际的翻译服务
		//console模式选择一次性翻译全部
		responses, code = s.doTranslate(request.Text, city.Language, start)
		ctx.AbortWithStatusJSON(code, responses)
		return
	}
	//非console模式选择多语言并发翻译
	for _, language := range city.Language {
		var wg sync.WaitGroup
		go func(lang string, wg *sync.WaitGroup) {
			defer wg.Done()
			wg.Add(1)
			// 调用实际的翻译服务
			resp, statusCode := s.doTranslate(request.Text, []string{lang}, start)
			if statusCode != 200 {
				code = statusCode
			}
			if len(resp) > 0 {
				exportErr := exportResponseToFile(lang, resp[0])
				if exportErr != nil {
					s.l.Errorf("导出文件失败: %v", exportErr)
				}
				responses = append(responses, resp...)
			}
		}(language, &wg)
		wg.Wait()
	}
	ctx.AbortWithStatusJSON(code, responses)
}

func (s TranslateService) doTranslate(text string, languages []string, start time.Time) ([]baseModel.APIResponse, int) {
	translator := translate.NewDeepSeekTranslator(os.Getenv("DEEPSEEK_API_KEY"))
	_, output, err := translator.Translate(text, languages)
	if err != nil {
		response := baseModel.NewErrorAPIResponse(time.Since(start), err.Error())
		return []baseModel.APIResponse{response}, response.Code
	}
	response := baseModel.NewSuccessAPIResponse(time.Since(start), output)
	return []baseModel.APIResponse{response}, response.Code
}
