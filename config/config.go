package config

import (
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
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
