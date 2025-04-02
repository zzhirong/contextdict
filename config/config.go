package config

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/spf13/viper"
)

// 在 Config 结构体前添加 Prompts 定义
type Prompts struct {
	Format             string `mapstructure:"format"`
	Summarize          string `mapstructure:"summarize"`
	TranslateOnContext string `mapstructure:"translate_on_context"`
	TranslateOrFormat  string `mapstructure:"translate_or_format"`
}

type Config struct {
	DSApiKey         string  `mapstructure:"ds_api_key"`
	DSBaseURL        string  `mapstructure:"ds_base_url"`
	DSModel          string  `mapstructure:"ds_model"`
	ServerPort       string  `mapstructure:"server_port"`
	MetricServerPort string  `mapstructure:"metric_server_port"`
	Prompts          Prompts `mapstructure:"prompts"`
	Database         struct {
		Host     string `mapstructure:"host"`
		Port     string `mapstructure:"port"`
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		DBName   string `mapstructure:"dbname"`
		SSLMode  string `mapstructure:"sslmode"`
	} `mapstructure:"database"`
}

// 修改返回类型
func (c *Config) GetPrompts() Prompts {
	return c.Prompts
}

// 尝试从三个位置加载配置
// 1. 命令行参数 -c
// 2. 当前目录
// 3. 系统配置目录 /etc
func Load(path string) *Config {
	viper.SetConfigName("config") // 一定要在 SetConfigName 之前调用
	viper.SetConfigType("yaml")

	if path != "" {
		viper.AddConfigPath(filepath.Dir(path))
		viper.SetConfigName(filepath.Base(path))
	}

	// 使用指定的配置文件路径
	configPath := flag.String("c", "", "配置文件路径")
	flag.Parse()
	if *configPath != "" {
		viper.AddConfigPath(filepath.Dir(*configPath))
		viper.SetConfigName(filepath.Base(*configPath))
	}

	viper.AddConfigPath(".") // 当前目录, 主要用于开发调试

	viper.AddConfigPath("/etc") // 系统配置目录

	viper.SetDefault("ds_base_url", "https://ark.cn-beijing.volces.com/api/v3")
	viper.SetDefault("ds_model", "ep-20250314123811-lt8tx")
	viper.SetDefault("server_port", "8085")

	// 启用环境变量支持
	viper.BindEnv("ds_api_key", "DS_API_KEY")
	viper.BindEnv("database.password", "PS_PASSWORD")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Printf("配置文件没找到:%v", err)
		}
		log.Fatalf("无法读取配置文件:%v", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("无法解析配置: %v", err)
	}

	if config.DSApiKey == "" {
		log.Fatal("DeepSeek API Key 是必需的")
	}

	return &config
}
