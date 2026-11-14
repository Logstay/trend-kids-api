package http

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	maxQueryLogLen = 200
	maxUALogLen    = 60
)

// requestLogger escreve UMA linha por requisição: método, caminho, query, status, latência, IP, origem e user-agent.
// Nunca registra corpo, cookies nem cabeçalhos de autenticação (login e pedido trazem dados pessoais).
// Fica na frente do CORS e fora do Recovery, então enxerga também os 403 de CORS e os 500 de pânico.
// Logger nil = sem log.
func requestLogger(l *slog.Logger) gin.HandlerFunc {
	if l == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		attrs := []any{"method", c.Request.Method, "path", c.Request.URL.Path}
		if q := c.Request.URL.RawQuery; q != "" {
			attrs = append(attrs, "query", truncate(q, maxQueryLogLen))
		}
		attrs = append(attrs,
			"status", status,
			"latency_ms", float64(time.Since(start).Microseconds())/1000,
			"bytes", c.Writer.Size(),
			"ip", c.ClientIP(),
		)
		if o := c.GetHeader("Origin"); o != "" {
			attrs = append(attrs, "origin", truncate(o, maxQueryLogLen))
		}
		if ua := c.GetHeader("User-Agent"); ua != "" {
			attrs = append(attrs, "ua", truncate(ua, maxUALogLen))
		}

		switch {
		case status >= 500:
			l.Error("request", attrs...)
		case status >= 400:
			l.Warn("request", attrs...)
		default:
			l.Info("request", attrs...)
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
