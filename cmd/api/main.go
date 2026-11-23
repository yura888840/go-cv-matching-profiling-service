package main

import (
	"context"
	"github.com/yura888840/go-cv-matching-profiling-service/internal/config"
	"github.com/yura888840/go-cv-matching-profiling-service/internal/httpapi"
	"github.com/yura888840/go-cv-matching-profiling-service/internal/service"
	"github.com/yura888840/go-cv-matching-profiling-service/internal/storage/postgres"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := config.Load()
	if len(cfg.APIKey) < 32 {
		log.Fatal("API_KEY must be set to a strong key (at least 32 characters)")
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	repo, err := postgres.New(ctx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()
	allowlist, err := httpapi.NewAllowlist(cfg.AllowedIPs, cfg.AllowedDomains)
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: httpapi.Router(service.NewMatchingService(repo, cfg.MaxCVs, cfg.Workers), repo, allowlist, cfg.APIKey), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 60 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http server: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
