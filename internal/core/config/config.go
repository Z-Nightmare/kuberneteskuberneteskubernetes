package config

import (
	"log"

	"github.com/spf13/viper"
	"zeusro.com/hermes/function/web/translate/model"
)

var configPath string = ".config.yaml"

func init() {
	// 根据环境变量设定不同的配置路径，可按需开启
	// e := os.Getenv("ENV")
	// if e == "dev" {
	// 	configPath = "config.yaml"
	// }
	// if e == "prod" {
	// 	configPath = "config-prod.yaml"
	// }
	// if e == "test" {
	// 	configPath = "config-test.yaml"
	// }
}

type Config struct {
	Debug                    bool         `mapstructure:"debug"`
	Gin                      GinConfig    `mapstructure:"web"`
	Log                      LogConfig    `mapstructure:"log"`
	JWT                      JWT          `mapstructure:"jwt"`
	Cities                   []model.City `yaml:"cities"`
	MinimumDeviationDistance float64      `mapstructure:"minimum_deviation_distance"` // 最小偏差距离
	OutputFormat             string       `mapstructure:"output"`                     // 输出形式
}

type JWT struct {
	SigningKey []byte
}

type GinConfig struct {
	Port int  `mapstructure:"port"`
	CORS bool `mapstructure:"cors"`
}

type LogConfig struct {
	Path  string `mapstructure:"path"`
	Level string `mapstructure:"level"`
}

func NewFileConfig() Config {
	var config Config

	viper.SetConfigType("yaml")
	viper.SetConfigFile(configPath)

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalln("无法读取配置文件:", err.Error())
	}

	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalln("无法解析配置文件:", err.Error())
	}

	return config
}
