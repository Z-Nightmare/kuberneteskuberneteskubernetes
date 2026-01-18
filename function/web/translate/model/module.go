package model

import (
	"fmt"
	"math"
)

type TranslateRequest struct {
	Text     string      `json:"text"`
	Location Coordinates `json:"location"`
}

type City struct {
	Name        string      `yaml:"name"`
	Timezone    string      `yaml:"timezone"`
	Coordinates Coordinates `yaml:"coordinates"`
	Language    []string    `yaml:"language"`
}

// Coordinates 经纬度
type Coordinates struct {
	Latitude  float64 `yaml:"latitude"`  //纬度
	Longitude float64 `yaml:"longitude"` //经度
}

// Guess 根据经纬度猜测城市
// 传入经纬度和城市列表，返回距离最近的城市
// 如果距离超过 acceptableDistance，则返回 nil.acceptableDistanc 要根据城市规模而决定，越大的城市距离越大
// fixme：实际上国内定位给出的经纬度是故意带偏移的，这部分是否要考虑
func (receiver Coordinates) GuessCity(cities []City, acceptableDistance float64) *City {
	fmt.Println("GuessCity called with receiver:", receiver)
	var closest City
	minDistance := math.MaxFloat64
	for _, city := range cities {
		dist := haversine(receiver.Latitude, receiver.Longitude, city.Coordinates.Latitude, city.Coordinates.Longitude)
		fmt.Println("current city found:", city.Name, "at distance:", dist, "km")
		if dist < minDistance {
			minDistance = dist
			closest = city
		}
	}
	fmt.Println("acceptableDistance:", acceptableDistance, ".Closest city found:", closest.Name, "at distance:", minDistance, "km")
	//距离判定函数
	if minDistance <= acceptableDistance {
		return &closest
	}
	return nil
}

// haversine 📌 Haversine 公式：计算地球上两点的距离
// 传入两点的经纬度，返回两点之间的距离（单位：公里）
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
