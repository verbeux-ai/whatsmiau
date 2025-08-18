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

	ApiKey    string `env:"API_KEY" envDefault:""`
	DBDialect string `ENV:"DIALECT_DB" envDefault:"sqlite3"`                   // sqlite3 or postgres
	DBURL     string `ENV:"DB_URL" envDefault:"file:data.db?_foreign_keys=on"` // "postgres://<user>:<pass>@<host>:<port>/<DB>?sslmode=disable
}

var Env E

func Load() error {
	_ = godotenv.Load(".env")
	err := env.Parse(&Env)

	return err
}
