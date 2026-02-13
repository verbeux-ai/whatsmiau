package s3

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/verbeux-ai/whatsmiau/interfaces"
)

var _ interfaces.Storage = (*Store)(nil)

type Store struct {
	client    *s3.Client
	bucket    string
	publicURL string
	region    string
}

type Options struct {
	Endpoint       string
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	UseSSL         bool
	ForcePathStyle bool
	PublicURL      string // if empty, we try to infer from endpoint
}

func New(ctx context.Context, opt Options) (*Store, error) {
	if opt.Region == "" {
		opt.Region = "us-east-1"
	}
	if opt.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}

	// Endpoint is required for MinIO and other S3-compatible providers.
	// For AWS S3, this can be left empty.
	var endpointResolver aws.EndpointResolverWithOptions
	if strings.TrimSpace(opt.Endpoint) != "" {
		scheme := "http"
		if opt.UseSSL {
			scheme = "https"
		}
		endpointResolver = aws.EndpointResolverWithOptionsFunc(func(service, region string, _ ...any) (aws.Endpoint, error) {
			if service == s3.ServiceID {
				return aws.Endpoint{
					URL:               fmt.Sprintf("%s://%s", scheme, strings.TrimPrefix(opt.Endpoint, scheme+"://")),
					SigningRegion:     opt.Region,
					HostnameImmutable: true,
				}, nil
			}
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		})
	}

	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(opt.Region),
		func(lo *config.LoadOptions) error {
			if endpointResolver != nil {
				lo.EndpointResolverWithOptions = endpointResolver
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	// Only override credentials if explicitly provided.
	// This keeps AWS default credential chain working (env/instance profile/etc).
	if strings.TrimSpace(opt.AccessKey) != "" || strings.TrimSpace(opt.SecretKey) != "" {
		cfg.Credentials = aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(opt.AccessKey, opt.SecretKey, ""))
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = opt.ForcePathStyle
	})

	publicURL := strings.TrimRight(opt.PublicURL, "/")
	if publicURL == "" && strings.TrimSpace(opt.Endpoint) != "" {
		scheme := "http"
		if opt.UseSSL {
			scheme = "https"
		}
		publicURL = fmt.Sprintf("%s://%s", scheme, strings.TrimPrefix(opt.Endpoint, scheme+"://"))
	}

	return &Store{
		client:    client,
		bucket:    opt.Bucket,
		publicURL: publicURL,
		region:    opt.Region,
	}, nil
}

func (s *Store) UploadBase64(ctx context.Context, fileName, mimetype, b64 string) (string, error) {
	file, newFileName, err := base64ToReader(b64, mimetype, fileName)
	if err != nil {
		return "", err
	}

	if mimetype == "" {
		mimetype = mime.TypeByExtension(filepath.Ext(newFileName))
	}

	url, _, err := s.Upload(ctx, newFileName, mimetype, file)
	if err != nil {
		return "", err
	}

	return url, nil
}

func base64ToReader(encodedData, mimeType, fileName string) (io.Reader, string, error) {
	decodedData, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, "", err
	}

	ext := filepath.Ext(fileName)
	if ext == "" {
		var dataSample []byte
		if len(decodedData) > 512 {
			dataSample = decodedData[:512]
		} else {
			dataSample = decodedData
		}
		detected := http.DetectContentType(dataSample)
		if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
			ext = exts[0]
		} else if exts, _ := mime.ExtensionsByType(detected); len(exts) > 0 {
			ext = exts[0]
		}
	}

	filename := uuid.New().String() + ext
	return bytes.NewReader(decodedData), filename, nil
}

func (s *Store) Upload(ctx context.Context, fileName, mimetype string, file io.Reader) (string, string, error) {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(fileName),
		Body:        file,
		ContentType: aws.String(mimetype),
	})
	if err != nil {
		return "", "", err
	}

	if s.publicURL != "" {
		// Path-style URL works for MinIO and most S3-compatible providers when o.UsePathStyle=true.
		return fmt.Sprintf("%s/%s/%s", s.publicURL, s.bucket, fileName), fileName, nil
	}

	// Fallback (AWS virtual-hosted-style). Works for AWS if the bucket is public and no custom endpoint is used.
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, fileName), fileName, nil
}
