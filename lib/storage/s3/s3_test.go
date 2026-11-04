package s3

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func testConfig(t *testing.T, endpoint string) Config {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "s3-unit-test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "s3-unit-test-secret")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", t.TempDir()+"/credentials")
	return Config{Bucket: "media", Region: "us-east-1", Endpoint: endpoint, UsePathStyle: true, Prefix: "core", PresignTTL: time.Hour}
}

func TestUploadPreservesObjectAndReturnsEscapedPublicURL(t *testing.T) {
	var body, key, contentType, auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body, key, contentType, auth = string(data), r.URL.Path, r.Header.Get("Content-Type"), r.Header.Get("Authorization")
		if r.Header.Get("X-Amz-Acl") != "" {
			t.Error("uploads must not make objects public through an ACL")
		}
		w.Header().Set("ETag", `"test"`)
	}))
	defer server.Close()
	cfg := testConfig(t, server.URL)
	cfg.PublicURL = "https://cdn.example.test/bucket/"
	s, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	// A non-seekable input must work through the SDK transfer manager.
	objectURL, name, err := s.Upload(context.Background(), "folder/áudio ?#%.ogg", "audio/ogg", struct{ io.Reader }{strings.NewReader("audio bytes")})
	if err != nil {
		t.Fatal(err)
	}
	if body != "audio bytes" || key != "/media/core/folder/áudio ?#%.ogg" || contentType != "audio/ogg" {
		t.Fatalf("unexpected upload: key=%q, body=%q, content-type=%q", key, body, contentType)
	}
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
		t.Fatal("upload was not signed by the SDK")
	}
	if name != "folder/áudio ?#%.ogg" || objectURL != "https://cdn.example.test/bucket/core/folder/%C3%A1udio%20%3F%23%25.ogg" {
		t.Fatalf("unexpected upload result: name=%q, URL=%q", name, objectURL)
	}
}

func TestPrivateURLIsPresignedWithConfiguredExpiryAndSessionCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("ETag", `"test"`) }))
	defer server.Close()
	cfg := testConfig(t, server.URL)
	t.Setenv("AWS_SESSION_TOKEN", "unit-session-token")
	cfg.PresignTTL = 15 * time.Minute
	s, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	objectURL, _, err := s.Upload(context.Background(), "private.txt", "", strings.NewReader("private"))
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(objectURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Path != "/media/core/private.txt" || q.Get("X-Amz-Expires") != "900" || q.Get("X-Amz-Signature") == "" || q.Get("X-Amz-Security-Token") != "unit-session-token" {
		t.Fatalf("missing private URL signing parameters: path=%s, fields=%v", u.Path, q.Has("X-Amz-Signature"))
	}
}

func TestBase64ConditionalUploadAcrossFreshProviders(t *testing.T) {
	var mutex sync.Mutex
	stored := ""
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()
		if r.Method != http.MethodPut || r.Header.Get("If-None-Match") != "*" || r.URL.Path != "/media/core/avatar.png" {
			t.Error("expected an atomic conditional object write")
		}
		if stored != "" {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusPreconditionFailed)
			fmt.Fprint(w, "<Error><Code>PreconditionFailed</Code></Error>")
			return
		}
		data, _ := io.ReadAll(r.Body)
		stored = string(data)
		writes++
		w.Header().Set("ETag", `"avatar"`)
	}))
	defer server.Close()
	cfg := testConfig(t, server.URL)
	cfg.PublicURL = "https://cdn.example.test"
	for _, contents := range []string{"original", "replacement"} {
		s, err := New(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		objectURL, err := s.UploadBase64IfDontExists(context.Background(), "avatar.png", "image/png", base64.StdEncoding.EncodeToString([]byte(contents)))
		if err != nil || objectURL != "https://cdn.example.test/core/avatar.png" {
			t.Fatalf("URL=%q, err=%v", objectURL, err)
		}
	}
	if stored != "original" || writes != 1 {
		t.Fatalf("existing object changed: body=%q, writes=%d", stored, writes)
	}
}

