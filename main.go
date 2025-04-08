package main

import (
	"log"
	"time"
	"io/fs"

	"github.com/zzhirong/contextdict/config"
	"github.com/zzhirong/contextdict/internal/ai"
	"github.com/zzhirong/contextdict/internal/database"
	"github.com/zzhirong/contextdict/internal/handlers"
	"github.com/zzhirong/contextdict/internal/metrics"
	"github.com/zzhirong/contextdict/internal/server"

	"context"
	"os"
	"os/signal"
	"syscall"
	"sync"
	"net/http"
	"embed"
)

//go:embed frontend/dist
var embeddedFS embed.FS

func main() {
	// Load configuration (adjust path "" if needed, e.g., "./config.yaml")
	cfg := config.Load("")
	if cfg == nil {
		log.Fatal("Failed to load configuration.")
	}

	dbRepo, err := database.NewRepository(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database repository: %v", err)
	}
	defer func() {
		if err := dbRepo.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		}
	}()

	aiClient := ai.NewClient(cfg.AI)

	promMetrics := metrics.NewMetrics()

	apiHandler := handlers.NewAPIHandler(dbRepo, aiClient, promMetrics, cfg.Prompts)

	servers := make(map[string]*http.Server)
	servers["metrics"] = metrics.StartServer(":" + cfg.MetricsPort)

	// --- Static File Serving ---
	contentFS, err := fs.Sub(embeddedFS, "frontend/dist")
	if err != nil {
		log.Fatalf("Failed to create sub FS for frontend/dist: %v", err)
	}

	ginServer := server.New(":" + cfg.ServerPort, cfg.MaxURLLen, apiHandler, &cfg.RateLimit, contentFS)
	servers["application"] = ginServer.Start()

	GracefulShutdown(10*time.Second, servers) // 10-second shutdown timeout
	log.Println("Application finished.")
}


func GracefulShutdown(timeout time.Duration, servers map[string]*http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal: %s. Shutting down servers...", sig)

	// Create a context with a timeout for shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Use a WaitGroup to wait for all servers to shut down.
	var wg sync.WaitGroup // Ensure sync is imported

	for name, server := range servers {
		wg.Add(1)
		go func(name string, srv *http.Server) {
			defer wg.Done()
			log.Printf("Shutting down %s server...", name)
			// Disable keep-alives before shutting down
			srv.SetKeepAlivesEnabled(false)
			if err := srv.Shutdown(ctx); err != nil {
				log.Printf("Error shutting down %s server: %v", name, err)
			} else {
				log.Printf("%s server gracefully stopped.", name)
			}
		}(name, server) // Pass name and server explicitly to the goroutine
	}

	// Wait for all shutdowns to complete or context deadline expires.
	wg.Wait()

	select {
	case <-ctx.Done():
		log.Println("Shutdown timeout reached.")
	default:
		log.Println("All servers shut down gracefully.")
	}
}
