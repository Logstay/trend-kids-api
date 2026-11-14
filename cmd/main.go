package main

import (
	"context"
	"errors"
	"log/slog"
	nethttp "net/http"
	"os"
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
		fatal("config", err)
	}
	logger, err := config.NewLogger(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		fatal("log", err)
	}
	slog.SetDefault(logger)

	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		fatal("banco", err)
	}
	defer db.Close()

	handler, err := internal.NewHandler(db, cfg, logger)
	if err != nil {
		fatal("setup", err)
	}

	srv := &nethttp.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		// Configuração efetiva, sem segredos: ajuda a achar o 403 de CORS (origem do site fora da lista).
		logger.Info("API no ar", "addr", srv.Addr, "cors_origins", cfg.AllowedOrigins, "trusted_proxies", cfg.TrustedProxies,
			"cookie_secure", cfg.CookieSecure, "shipping_flat_cents", cfg.ShippingFlatCents, "shipping_free_above_cents", cfg.ShippingFreeAboveCents)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			fatal("servidor", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
	}
	logger.Info("API encerrada")
}

func fatal(what string, err error) {
	slog.Error("falha ao iniciar", "etapa", what, "err", err)
	os.Exit(1)
}
