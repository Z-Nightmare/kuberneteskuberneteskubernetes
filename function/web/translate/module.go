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

// Translate AI翻译多种语言
// source string
// targets []string
func (d DeepSeekTranslator) Translate(source string, targets []string) (cost time.Duration, output string, err error) {
	// 猜语言这个事情交给ai算了
	// country:=
	oracle := fmt.Sprintf("使用最简短的纯文本（不包含markdown，不需要注明语言类型）将下述文字翻译成目标语言（%s）：%s", strings.Join(targets, ","), source)
	fmt.Println("oracle：", oracle)
	start := time.Now()
	defer func() {
		cost = time.Since(start)
		fmt.Println("DeepSeekTranslator.Translate 耗时：", cost)
	}()
	var w sync.WaitGroup
	w.Add(1)
	go func() {
		defer w.Done()
		response, err := web.CallDeepSeek(oracle, d.ApiKey)
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
