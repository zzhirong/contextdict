package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"


	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zzhirong/contextdict/config"
	"gorm.io/driver/postgres" // 替换 sqlite 导入
	"gorm.io/gorm"

	"context"

	"sync"

	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/time/rate"
)

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
	cfg                = config.Load("")
	translationCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "translation_requests_total",
			Help: "Total number of translation requests",
		},
		[]string{"type"},
	)
	translationCacheHitCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "translation_cache_hits_total",
			Help: "Total number of translation cache hits",
		},
		[]string{"type"},
	)
)

func init() {
	var err error
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
	)

	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		// Logger: logger.Default.LogMode(logger.Info), // 添加这行开启 SQL 日志
	})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	if err = db.AutoMigrate(&TranslationResponse{}); err != nil {
		log.Fatal("创建数据库表失败:", err)
	}

	prometheus.MustRegister(translationCounter, translationCacheHitCounter)
}

func checkKeyword(c *gin.Context) {
	keyword := c.Query("keyword")
	path := c.Request.URL.Path
	if path != "/" &&
		!strings.HasPrefix(path, "/assets") &&
		!strings.HasPrefix(path, "/metrics") &&
		keyword == "" {

		c.JSON(400, gin.H{"error": "缺少参数 keyword"})
		c.Abort()
		return
	}
	c.Next()
}

const maxParamLength = 1024 // 1k 字符限制

func checkParamLength(c *gin.Context) {
	keyword := c.Query("keyword")
	context := c.Query("context")

	if len(keyword) > maxParamLength || len(context) > maxParamLength {
		c.JSON(400, gin.H{"error": "参数长度超过限制"})
		c.Abort()
		return
	}
	c.Next()
}

// IP 限流器
type IPRateLimiter struct {
	ips   map[string]*rate.Limiter
	mu    sync.RWMutex
	rate  rate.Limit
	burst int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		ips:   make(map[string]*rate.Limiter),
		rate:  r,
		burst: b,
	}
}

func (i *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.rate, i.burst)
		i.ips[ip] = limiter
	}

	return limiter
}

func ipRateLimit(c *gin.Context) {
	ip := c.ClientIP()
	limiter := rateLimiter.getLimiter(ip)
	if !limiter.Allow() {
		c.JSON(429, gin.H{"error": "请求太频繁，请稍后再试"})
		c.Abort()
		return
	}
	c.Next()
}

var rateLimiter = NewIPRateLimiter(10, 10) // 每秒10次请求，突发最多10次

func main() {
	router := gin.Default()
	router.Use(ipRateLimit) // 添加 IP 限流中间件
	router.Use(checkKeyword)
	router.Use(checkParamLength)

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

	// Setup Prometheus metrics endpoint on port 8086
	metricsServer := &http.Server{
		Addr:    ":8086",
		Handler: promhttp.Handler(),
	}
	go func() {
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server error: %v\n", err)
		}
	}()

	log.Printf("Server starting on port %s", cfg.ServerPort)
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	// Start server in a goroutine to not block graceful shutdown handling
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	gracefulShutdown(srv, metricsServer)
}

// gracefulShutdown handles the graceful shutdown of HTTP servers
func gracefulShutdown(servers ...*http.Server) {
	// Wait for interrupt signal to gracefully shutdown the servers
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down servers...")

	// The context is used to inform the servers they have 5 seconds to finish
	// the request they are currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown all servers concurrently
	for _, server := range servers {
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
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
		log.Println("Cache hits")
		translationCacheHitCounter.WithLabelValues("translate").Inc()
		c.JSON(200, gin.H{"result": q.Translation})
		return
	}

	var prompt string
	prompts := cfg.GetPrompts()
	if q.Context != "" {
		prompt = fmt.Sprintf(prompts.TranslateOnContext, q.Keyword, q.Context)
	} else {
		prompt = fmt.Sprintf(prompts.TranslateOrFormat, q.Keyword)
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
	prompts := cfg.GetPrompts()
	prompt := fmt.Sprintf(prompts.Format, c.Query("keyword"))
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
	prompts := cfg.GetPrompts()
	prompt := fmt.Sprintf(prompts.Summarize, c.Query("keyword"))
	translationCounter.WithLabelValues("summarize").Inc()
	res, err := makeDeepSeekRequest(prompt)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Failed to format text"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"result": res,
	})
}

func makeDeepSeekRequest(prompt string) (string, error) {
	fmt.Println("prompt:", prompt)
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
