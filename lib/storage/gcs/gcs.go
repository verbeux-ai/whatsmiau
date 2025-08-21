package gcs

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

var _ interfaces.Storage = (*Gcs)(nil)

type Gcs struct {
	googleBucket *storage.BucketHandle
}

func New(bucket string) (*Gcs, error) {
	ctx, c := context.WithTimeout(context.Background(), time.Second*10)
	defer c()

	credentials, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, err
	}

	storageClient, err := storage.NewClient(ctx, option.WithCredentials(credentials))
	if err != nil {
		return nil, err
	}

	return &Gcs{
		googleBucket: storageClient.Bucket(bucket),
	}, nil

}

func (s *Gcs) UploadBase64(ctx context.Context, fileName, mimetype, b64 string) (string, error) {
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

func (s *Gcs) Upload(ctx context.Context, fileName, mimetype string, file io.Reader) (string, string, error) {
	obj := s.googleBucket.Object(fileName)
	writer := obj.NewWriter(ctx)
	writer.ContentType = mimetype
	writer.Name = fileName

	if _, err := io.Copy(writer, file); err != nil {
		return "", "", err
	}

	if err := writer.Close(); err != nil {
		return "", "", err
	}

	return fmt.Sprintf("%s/%s/%s",
		env.Env.GCSURL,
		obj.BucketName(),
		fileName,
	), fileName, nil
}
