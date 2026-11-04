package storage

import (
	"context"
	"testing"
	"time"

	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/lib/storage/s3"
)

func TestProviderSelection(t *testing.T) {
	ctx := context.Background()
	if provider, err := New(ctx, env.E{}); err != nil || provider != nil {
		t.Fatalf("disabled storage: provider=%T, err=%v", provider, err)
	}
	if _, err := New(ctx, env.E{GCSEnabled: true, S3Enabled: true}); err == nil {
		t.Fatal("ambiguous storage configuration was accepted")
	}
	t.Setenv("AWS_ACCESS_KEY_ID", "s3-factory-test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "s3-factory-test-secret")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	provider, err := New(ctx, env.E{S3Enabled: true, S3Bucket: "media", S3PresignTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := provider.(*s3.Storage); !ok {
		t.Fatalf("S3 was not selected: %T", provider)
	}
}
