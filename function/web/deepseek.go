package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const deepSeekAPI = "https://api.deepseek.com/v1/chat/completions" // 替

// Message 表示一次对话消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RequestPayload 是发送给 DeepSeek 的请求体
type RequestPayload struct {
	Model    string    `json:"model"`    // 使用 deepseek-chat 模型
	Messages []Message `json:"messages"` // 对话内容
}

// Choice 表示返回的选择
type Choice struct {
	Message Message `json:"message"`
}

// APIResponse 是 DeepSeek 的响应结构
type APIResponse struct {
	Choices []Choice `json:"choices"`
}

// 调用 DeepSeek API 进行文本交互
func CallDeepSeek(prompt string, apiKey string) (string, error) {
	payload := RequestPayload{
		Model: "deepseek-chat",
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", deepSeekAPI, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("请求失败: %s", body)
	}

	var result APIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("无返回结果")
	}

	return result.Choices[0].Message.Content, nil
}
