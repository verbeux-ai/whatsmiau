package whatsmiau

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// Sentinels so the HTTP layer can distinguish client-supplied input problems
// from genuine server/decryption failures and return the right status code.
var (
	ErrMediaFieldsMissing = errors.New("request does not contain a supported media message")
	ErrInstanceNotFound   = errors.New("instance not connected")
)

// DownloadableFields are the media fields required by the WhatsApp servers to
// locate and decrypt a previously uploaded file. The caller must provide the
// fields it received from the webhook (or equivalent source).
//
// Keys that hold binary values (mediaKey, fileEncSha256, fileSha256) may come
// as base64-encoded strings (webhook format) or already decoded bytes.
type DownloadableFields struct {
	URL               string `json:"url,omitempty"`
	DirectPath        string `json:"directPath,omitempty"`
	MediaKey          string `json:"mediaKey,omitempty"`      // base64
	FileEncSHA256     string `json:"fileEncSha256,omitempty"` // base64
	FileSHA256        string `json:"fileSha256,omitempty"`    // base64
	Mimetype          string `json:"mimetype,omitempty"`
	FileName          string `json:"fileName,omitempty"`
	FileLength        any    `json:"fileLength,omitempty"`        // may arrive as string or number
	MediaKeyTimestamp any    `json:"mediaKeyTimestamp,omitempty"` // may arrive as string or number
}

// DownloadMediaRequest describes a single message carrying one media field.
// Only one of the *Message fields must be non-nil.
type DownloadMediaRequest struct {
	InstanceID      string              `json:"instanceId"`
	ImageMessage    *DownloadableFields `json:"imageMessage,omitempty"`
	AudioMessage    *DownloadableFields `json:"audioMessage,omitempty"`
	VideoMessage    *DownloadableFields `json:"videoMessage,omitempty"`
	DocumentMessage *DownloadableFields `json:"documentMessage,omitempty"`
	StickerMessage  *DownloadableFields `json:"stickerMessage,omitempty"`
}

