package s3

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/verbeux-ai/whatsmiau/interfaces"
)

type Config struct {
	Bucket       string
	Region       string
	Endpoint     string
	UsePathStyle bool
	Prefix       string
	PublicURL    string
	PresignTTL   time.Duration
}

// Storage holds immutable SDK clients/configuration. Objects and existence
// checks live in S3, so uploads remain correct across replicas and restarts.
type Storage struct {
	uploader  *transfermanager.Client
	presigner *awss3.PresignClient
	bucket    string
	prefix    string
	publicURL *url.URL
	ttl       time.Duration
}

var _ interfaces.Storage = (*Storage)(nil)

func New(ctx context.Context, cfg Config) (*Storage, error) {
	if strings.TrimSpace(cfg.Bucket) == "" || strings.ContainsAny(cfg.Bucket, "/\\ \t\r\n") {
		return nil, errors.New("S3_BUCKET must be a bucket name")
	}
	if cfg.PresignTTL < time.Second || cfg.PresignTTL > 7*24*time.Hour {
		return nil, errors.New("S3_PRESIGN_TTL must be between 1s and 168h")
	}
	if cfg.Endpoint != "" {
		if _, err := parseBaseURL(cfg.Endpoint); err != nil {
			return nil, fmt.Errorf("S3_ENDPOINT: %w", err)
		}
	}
	var publicURL *url.URL
	if cfg.PublicURL != "" {
		var err error
		publicURL, err = parseBaseURL(cfg.PublicURL)
		if err != nil {
			return nil, fmt.Errorf("S3_PUBLIC_URL: %w", err)
		}
	}
	prefix := strings.Trim(cfg.Prefix, "/")
	if prefix != "" {
		if !validKey(prefix) {
			return nil, errors.New("S3_PREFIX must be an object path without dot segments")
		}
		prefix += "/"
	}
	opts := []func(*config.LoadOptions) error{
		config.WithDefaultRegion("us-east-1"),
		config.WithHTTPClient(&http.Client{Timeout: time.Minute}),
	}
	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}
	awsConfig, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load S3 configuration: %w", err)
	}
	client := awss3.NewFromConfig(awsConfig, func(o *awss3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})
	return &Storage{
		uploader: transfermanager.New(client, func(o *transfermanager.Options) {
			// Let the SDK abort incomplete multipart uploads even after the
			// request context expires, with a bounded cleanup context.
			o.FailTimeout = 10 * time.Second
		}),
		presigner: awss3.NewPresignClient(client),
		bucket:    cfg.Bucket, prefix: prefix, publicURL: publicURL, ttl: cfg.PresignTTL,
	}, nil
}

func parseBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return nil, errors.New("expected an absolute HTTP(S) URL without credentials, query or fragment")
	}
	return u, nil
}

func validKey(key string) bool {
	if key == "" || len(key) > 1024 || !utf8.ValidString(key) || strings.HasPrefix(key, "/") {
		return false
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func (s *Storage) key(fileName string) (string, error) {
	key := s.prefix + fileName
	if !validKey(fileName) || !validKey(key) {
		return "", errors.New("invalid S3 object name")
	}
	return key, nil
}

func (s *Storage) objectURL(ctx context.Context, key string) (string, error) {
	if s.publicURL != nil {
		u := *s.publicURL
		u.Path = strings.TrimRight(u.Path, "/") + "/" + key
		u.RawPath = ""
		return u.String(), nil
	}
	signed, err := s.presigner.PresignGetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	}, func(o *awss3.PresignOptions) { o.Expires = s.ttl })
	if err != nil {
		return "", fmt.Errorf("sign S3 media URL: %w", err)
	}
	return signed.URL, nil
}

func contentType(fileName, mimetype string) string {
	if mimetype == "" {
		mimetype = mime.TypeByExtension(path.Ext(fileName))
	}
	if mimetype == "" {
		mimetype = "application/octet-stream"
	}
	return mimetype
}

func (s *Storage) Upload(ctx context.Context, fileName, mimetype string, file io.Reader) (string, string, error) {
	if file == nil {
		return "", "", errors.New("S3 media reader is required")
	}
	key, err := s.key(fileName)
	if err != nil {
		return "", "", err
	}
	if _, err := s.uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), Body: file,
		ContentType: aws.String(contentType(fileName, mimetype)),
	}); err != nil {
		return "", "", fmt.Errorf("upload S3 media: %w", err)
	}
	objectURL, err := s.objectURL(ctx, key)
	if err != nil {
		return "", "", err
	}
	return objectURL, fileName, nil
}

func (s *Storage) UploadBase64IfDontExists(ctx context.Context, fileName, mimetype, encoded string) (string, error) {
	key, err := s.key(fileName)
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode S3 base64 media: %w", err)
	}
	if mimetype == "" {
		mimetype = http.DetectContentType(data)
	}
	_, err = s.uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), Body: bytes.NewReader(data),
		ContentType: aws.String(mimetype), IfNoneMatch: aws.String("*"),
	})
	if err != nil {
		var apiErr smithy.APIError
		if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "PreconditionFailed" {
			return "", fmt.Errorf("upload S3 base64 media: %w", err)
		}
	}
	return s.objectURL(ctx, key)
}
