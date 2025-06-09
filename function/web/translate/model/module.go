package model

type TranslateRequest struct {
	Text     string `json:"text"`
	Location struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"location"`
}

type City struct {
	Name        string      `yaml:"name"`
	Timezone    string      `yaml:"timezone"`
	Coordinates Coordinates `yaml:"coordinates"`
	Language    []string    `yaml:"language"`
}

type Coordinates struct {
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`
}
