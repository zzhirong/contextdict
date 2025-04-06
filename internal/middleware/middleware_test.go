package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestIPRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(IPRateLimiter(10, 1))
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	// 测试正常请求
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("首次请求应该成功，got %v", w.Code)
	}

	// 测试频率限制
	for i := 0; i < 15; i++ {
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("应该触发频率限制，got %v", w.Code)
	}

	// 等待重置
	time.Sleep(time.Second)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("限制重置后应该成功，got %v", w.Code)
	}
}

func TestLimitURLLen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(LimitURLLen(100))
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	// 测试正常请求
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test?url=URL_ADDRESS.com", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("请求应该成功，got %v", w.Code)
	}
	// 测试过长的 URL
	w = httptest.NewRecorder()
	longURL := "https://very-long-domain-name.com/path?param=" + strings.Repeat("abcdefghijk", 15) // 这会生成一个超过 100 字符的 URL
	req, _ = http.NewRequest("GET", "/test?url="+longURL, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("应该触发长度限制，got %v", w.Code)
	}
}
