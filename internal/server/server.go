package server

import (
	"errors"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zzhirong/contextdict/config"
	"github.com/zzhirong/contextdict/internal/handlers"
	mw "github.com/zzhirong/contextdict/internal/middleware"
)

// GinServer holds the Gin engine and configuration.
type GinServer struct {
	router *gin.Engine
	addr   string
}

func New(
	addr string,
	maxURLLen int,
	apiHandler *handlers.APIHandler,
	rlcfg *config.RateLimitConfig,
	contentFS fs.FS, // Pass embedded FS
) *GinServer {

	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	if rlcfg.Enabled {
		log.Printf("IP Rate Limiting enabled (Rate: %.2f/s, ExpireDays: %d)", rlcfg.Rate, rlcfg.ExpireDays)
		router.Use(mw.IPRateLimiter(rlcfg.Rate, rlcfg.ExpireDays))
	} else {
		log.Println("IP Rate Limiting disabled.")
	}

	router.Use(mw.LimitURLLen(maxURLLen))

	router.GET("/", func(c *gin.Context) {
		// 注意：不能是 c.FileFromFS("/index.html", http.FS(contentFS)), 不然会被重定向到 `/`
		c.FileFromFS("/", http.FS(contentFS))
	})

	assetsFS, err := fs.Sub(contentFS, "assets")
	if err != nil {
		log.Fatalf("Failed to create sub FS: %v", err)
	}
	router.StaticFS("/assets", http.FS(assetsFS))

	apiGroup := router.Group("/api")
	{
		apiGroup.GET("/translate", apiHandler.Translate)
		apiGroup.GET("/format", apiHandler.Format)
		apiGroup.GET("/summarize", apiHandler.Summarize)
	}

	return &GinServer{
		router: router,
		addr:   addr,
	}
}

// Start runs the Gin HTTP server.
func (s *GinServer) Start() *http.Server {
	srv := &http.Server{
		Addr:         s.addr,
		Handler:      s.router,
		ReadTimeout:  10 * time.Second, // Example timeouts
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Application server starting on %s", s.addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Application server listen error: %s\n", err)
		}
	}()
	return srv
}
