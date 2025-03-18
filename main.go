package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"context"
	"flag"

	openai "github.com/sashabaranov/go-openai"
)

var role_templates = map[string]string{
	"translator": "Please translate the following text to Chinese.",
}

type Config struct {
	DSApiKey   string
	DSBaseURL  string
	DSModel    string
	ServerPort string
}

func initConfig() *Config {
	var config Config

	// Get API key from command line
	flag.StringVar(&config.DSApiKey, "k", "", "DeepSeek API Key")
	if config.DSApiKey == "" {
		// if os.Getenv("DEEPSEEK_API_KEY") != "" {
		if os.Getenv("V_API_KEY") != "" {
			config.DSApiKey = os.Getenv("V_API_KEY")
		}
		if config.DSApiKey == "" {
			fmt.Println("DeepSeek API Key is required. Use -k flag to provide it.")
			os.Exit(1)
		}
	}
	flag.StringVar(&config.DSBaseURL, "u", "https://ark.cn-beijing.volces.com/api/v3", "DeepSeek API Base URL")
	// flag.StringVar(&config.DSModel, "m", "deepseek-chat", "DeepSeek Model")
	flag.StringVar(&config.DSModel, "m", "deepseek-v3-241226", "DeepSeek Model")
	flag.StringVar(&config.ServerPort, "p", "8085", "Server Port")
	flag.Parse()

	// Validate required fields
	if config.DSApiKey == "" {
		log.Fatal("DeepSeek API Key is required. Use -k flag to provide it.")
	}

	return &config
}

//go:embed frontend/dist
var distFS embed.FS

type Translation struct {
	gorm.Model
	Word    string `gorm:"index"`
	Context string `gorm:"index"`
	Result  string
}

var (
	db                 *gorm.DB
	cfg                = initConfig()
	translationCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "translation_requests_total",
			Help: "Total number of translation requests",
		},
		[]string{"type"},
	)
)

func init() {
	var err error

	db, err = gorm.Open(sqlite.Open("translations.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	err = db.AutoMigrate(&Translation{})
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	prometheus.MustRegister(translationCounter)
}

func main() {
	router := gin.Default()

	// Serve embedded static files
	assets, _ := fs.Sub(distFS, "frontend/dist/assets")
	router.StaticFS("/assets", http.FS(assets))
	dist, _ := fs.Sub(distFS, "frontend/dist")
	router.GET("/", func(c *gin.Context) {
		c.FileFromFS("/", http.FS(dist))
	})

	router.GET("/translate", handleTranslate)

	// Prometheus metrics
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := router.Run(":" + cfg.ServerPort); err != nil {
		log.Fatal(err)
	}
}

func serveIndex(dist fs.FS) gin.HandlerFunc {
	return func(c *gin.Context) {
		indexHTML, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to read index.html")
			return
		}
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, string(indexHTML))
	}
}

func handleTranslate(c *gin.Context) {
	query := c.Query("q")
	context := c.Query("context")

	if query == "" {
		c.JSON(400, gin.H{"error": "Missing query parameter"})
		return
	}

	translationType := "direct"
	if context != "" {
		translationType = "context"
	}

	translationCounter.WithLabelValues(translationType).Inc()

	// Check cache first
	var translation Translation
	result := db.Where("word = ? AND context = ?", query, context).First(&translation)
	if result.Error == nil {
		c.JSON(200, gin.H{"translation": translation.Result})
		return
	}

	// Call DeepSeek API with retries
	translation.Word = query
	translation.Context = context
	translation.Result = callDeepSeekAPI(query, context)

	// Cache the result
	db.Create(&translation)

	c.JSON(200, gin.H{"translation": translation.Result})
}

func callDeepSeekAPI(query, context string) string {
	fmt.Println(query)
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		result, err := makeDeepSeekRequest(query, context)
		if err == nil {
			return result
		}
		log.Printf("DeepSeek API call failed (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(time.Second * time.Duration(i+1))
	}
	return "Translation failed after multiple attempts"
}

func makeDeepSeekRequest(query, queryCtx string) (string, error) {
	prompt := query
	if queryCtx != "" && strings.Contains(queryCtx, query) {
		prompt = fmt.Sprintf("在这个语境下: `%s`，帮我理解这个词语：`%s`", query, queryCtx)
	}

	config := openai.DefaultConfig(cfg.DSApiKey)
	config.BaseURL = cfg.DSBaseURL

	client := openai.NewClientWithConfig(config)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: cfg.DSModel,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: role_templates["translator"],
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}
