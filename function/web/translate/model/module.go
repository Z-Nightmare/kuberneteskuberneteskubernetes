package model

import "math"

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

// Coordinates 经纬度
type Coordinates struct {
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`
}

func (receiver Coordinates) Guess(cities []City) City {
	var closest City
	minDistance := math.MaxFloat64 //todo 改一下

	for _, city := range cities {
		dist := haversine(receiver.Latitude, receiver.Longitude, city.Coordinates.Latitude, city.Coordinates.Longitude)
		if dist < minDistance {
			minDistance = dist
			closest = city
		}
	}

	return closest
}

// haversine 📌 Haversine 公式：计算地球上两点的距离
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371 // 地球半径（单位：公里）

	dLat := degreesToRadians(lat2 - lat1)
	dLon := degreesToRadians(lon2 - lon1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(degreesToRadians(lat1))*math.Cos(degreesToRadians(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180
}
