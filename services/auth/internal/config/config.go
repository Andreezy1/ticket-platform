package config

import "github.com/caarlos0/env/v11"

type Config struct {
	HTTPPort string `env:"HTTP_PORT" envDefault:"8080"`

	DBHost     string `env:"DB_HOST,required"`
	DBPort     string `env:"DB_PORT,required"`
	DBUser     string `env:"DB_USER,required"`
	DBPassword string `env:"DB_PASSWORD,required"`
	DBName     string `env:"DB_NAME,required"`
	DBSSLMode  string `env:"DB_SSLMODE" envDefault:"disable"`

	// RedisAddress string `env:"REDIS_ADDRESS,required"`
	AccessTokenTTL  string `env:"ACCESS_TOKEN_TTL" envDefault:"15m"`
	RefreshTokenTTL string `env:"REFRESH_TOKEN_TTL" envDefault:"720h"`
}

func Load() (*Config, error) {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
