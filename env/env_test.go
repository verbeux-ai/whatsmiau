package env

import (
	"testing"
	"time"

	caarlosenv "github.com/caarlos0/env/v11"
)

func TestS3EnvironmentDefaultsAndOverrides(t *testing.T) {
	var cfg E
	if err := caarlosenv.ParseWithOptions(&cfg, caarlosenv.Options{Environment: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	if cfg.S3Enabled || cfg.GCSEnabled || cfg.S3PresignTTL != time.Hour {
		t.Fatal("storage must remain opt-in with a one-hour signed URL default")
	}
	values := map[string]string{
		"S3_ENABLED": "true", "S3_BUCKET": "media", "S3_REGION": "auto",
		"S3_ENDPOINT": "https://storage.example.test", "S3_USE_PATH_STYLE": "true",
		"S3_PREFIX": "core/media", "S3_PUBLIC_URL": "https://cdn.example.test",
		"S3_PRESIGN_TTL": "15m",
	}
	if err := caarlosenv.ParseWithOptions(&cfg, caarlosenv.Options{Environment: values}); err != nil {
		t.Fatal(err)
	}
	if !cfg.S3Enabled || cfg.S3Bucket != "media" || cfg.S3Region != "auto" || cfg.S3Endpoint != values["S3_ENDPOINT"] || !cfg.S3UsePathStyle || cfg.S3Prefix != "core/media" || cfg.S3PublicURL != values["S3_PUBLIC_URL"] || cfg.S3PresignTTL != 15*time.Minute {
		t.Fatal("S3 environment overrides were not parsed")
	}
	values["S3_PRESIGN_TTL"] = "not-a-duration"
	if err := caarlosenv.ParseWithOptions(&cfg, caarlosenv.Options{Environment: values}); err == nil {
		t.Fatal("invalid signed URL duration was ignored")
	}
}
