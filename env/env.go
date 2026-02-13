package env

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type E struct {
	Port           string `env:"PORT" envDefault:"8080"`
	DebugMode      bool   `env:"DEBUG_MODE" envDefault:"false"`
	DebugWhatsmeow bool   `env:"DEBUG_WHATSMEOW" envDefault:"false"`
	DebugRawMsgs   bool   `env:"DEBUG_RAW_MESSAGES" envDefault:"false"`

	RedisURL      string `env:"REDIS_URL" envDefault:"localhost:6379"`
	RedisPassword string `env:"REDIS_PASSWORD"`
	RedisTLS      bool   `env:"REDIS_TLS" envDefault:"false"`

	ApiKey    string `env:"API_KEY" envDefault:""`
	DBDialect string `env:"DIALECT_DB" envDefault:"sqlite3"`                   // sqlite3 or postgres
	DBURL     string `env:"DB_URL" envDefault:"file:data.db?_foreign_keys=on"` // "postgres://<user>:<pass>@<host>:<port>/<DB>?sslmode=disable

	// Optional: direct MongoDB persistence (so the app doesn't depend on webhooks for storage).
	// Leave MONGO_URI empty to disable.
	MongoURI string `env:"MONGO_URI" envDefault:""`
	MongoDB  string `env:"MONGO_DB" envDefault:"rocketzap"`

	// Redis Streams for event fanout (backend consumes and emits Socket.IO).
	StreamEnabled      bool   `env:"STREAM_ENABLED" envDefault:"true"`
	StreamKey          string `env:"STREAM_KEY" envDefault:"rz:events"`
	StreamMaxLen       int    `env:"STREAM_MAXLEN" envDefault:"10000"`
	StreamMaxLenApprox bool   `env:"STREAM_MAXLEN_APPROX" envDefault:"true"`

	GCSEnabled bool   `env:"GCS_ENABLED" envDefault:"false"`
	GCSBucket  string `env:"GCS_BUCKET" envDefault:"whatsmiau"`
	GCSURL     string `env:"GCS_URL" envDefault:"https://storage.googleapis.com"`

	// S3-compatible storage (AWS S3 / MinIO / R2 / etc).
	S3Enabled        bool   `env:"S3_ENABLED" envDefault:"false"`
	S3Endpoint       string `env:"S3_ENDPOINT" envDefault:""`
	S3Region         string `env:"S3_REGION" envDefault:"us-east-1"`
	S3Bucket         string `env:"S3_BUCKET" envDefault:"whatsmiau"`
	S3AccessKey      string `env:"S3_ACCESS_KEY" envDefault:""`
	S3SecretKey      string `env:"S3_SECRET_KEY" envDefault:""`
	S3UseSSL         bool   `env:"S3_USE_SSL" envDefault:"false"`
	S3ForcePathStyle bool   `env:"S3_FORCE_PATH_STYLE" envDefault:"true"`
	S3PublicURL      string `env:"S3_PUBLIC_URL" envDefault:""` // e.g. http://localhost:9000 (for MinIO)

	GCL          string `json:"GCL_APP_NAME" envDefault:"whatsmiau-br-1"`
	GCLEnabled   bool   `json:"GCL_ENABLED" envDefault:"false"`
	GCLProjectID string `json:"GCL_PROJECT_ID"`

	EmitterBufferSize    int `env:"EMITTER_BUFFER_SIZE" envDefault:"2048"`
	HandlerSemaphoreSize int `env:"HANDLER_SEMAPHORE_SIZE" envDefault:"512"`

	ProxyAddresses []string `env:"PROXY_ADDRESSES" envDefault:""`      // random choices proxies ex: <SOCKS5|HTTP|HTTPS>://<username>:<password>@<host>:<port>
	ProxyStrategy  string   `env:"PROXY_STRATEGY" envDefault:"RANDOM"` // todo: implement BALANCED
	ProxyNoMedia   bool     `env:"PROXY_NO_MEDIA" envDefault:"false"`
}

var Env E

func Load() error {
	_ = godotenv.Load(".env")
	err := env.Parse(&Env)

	return err
}
