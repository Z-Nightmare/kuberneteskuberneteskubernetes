package model

import (
	"net/http"
	"time"
)

type APIResponse struct {
	Cost    time.Duration `json:"cost,omitempty"`  // 处理耗时
	Code    int           `json:"code"`            // 业务状态码（如 0 表示成功）
	Message string        `json:"message"`         // 消息提示
	Data    interface{}   `json:"data,omitempty"`  // 返回数据体，可为任意结构
	Error   string        `json:"error,omitempty"` // 可选错误描述（一般调试用）
}

func NewErrorAPIResponse(cost time.Duration, msg string) APIResponse {
	return APIResponse{
		Code:    http.StatusInternalServerError,
		Message: msg,
		Cost:    cost,
		// Error:   "An unexpected error occurred",
	}
}

func NewSuccessAPIResponse(cost time.Duration, msg string) APIResponse {
	return APIResponse{
		Code:    http.StatusOK,
		Message: msg,
		Cost:    cost,
		// Data:    nil,
	}
}

type Hermes interface {
	Translate(source Language, location Location) (target []Language, err error)
}

type Language struct {
	Word     string
	Location Location
}

// Location 简化为地球上的位置
type Location struct {
	Latitude  float64 // 纬度
	Longitude float64 // 经度
}
