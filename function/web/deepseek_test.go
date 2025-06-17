package web

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func TestCallDeepSeek(t *testing.T) {
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Fatalf("加载环境变量失败：%v", err)
	}
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	start := time.Now()
	var w sync.WaitGroup
	w.Add(1)
	go func() {
		defer w.Done()
		prompt := "使用最简短的纯文本（不包含markdown）将下述文字翻译成多国语言（英文，日语）：我想学习Go语言。"
		response, err := CallDeepSeek(prompt, apiKey)
		if err != nil {
			fmt.Println("调用失败：", err)
			return
		}
		fmt.Println("DeepSeek 返回：\n", response)
	}()
	w.Wait()
	duration := time.Since(start)
	fmt.Printf("took %s\n", duration)

}
