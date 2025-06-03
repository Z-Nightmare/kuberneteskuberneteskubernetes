package web

import (
	"fmt"
	"os"
	"testing"
)

func TestCallDeepSeek(t *testing.T) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	// 示例调用：翻译文本
	prompt := "请把这句话翻译成英文：我想学习Go语言。"
	response, err := CallDeepSeek(prompt, apiKey)
	if err != nil {
		fmt.Println("调用失败：", err)
		return
	}
	fmt.Println("DeepSeek 返回：\n", response)
}
