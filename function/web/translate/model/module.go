package model

type TranslateRequest struct {
	Text     string `json:"text"`
	Location struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"location"`
}
