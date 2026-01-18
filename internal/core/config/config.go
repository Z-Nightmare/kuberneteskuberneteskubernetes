package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/function/web/translate/model"
	"github.com/spf13/viper"
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
	Debug                    bool          `mapstructure:"debug"`
	Gin                      GinConfig     `mapstructure:"web"`
	Log                      LogConfig     `mapstructure:"log"`
	JWT                      JWT           `mapstructure:"jwt"`
	Storage                  StorageConfig `mapstructure:"storage"`
	Cities                   []model.City  `yaml:"cities"`
	MinimumDeviationDistance float64       `mapstructure:"minimum_deviation_distance"` // 最小偏差距离
	OutputFormat             string        `mapstructure:"output"`                     // 输出形式
}

type JWT struct {
	SigningKey []byte
}

type GinConfig struct {
	Port int  `mapstructure:"port"`
	CORS bool `mapstructure:"cors"`
}

// WebConfig is an alias for GinConfig for backward compatibility
type WebConfig = GinConfig

type LogConfig struct {
	Path  string `mapstructure:"path"`
	Level string `mapstructure:"level"`
}

type StorageConfig struct {
	Type  string      `mapstructure:"type"` // memory / mysql / etcd
	MySQL MySQLConfig `mapstructure:"mysql"`
	Etcd  EtcdConfig  `mapstructure:"etcd"`
}

type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type EtcdConfig struct {
	Endpoints   []string `mapstructure:"endpoints"`
	DialTimeout string   `mapstructure:"dial_timeout"`
	Username    string   `mapstructure:"username"`
	Password    string   `mapstructure:"password"`
}

func NewFileConfig() Config {
	var config Config

	// 允许通过环境变量覆盖配置路径（便于 cmd/k3 之类的 CLI 管理多实例/多配置）
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		configPath = p
	}
	// 如果默认配置不存在，则回退到示例配置，保证开箱可运行（尤其是 cmd/web 看板）
	if _, err := os.Stat(configPath); err != nil {
		if _, err2 := os.Stat(filepath.Join("configs", "config-example.yaml")); err2 == nil {
			configPath = filepath.Join("configs", "config-example.yaml")
		}
	}

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
