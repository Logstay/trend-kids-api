package main

import (
	"context"
	"errors"
	"log"
	nethttp "net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Logstay/trend-kids-api/config"
	"github.com/Logstay/trend-kids-api/internal"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("banco: %v", err)
	}
	defer db.Close()

	handler, err := internal.NewHandler(db, cfg)
	if err != nil {
		log.Fatalf("setup: %v", err)
	}

	srv := &nethttp.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("API ouvindo em %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			log.Fatalf("servidor: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