func TestStorageFailuresAndCancellationReturnNoSuccessURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "<Error><Code>AccessDenied</Code></Error>")
	}))
	defer server.Close()
	cfg := testConfig(t, server.URL)
	cfg.PublicURL = "https://cdn.example.test"
	s, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if objectURL, _, err := s.Upload(context.Background(), "file.bin", "", strings.NewReader("bytes")); err == nil || objectURL != "" {
		t.Fatal("failed upload returned success")
	}
	if objectURL, err := s.UploadBase64IfDontExists(context.Background(), "file.bin", "", "ZGF0YQ=="); err == nil || objectURL != "" {
		t.Fatal("AccessDenied was treated as an existing object")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if objectURL, _, err := s.Upload(ctx, "file.bin", "", strings.NewReader("bytes")); !errors.Is(err, context.Canceled) || objectURL != "" {
		t.Fatalf("cancellation was not propagated: URL=%q, err=%v", objectURL, err)
	}
	if objectURL, err := s.UploadBase64IfDontExists(ctx, "file.bin", "", "not base64!"); err == nil || objectURL != "" {
		t.Fatal("invalid base64 was accepted")
	}
}

func TestCanceledMultipartUploadIsAbortedUsingSDKCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	aborted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Query().Has("uploads"):
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, "<InitiateMultipartUploadResult><Bucket>media</Bucket><Key>core/large.bin</Key><UploadId>upload-1</UploadId></InitiateMultipartUploadResult>")
		case r.Method == http.MethodPut && r.URL.Query().Get("uploadId") == "upload-1":
			cancel()
			w.Header().Set("ETag", `"part"`)
		case r.Method == http.MethodDelete && r.URL.Query().Get("uploadId") == "upload-1":
			aborted <- struct{}{}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected multipart request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	s, err := New(context.Background(), testConfig(t, server.URL))
	if err != nil {
		t.Fatal(err)
	}
	objectURL, _, err := s.Upload(ctx, "large.bin", "application/octet-stream", bytes.NewReader(make([]byte, 20*1024*1024)))
	if err == nil || objectURL != "" {
		t.Fatalf("canceled multipart upload returned success: URL=%q, err=%v", objectURL, err)
	}
	select {
	case <-aborted:
	default:
		t.Fatal("SDK did not abort the incomplete upload after cancellation")
	}
}

func TestInvalidConfigurationIsRejectedBeforeCredentialsAreLoaded(t *testing.T) {
	base := Config{Bucket: "media", PresignTTL: time.Hour}
	for name, change := range map[string]func(*Config){
		"bucket":               func(c *Config) { c.Bucket = "" },
		"endpoint":             func(c *Config) { c.Endpoint = "ftp://storage.example.test" },
		"endpoint credentials": func(c *Config) { c.Endpoint = "https://user:secret@storage.example.test" },
		"public query":         func(c *Config) { c.PublicURL = "https://cdn.example.test?token=secret" },
		"public relative":      func(c *Config) { c.PublicURL = "/media" },
		"prefix traversal":     func(c *Config) { c.Prefix = "core/../other" },
		"zero expiry":          func(c *Config) { c.PresignTTL = 0 },
		"long expiry":          func(c *Config) { c.PresignTTL = 169 * time.Hour },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := base
			change(&cfg)
			if _, err := New(context.Background(), cfg); err == nil {
				t.Fatal("invalid config was accepted")
			}
		})
	}
}

func TestObjectNamesCannotEscapePrefixOrChangeURLMeaning(t *testing.T) {
	s := &Storage{prefix: "core/"}
	for _, key := range []string{"", "/absolute.png", "../outside.png", "folder/./file", strings.Repeat("x", 1024)} {
		if _, err := s.key(key); err == nil {
			t.Fatalf("invalid key was accepted: %.30q", key)
		}
	}
}
