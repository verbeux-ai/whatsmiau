package storage

import (
	"context"
	"errors"

	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib/storage/gcs"
	"github.com/verbeux-ai/whatsmiau/lib/storage/s3"
)

// New selects the configured provider without changing the media storage contract.
func New(ctx context.Context, cfg env.E) (interfaces.Storage, error) {
	if cfg.GCSEnabled && cfg.S3Enabled {
		return nil, errors.New("enable only one media storage provider: GCS_ENABLED or S3_ENABLED")
	}
	if cfg.S3Enabled {
		return s3.New(ctx, s3.Config{
			Bucket: cfg.S3Bucket, Region: cfg.S3Region, Endpoint: cfg.S3Endpoint,
			UsePathStyle: cfg.S3UsePathStyle, Prefix: cfg.S3Prefix,
			PublicURL: cfg.S3PublicURL, PresignTTL: cfg.S3PresignTTL,
		})
	}
	if cfg.GCSEnabled {
		return gcs.New(cfg.GCSBucket)
	}
	return nil, nil
}
