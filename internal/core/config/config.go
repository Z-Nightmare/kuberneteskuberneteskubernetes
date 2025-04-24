package config

import (
	"log"
	"os"

	"github.com/spf13/viper"
)

var configPath string = ".config.yaml"

func init() {
	e := os.Getenv("ENV")
	if e == "dev" {
		configPath = "config.yaml"
	}
	if e == "prod" {
		configPath = "config-prod.yaml"
	}
	if e == "test" {
		configPath = "config-test.yaml"
	}
}

type Config struct {
	Debug bool      `mapstructure:"debug"`
	Gin   GinConfig `mapstructure:"web"`
	Log   LogConfig `mapstructure:"log"`
	JWT   JWT       `mapstructure:"jwt"`
	//todo :自行按需补充

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
	config := Config{}

	viper.SetConfigType("yaml")
	viper.SetConfigFile(configPath)

	if err := viper.ReadInConfig(); err != nil {
		// exePath, _ := os.Executable()
		// log.Printf("当前执行文件路径: %s", exePath)
		log.Fatalln("无法读取配置文件: ", err.Error())
	}

	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalln("无法解析配置文件: ", err.Error())
	}
	return config
}
