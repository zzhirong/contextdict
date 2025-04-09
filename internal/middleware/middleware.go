package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/didip/tollbooth/v8"
	"github.com/didip/tollbooth/v8/limiter"
	"github.com/gin-gonic/gin"
)

func LimitHandler(lmt *limiter.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		httpError := tollbooth.LimitByRequest(lmt, c.Writer, c.Request)
		if httpError != nil {
			c.Data(httpError.StatusCode, lmt.GetMessageContentType(), []byte(httpError.Message))
			c.Abort()
		} else {
			c.Next()
		}
	}
}

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
func IPRateLimiter(rate float64, expireDays int, RealIPHeaderName string) gin.HandlerFunc {
	ttl := time.Duration(expireDays) * 24 * time.Hour
	lmt := tollbooth.NewLimiter(
		rate,
		&limiter.ExpirableOptions{
			DefaultExpirationTTL: ttl,
		},
	)
	lmt.SetIPLookup((limiter.IPLookup{
		// 大小写敏感, 不能是 Cf-Connecting-Ip
		Name:           RealIPHeaderName,
		IndexFromRight: 0, // 从右往左数, 所以优先 CF-Connecting-IP
	}))
	return LimitHandler(lmt)
}
