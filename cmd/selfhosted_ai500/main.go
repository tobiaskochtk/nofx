package main

import (
	"context"
	"log"
	"net/http"
	"nofx/internal/selfhostedai500"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := selfhostedai500.LoadConfig()

	store, err := selfhostedai500.OpenStoreWithConfig(cfg.StoreConfig())
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()

	service := selfhostedai500.NewService(cfg, store)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := service.Start(ctx); err != nil {
		log.Fatalf("start service: %v", err)
	}

	router := selfhostedai500.NewRouter(service, cfg.AuthToken)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("[selfhosted-ai500] listening on :%s", cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
	}
}
