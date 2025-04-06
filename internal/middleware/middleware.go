package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth/v7/limiter"
	"github.com/didip/tollbooth_gin"
	"github.com/gin-gonic/gin"
)

// 为防止滥用，限制 URL 长度
func LimitURLLen(maxURLLen int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(c.Request.URL.String()) > maxURLLen {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Input length exceeds limit (%d characters)",
					maxURLLen),
			})
			c.Abort()
			return
		}
		fmt.Printf("url length check passed %d\n", len(c.Request.URL.String()))
		c.Next()
	}
}

// 根据 ip 限速，单位是
func IPRateLimiter(rate float64, expireDays int) gin.HandlerFunc {
	ttl := time.Duration(expireDays) * 24 * time.Hour
	limiter := tollbooth.NewLimiter(
		rate,
		&limiter.ExpirableOptions{
			DefaultExpirationTTL: ttl,
		},
	)
	return tollbooth_gin.LimitHandler(limiter)
}
