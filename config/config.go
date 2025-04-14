package config

import (
	"log"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	ServerPort  string            `yaml:"ServerPort" env-default:"8080"`
	MaxURLLen   int               `yaml:"MaxURLLen"`
	MetricsPort string            `yaml:"MetricsPort" env-default:"8086"`
	Database    DatabaseConfig    `yaml:"Database"`
	AI          AIConfig          `yaml:"AI"`
	RateLimit   RateLimitConfig   `yaml:"RateLimit"`
	Prompts     map[string]string `yaml:"Prompts"`
}

type DatabaseConfig struct {
	Host     string `yaml:"Host" env-default:"localhost"`
	Port     string `yaml:"Port" env-default:"5432"`
	User     string `yaml:"User" env-default:"postgres"`
	Password string `yaml:"Password" env:"PS_PASSWORD" env-required:"true"`
	DBName   string `yaml:"DBName" env-default:"contextdict"`
	SSLMode  string `yaml:"SSLMode" env-default:"disable"`
}

type AIConfig struct {
	APIKey  string `yaml:"APIKey" env:"DS_API_KEY" env-required:"true"`
	BaseURL string `yaml:"BaseURL"`
	Model   string `yaml:"Model"`
}

type RateLimitConfig struct {
	Enabled      bool    `yaml:"Enabled" env-default:"true"`
	Rate         float64 `yaml:"Rate" env-default:"10"` // requests per second
	ExpireDays   int     `yaml:"ExpireDays" env-default:"1"`
	RealIPHeader string  `yaml:"RealIPHeader" env-default:"CF-Connecting-IP"`
}

// 按照优先级查找配置文件
// 1. 命令行参数
// 2. /etc/contextdict/config.yaml
// 3. ./config.yaml
func FindConfigFile(path string) string {
	// 实现逻辑
	if path != "" {
		return path
	}
	// 检查 /etc/contextdict/config.yaml
	if _, err := os.ReadFile("/etc/contextdict/config.yaml"); err == nil {
		return "/etc/contextdict/config.yaml"
	}
	// 检查 ./config.yaml
	if _, err := os.ReadFile("./config.yaml"); err == nil {
		return "./config.yaml"
	}
	return ""
}

func Load(path string) *Config {
	cfg := &Config{}

	filePath := FindConfigFile(path)
	if filePath == "" {
		log.Fatal("No config file found.")
	}
	err := cleanenv.ReadConfig(filePath, cfg)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}
	// Simple validation example
	if cfg.AI.APIKey == "" {
		log.Fatalf("Warning: DS_API_KEY is not set or using default. AI features will likely fail.")
	}

	log.Println("Configuration loaded.")
	return cfg
}
