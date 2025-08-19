package interfaces

import (
	"io"

	"golang.org/x/net/context"
)

type Storage interface {
	UploadBase64(ctx context.Context, fileName, mimetype, b64 string) (string, error)
	Upload(ctx context.Context, fileName, mimetype string, file io.Reader) (string, string, error)
}
