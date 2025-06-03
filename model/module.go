package model

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
