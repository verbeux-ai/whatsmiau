package env

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type E struct {
	Port      string `env:"PORT" envDefault:"8080"`
	DebugMode bool   `env:"DEBUG_MODE" envDefault:"false"`

	RedisURL      string `ENV:"REDIS_URL" envDefault:"localhost:6379"`
	RedisPassword string `ENV:"REDIS_PASSWORD"`
	RedisTLS      bool   `ENV:"REDIS_TLS" envDefault:"false"`
}

var Env E

func Load() error {
	_ = godotenv.Load(".env")
	err := env.Parse(&Env)

	return err
}
