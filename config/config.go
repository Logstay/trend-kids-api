package config

import "github.com/caarlos0/env/v11"

type Config struct {
	DatabaseURL    string   `env:"DATABASE_URL,required"`
	Port           string   `env:"PORT" envDefault:"8080"`
	AllowedOrigins []string `env:"CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
