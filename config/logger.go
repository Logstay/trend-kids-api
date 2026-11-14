package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// ParseLogLevel entende debug, info, warn e error (vazio = info).
func ParseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, fmt.Errorf("LOG_LEVEL inválido: %q (use debug, info, warn ou error)", s)
}

// NewLogger cria o logger de saída padrão. format: "text" (legível no terminal) ou "json" (para produção).
func NewLogger(level, format string) (*slog.Logger, error) {
	lvl, err := ParseLogLevel(level)
	if err != nil {
		return nil, err
	}
	opts := &slog.HandlerOptions{Level: lvl}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		return slog.New(slog.NewTextHandler(os.Stdout, opts)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stdout, opts)), nil
	}
	return nil, fmt.Errorf("LOG_FORMAT inválido: %q (use text ou json)", format)
}