// DownloadMediaResponse matches Evolution API's getBase64FromMediaMessage shape
// so clients can drop the WhatsMiau implementation in without changes.
type DownloadMediaResponse struct {
	MessageType string `json:"messageType"`
	Mimetype    string `json:"mimetype,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	Base64      string `json:"base64"`
}

// DownloadMedia decrypts a media payload (image/audio/video/document/sticker)
// using the downloadable fields supplied by the caller and returns it as
// base64-encoded bytes.
//
// The caller typically has received these fields through the `messages.upsert`
// webhook and just wants to retrieve the actual file bytes later on. Requiring
// the caller to echo the fields back means WhatsMiau does not need to keep a
// large cache of historical messages just to satisfy download requests.
func (s *Whatsmiau) DownloadMedia(ctx context.Context, req *DownloadMediaRequest) (*DownloadMediaResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrInstanceNotFound, req.InstanceID)
	}

	msg, messageType, mimetype, fileName, err := buildDownloadableMessage(req)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, ErrMediaFieldsMissing
	}

	data, err := client.Download(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("whatsmeow download failed: %w", err)
	}

	return &DownloadMediaResponse{
		MessageType: messageType,
		Mimetype:    mimetype,
		FileName:    fileName,
		Base64:      base64.StdEncoding.EncodeToString(data),
	}, nil
}

// buildDownloadableMessage picks the first non-nil media field and returns a
// fully populated whatsmeow.DownloadableMessage plus metadata for the response.
func buildDownloadableMessage(req *DownloadMediaRequest) (whatsmeow.DownloadableMessage, string, string, string, error) {
	switch {
	case req.ImageMessage != nil:
		m, err := fillImage(req.ImageMessage)
		return m, "imageMessage", req.ImageMessage.Mimetype, "", err
	case req.StickerMessage != nil:
		m, err := fillSticker(req.StickerMessage)
		return m, "stickerMessage", req.StickerMessage.Mimetype, "", err
	case req.AudioMessage != nil:
		m, err := fillAudio(req.AudioMessage)
		return m, "audioMessage", req.AudioMessage.Mimetype, "", err
	case req.VideoMessage != nil:
		m, err := fillVideo(req.VideoMessage)
		return m, "videoMessage", req.VideoMessage.Mimetype, "", err
	case req.DocumentMessage != nil:
		m, err := fillDocument(req.DocumentMessage)
		return m, "documentMessage", req.DocumentMessage.Mimetype, req.DocumentMessage.FileName, err
	}
	return nil, "", "", "", nil
}

func fillImage(f *DownloadableFields) (*waProto.ImageMessage, error) {
	mediaKey, encHash, plainHash, err := decodeBinaryFields(f)
	if err != nil {
		return nil, err
	}
	return &waProto.ImageMessage{
		URL:               proto.String(f.URL),
		Mimetype:          strPtrOrNil(f.Mimetype),
		DirectPath:        proto.String(f.DirectPath),
		MediaKey:          mediaKey,
		FileEncSHA256:     encHash,
		FileSHA256:        plainHash,
		MediaKeyTimestamp: toInt64Ptr(f.MediaKeyTimestamp),
		FileLength:        toUint64Ptr(f.FileLength),
	}, nil
}

func fillAudio(f *DownloadableFields) (*waProto.AudioMessage, error) {
	mediaKey, encHash, plainHash, err := decodeBinaryFields(f)
	if err != nil {
		return nil, err
	}
	return &waProto.AudioMessage{
		URL:               proto.String(f.URL),
		Mimetype:          strPtrOrNil(f.Mimetype),
		DirectPath:        proto.String(f.DirectPath),
		MediaKey:          mediaKey,
		FileEncSHA256:     encHash,
		FileSHA256:        plainHash,
		MediaKeyTimestamp: toInt64Ptr(f.MediaKeyTimestamp),
		FileLength:        toUint64Ptr(f.FileLength),
	}, nil
}

func fillVideo(f *DownloadableFields) (*waProto.VideoMessage, error) {
	mediaKey, encHash, plainHash, err := decodeBinaryFields(f)
	if err != nil {
		return nil, err
	}
	return &waProto.VideoMessage{
		URL:               proto.String(f.URL),
		Mimetype:          strPtrOrNil(f.Mimetype),
		DirectPath:        proto.String(f.DirectPath),
		MediaKey:          mediaKey,
		FileEncSHA256:     encHash,
		FileSHA256:        plainHash,
		MediaKeyTimestamp: toInt64Ptr(f.MediaKeyTimestamp),
		FileLength:        toUint64Ptr(f.FileLength),
	}, nil
}

func fillDocument(f *DownloadableFields) (*waProto.DocumentMessage, error) {
	mediaKey, encHash, plainHash, err := decodeBinaryFields(f)
	if err != nil {
		return nil, err
	}
	return &waProto.DocumentMessage{
		URL:               proto.String(f.URL),
		Mimetype:          strPtrOrNil(f.Mimetype),
		FileName:          strPtrOrNil(f.FileName),
		DirectPath:        proto.String(f.DirectPath),
		MediaKey:          mediaKey,
		FileEncSHA256:     encHash,
		FileSHA256:        plainHash,
		MediaKeyTimestamp: toInt64Ptr(f.MediaKeyTimestamp),
		FileLength:        toUint64Ptr(f.FileLength),
	}, nil
}

func fillSticker(f *DownloadableFields) (*waProto.StickerMessage, error) {
	mediaKey, encHash, plainHash, err := decodeBinaryFields(f)
	if err != nil {
		return nil, err
	}
	return &waProto.StickerMessage{
		URL:               proto.String(f.URL),
		Mimetype:          strPtrOrNil(f.Mimetype),
		DirectPath:        proto.String(f.DirectPath),
		MediaKey:          mediaKey,
		FileEncSHA256:     encHash,
		FileSHA256:        plainHash,
		MediaKeyTimestamp: toInt64Ptr(f.MediaKeyTimestamp),
		FileLength:        toUint64Ptr(f.FileLength),
	}, nil
}

func decodeBinaryFields(f *DownloadableFields) ([]byte, []byte, []byte, error) {
	mediaKey, err := decodeBase64Field("mediaKey", f.MediaKey)
	if err != nil {
		return nil, nil, nil, err
	}
	encHash, err := decodeBase64Field("fileEncSha256", f.FileEncSHA256)
	if err != nil {
		return nil, nil, nil, err
	}
	plainHash, err := decodeBase64Field("fileSha256", f.FileSHA256)
	if err != nil {
		return nil, nil, nil, err
	}
	return mediaKey, encHash, plainHash, nil
}

// ErrInvalidBase64 is returned wrapped with the offending field name so the
// HTTP layer can surface a 400 response.
var ErrInvalidBase64 = errors.New("invalid base64")

func decodeBase64Field(name, v string) ([]byte, error) {
	if v == "" {
		return nil, nil
	}
	// Tolerate both standard and URL-safe base64 encodings, with or without
	// padding — consumers echo back whatever the webhook delivered.
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.RawURLEncoding,
	} {
		if data, err := enc.DecodeString(v); err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("%w for field %s", ErrInvalidBase64, name)
}

func strPtrOrNil(v string) *string {
	if v == "" {
		return nil
	}
	return proto.String(v)
}

func toInt64Ptr(v any) *int64 {
	switch x := v.(type) {
	case nil:
		return nil
	case float64:
		n := int64(x)
		return &n
	case int:
		n := int64(x)
		return &n
	case int64:
		return &x
	case string:
		if n, err := strconv.ParseInt(x, 10, 64); err == nil {
			return &n
		}
	}
	return nil
}

func toUint64Ptr(v any) *uint64 {
	switch x := v.(type) {
	case nil:
		return nil
	case float64:
		n := uint64(x)
		return &n
	case int:
		n := uint64(x)
		return &n
	case uint64:
		return &x
	case string:
		if n, err := strconv.ParseUint(x, 10, 64); err == nil {
			return &n
		}
	}
	return nil
}
