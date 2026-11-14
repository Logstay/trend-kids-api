package config

import (
	"fmt"
	"net"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	DatabaseURL    string   `env:"DATABASE_URL,required"`
	Port           string   `env:"PORT" envDefault:"8080"`
	AllowedOrigins []string `env:"CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000"`

	// Mínimo de 32 caracteres (validado ao subir). Gere com: openssl rand -hex 32
	JWTSecret    string        `env:"JWT_SECRET,required"`
	SessionTTL   time.Duration `env:"SESSION_TTL" envDefault:"12h"`
	CookieSecure bool          `env:"COOKIE_SECURE" envDefault:"true"`

	// IPs/CIDRs do proxy na frente da API (o servidor Next). Só deles o X-Forwarded-For é aceito.
	TrustedProxies []string `env:"TRUSTED_PROXIES" envSeparator:","`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	for _, p := range cfg.TrustedProxies {
		if net.ParseIP(p) == nil {
			if _, _, err := net.ParseCIDR(p); err != nil {
				return nil, fmt.Errorf("TRUSTED_PROXIES inválido: %q", p)
			}
		}
	}
	return cfg, nil
}
