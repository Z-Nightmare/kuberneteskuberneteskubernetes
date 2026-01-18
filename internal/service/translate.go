package service

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/function/web/translate"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/function/web/translate/model"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/config"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/webprovider"
	baseModel "github.com/Z-Nightmare/kuberneteskuberneteskubernetes/model"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func NewTranslateService(fiber webprovider.FiberEngine, l logprovider.Logger,
	config config.Config) TranslateService {
	return TranslateService{
		fiber:  fiber,
		l:      l,
		config: config,
	}
}

type TranslateService struct {
	fiber  webprovider.FiberEngine
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
func (s TranslateService) Translate(c *fiber.Ctx) error {
	start := time.Now()
	var request model.TranslateRequest
	if err := c.BodyParser(&request); err != nil {
		// 参数校验失败
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	city := request.Location.GuessCity(s.config.Cities, s.config.MinimumDeviationDistance)
	if city == nil {
		response := baseModel.NewErrorAPIResponse(time.Since(start), "无法识别城市")
		return c.Status(response.Code).JSON(response)
	}
	err := godotenv.Load("../../.env")
	if err != nil {
		s.l.Errorf("加载环境变量失败：%v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "加载环境变量失败",
		})
	}
	format := s.config.OutputFormat
	responses := make([]baseModel.APIResponse, 0)
	code := 200
	if format == "console" {
		// 调用实际的翻译服务
		//console模式选择一次性翻译全部
		responses, code = s.doTranslate(request.Text, city.Language, start)
		return c.Status(code).JSON(responses)
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
	return c.Status(code).JSON(responses)
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
