package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"context"
	"flag"

	openai "github.com/sashabaranov/go-openai"
)

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
	flag.StringVar(&config.DSBaseURL, "u", "https://ark.cn-beijing.volces.com/api/v3", "DeepSeek API Base URL")
	// flag.StringVar(&config.DSModel, "m", "deepseek-chat", "DeepSeek Model")
	flag.StringVar(&config.DSModel, "m", "deepseek-v3-241226", "DeepSeek Model")
	flag.StringVar(&config.ServerPort, "p", "8085", "Server Port")
	flag.Parse()

	// Validate required fields
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

	return &config
}

//go:embed frontend/dist
var distFS embed.FS

type TranslationResponse struct {
	gorm.Model
	Keyword     string `gorm:"index:idx_keyword_context" form:"keyword"`
	Context     string `gorm:"idx_keyword_context" form:"context"`
	Translation string
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

	err = db.AutoMigrate(&TranslationResponse{})
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	prometheus.MustRegister(translationCounter)
}

func checkKeyword(c *gin.Context) {
	keyword := c.Query("keyword")
	path := c.Request.URL.Path
	if path != "/" &&
		!strings.HasPrefix(path, "/assets") &&
		!strings.HasPrefix(path, "/metrics") &&
		keyword == "" {

		c.JSON(400, gin.H{"error": "Missing keyword parameter"})
		c.Abort()
		return
	}
	c.Next()
}

func main() {
	router := gin.Default()
	// Add middleware to check the parameter keyword on url paths beside /
	router.Use(checkKeyword)
	// Serve embedded static files
	assets, _ := fs.Sub(distFS, "frontend/dist/assets")
	router.StaticFS("/assets", http.FS(assets))
	dist, _ := fs.Sub(distFS, "frontend/dist")

	router.GET("/", func(c *gin.Context) {
		c.FileFromFS("/", http.FS(dist))
	})

	// API routes
	router.GET("/translate", handleTranslate)
	router.GET("/format", handleFormat)
	router.GET("/summarize", handleSummarize)

	// Prometheus metrics
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := router.Run(":" + cfg.ServerPort); err != nil {
		log.Fatal(err)
	}
}

func handleTranslate(c *gin.Context) {
	var q TranslationResponse
	var err error

	if err = c.ShouldBind(&q); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	translationCounter.WithLabelValues("translate").Inc()

	// Check cache first
	result := db.Where("keyword = ? AND context = ?", q.Keyword, q.Context).First(&q)
	if result.Error == nil {
		c.JSON(200, gin.H{"result": q.Translation})
		return
	}

	var prompt string
	// Call DeepSeek API with retries
	if q.Context != "" {
		prompt = fmt.Sprintf(`
	Please help me to understand the {{ %s }} in the context of {{ %s }} in Chinese.
	`, q.Keyword, q.Context)
	} else {
		prompt = fmt.Sprintf(`
	First you need to determin if the following text is a code snippet or a plain text.
	If it is just plain text, Please translate it to Chinese.
	If it is a code snippet, please just format the source code snippet without translation.
	Just the result, no explanation.

	%s
	`, q.Keyword)
	}
	q.Translation, err = makeDeepSeekRequest(prompt)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Failed to translate text"})
		return
	}

	// Cache the result
	db.Create(&q)

	c.JSON(http.StatusOK, gin.H{"result": q.Translation})
}

func handleFormat(c *gin.Context) {
	prompt := fmt.Sprintf(`
    Please analyze the input text. If it is plain text, translate it into Chinese.
    If it is source code, simply format the code without translating it.
    Provide only the final output without any additional explanation.

    %s
    `, c.Query("keyword"))
	translationCounter.WithLabelValues("format").Inc()
	res, err := makeDeepSeekRequest(prompt)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Failed to format text"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"result": res,
	})
}

func handleSummarize(c *gin.Context) {
	prompt := fmt.Sprintf(`
    I would like to have the content refined in its original language.

    %s
    `, c.Query("keyword"))
	translationCounter.WithLabelValues("summarize").Inc()
	res, err := makeDeepSeekRequest(prompt)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Failed to format text"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"result": res,
	})
	// 实现总结逻辑
}

func makeDeepSeekRequest(prompt string) (string, error) {
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
