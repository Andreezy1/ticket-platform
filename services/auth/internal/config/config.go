package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	HTTPPort string `env:"HTTP_PORT" envDefault:"8080"`

	DBHost     string `env:"DB_HOST,required"`
	DBPort     string `env:"DB_PORT,required"`
	DBUser     string `env:"DB_USER,required"`
	DBPassword string `env:"DB_PASSWORD,required"`
	DBName     string `env:"DB_NAME,required"`
	DBSSLMode  string `env:"DB_SSLMODE" envDefault:"disable"`

	// RedisAddress string `env:"REDIS_ADDRESS,required"`
	AccessTokenTTL    time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"15m"`
	RefreshTokenTTL   time.Duration `env:"REFRESH_TOKEN_TTL" envDefault:"720h"`
	RefreshReuseGrace time.Duration `env:"REFRESH_REUSE_GRACE" envDefault:"15s"`

	AUTH_PRIVATE_KEY_PATH string `env:"AUTH_PRIVATE_KEY_PATH" envDefault:"./keys/ed25519_private.pem"`
	AUTH_ISSUER           string `env:"AUTH_ISSUER"`
}

func Load() (*Config, error) {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
