package server

import (
	"errors"
	"io/fs"
	"log"
	"net/http"
	"time"

	"context"

	sentry "github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"github.com/zzhirong/contextdict/config"
	"github.com/zzhirong/contextdict/internal/handlers"
	mw "github.com/zzhirong/contextdict/internal/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("gin-server")

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
	sentryDsn string,
) *GinServer {

	tp, err := initTracer()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	router := gin.New()

	if err := sentry.Init(sentry.ClientOptions{
		Dsn: sentryDsn,
		// Adds request headers and IP for users,
		// visit: https://docs.sentry.io/platforms/go/data-management/data-collected/ for more info
		SendDefaultPII: true,
	}); err != nil {
		log.Printf("Sentry initialization failed: %v\n", err)
	}

	router.Use(otelgin.Middleware("my-server"))
	router.Use(traceRole)
	router.Use(gin.Recovery())
	router.Use(gin.Logger())
	router.Use(sentrygin.New(sentrygin.Options{}))

	if rlcfg.Enabled {
		log.Printf("IP Rate Limiting enabled (Rate: %.2f/s, ExpireDays: %d)", rlcfg.Rate, rlcfg.ExpireDays)
		router.Use(mw.IPRateLimiter(rlcfg.Rate, rlcfg.ExpireDays, rlcfg.RealIPHeader))
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

	router.GET("/api", apiHandler.Handle)

	return &GinServer{
		router: router,
		addr:   addr,
	}
}

func traceRole(c *gin.Context) {
	_, span := tracer.Start(c.Request.Context(),
		"getUser",
		oteltrace.WithAttributes(
			attribute.String("id", c.Query("role")),
		),
	)
	defer span.End()
	c.Next()
}

// Start runs the Gin HTTP server.
func (s *GinServer) Start() *http.Server {
	srv := &http.Server{
		Addr:         s.addr,
		Handler:      s.router,
		ReadTimeout:  60 * time.Second, // Example timeouts
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Application server starting on %s", s.addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Application server listen error: %s\n", err)
		}
	}()
	return srv
}

func initTracer() (*sdktrace.TracerProvider, error) {
	// exporter, err := stdout.New(stdout.WithPrettyPrint())
	exporter, err := otlptrace.New(context.TODO(), otlptracehttp.NewClient())
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp, nil
}
