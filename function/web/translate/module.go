package translate

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"zeusro.com/hermes/function/web"
)

type Translator interface {
	Translate(source string) (cost time.Time, output string, err error)
	// Translate(source string) (cost time.Time, output []model.Language, err error)

}

type DeepSeekTranslator struct {
	ApiKey string
}

func NewDeepSeekTranslator(apiKey string) DeepSeekTranslator {
	return DeepSeekTranslator{
		ApiKey: apiKey,
	}
}

// NewDeepSeekTranslator AI翻译
// source string
// targets []string
func (d DeepSeekTranslator) Translate(source string, targets []string) (cost time.Duration, output string, err error) {
	// 猜语言这个事情交给ai算了
	// country:=
	prompt := fmt.Sprintf("使用最简短的语言将下述文字翻译成多国语言（%s）：%s", strings.Join(targets, ","), source)
	start := time.Now()
	defer func() {
		cost = time.Since(start)
		fmt.Println("DeepSeekTranslator.Translate 耗时：", cost)
	}()
	var w sync.WaitGroup
	w.Add(1)
	go func() {
		defer w.Done()
		// prompt := "使用最简短的语言将下述文字翻译成多国语言（英文，日语）：我想学习Go语言。"
		response, err := web.CallDeepSeek(prompt, d.ApiKey)
		if err != nil {
			fmt.Println("调用失败：", err)
			return
		}
		output = response
		fmt.Println("DeepSeek 返回：\n", response)
	}()
	w.Wait()
	return
}
