package s3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// This test uses only an explicitly configured, disposable S3 test service.
// It creates and removes its own bucket, never an application bucket.
func TestS3Integration(t *testing.T) {
	endpoint := os.Getenv("S3_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("set S3_TEST_ENDPOINT to run against a disposable S3-compatible service")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		t.Fatal(err)
	}
	client := awss3.NewFromConfig(awsConfig, func(o *awss3.Options) {
		o.BaseEndpoint, o.UsePathStyle = aws.String(endpoint), true
	})
	// The Compose service may still be starting when CI begins the tests.
	ready, stopReady := context.WithTimeout(ctx, 30*time.Second)
	defer stopReady()
	for {
		if _, err := client.ListBuckets(ready, &awss3.ListBucketsInput{}); err == nil {
			break
		}
		select {
		case <-ready.Done():
			t.Fatal("S3 test service did not become ready")
		case <-time.After(250 * time.Millisecond):
		}
	}
	bucket := "whatsmiau-s3-test-" + uuid.NewString()
	if _, err := client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		objects, err := client.ListObjectsV2(cleanup, &awss3.ListObjectsV2Input{Bucket: aws.String(bucket)})
		if err != nil {
			t.Error(err)
			return
		}
		for _, object := range objects.Contents {
			if _, err := client.DeleteObject(cleanup, &awss3.DeleteObjectInput{Bucket: aws.String(bucket), Key: object.Key}); err != nil {
				t.Error(err)
			}
		}
		if _, err := client.DeleteBucket(cleanup, &awss3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
			t.Error(err)
		}
	})
	cfg := Config{Bucket: bucket, Region: "us-east-1", Endpoint: endpoint, UsePathStyle: true, Prefix: "core", PresignTTL: 15 * time.Minute}
	s, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	readSigned := func(objectURL string) []byte {
		t.Helper()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, objectURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("signed GET returned HTTP %d", response.StatusCode)
		}
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	// Above the transfer manager's 16 MiB threshold, using only io.Reader.
	payload := bytes.Repeat([]byte("S3 multipart media\n"), 1_200_000)
	objectURL, name, err := s.Upload(ctx, "media.bin", "application/octet-stream", struct{ io.Reader }{bytes.NewReader(payload)})
	if err != nil || name != "media.bin" {
		t.Fatalf("multipart upload: name=%q, err=%v", name, err)
	}
	if sha256.Sum256(readSigned(objectURL)) != sha256.Sum256(payload) {
		t.Fatal("multipart upload/download changed the media bytes")
	}
	u, err := url.Parse(objectURL)
	if err != nil {
		t.Fatal(err)
	}
	u.RawQuery = ""
	response, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden && response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("private object was accessible without signature: HTTP %d", response.StatusCode)
	}
	for _, data := range []string{"first avatar", "do not overwrite"} {
		// A new provider has no process-local record of the prior upload.
		fresh, err := New(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		avatarURL, err := fresh.UploadBase64IfDontExists(ctx, "avatar.png", "image/png", base64.StdEncoding.EncodeToString([]byte(data)))
		if err != nil {
			t.Fatal(err)
		}
		if string(readSigned(avatarURL)) != "first avatar" {
			t.Fatal("a new provider overwrote an existing avatar")
		}
	}
}
