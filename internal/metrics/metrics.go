package metrics

import (
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the Prometheus counters.
type Metrics struct {
	TranslationCounter      *prometheus.CounterVec
	TranslationCacheHitCounter *prometheus.CounterVec
    // Add other metrics here if needed
}

// NewMetrics initializes and registers Prometheus metrics.
func NewMetrics() *Metrics {
	m := &Metrics{
		TranslationCounter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "app_translation_requests_total", // Added prefix for clarity
				Help: "Total number of translation requests by type",
			},
			[]string{"type"}, // e.g., "translate", "translate_context", "format", "summarize"
		),
		TranslationCacheHitCounter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "app_translation_cache_hits_total",
				Help: "Total number of translation cache hits by type",
			},
			[]string{"type"}, // "translate" (only translate uses cache currently)
		),
	}
	log.Println("Prometheus metrics registered.")
	return m
}

// StartServer starts the Prometheus metrics HTTP server.
func StartServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Starting Prometheus metrics server on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v\n", err)
		}
	}()

	return server
}
