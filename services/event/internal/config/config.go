package config

import (
	"github.com/caarlos0/env/v11"
)

type Config struct {
	HTTPPort string `env:"EVENT_HTTP_PORT" envDefault:"8081"`

	DBHost     string `env:"EVENT_DB_HOST,required"`
	DBPort     string `env:"EVENT_DB_PORT,required"`
	DBUser     string `env:"EVENT_DB_USER,required"`
	DBPassword string `env:"EVENT_DB_PASSWORD,required"`
	DBName     string `env:"EVENT_DB_NAME,required"`
	DBSSLMode  string `env:"EVENT_DB_SSLMODE" envDefault:"disable"`
}

func Load() (*Config, error) {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
