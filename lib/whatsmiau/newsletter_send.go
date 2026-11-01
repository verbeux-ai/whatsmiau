package whatsmiau

import (
	"context"
	"errors"
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
)

// Channels (newsletters, "…@newsletter") receive plaintext messages: media is
// uploaded unencrypted through its own endpoint and referenced by a media
// handle on the send request, and posts have no replies or mentions.

// ErrNewsletterUnsupported is returned when a message type cannot be posted
// to a channel. Callers should treat it as a client error.
var ErrNewsletterUnsupported = errors.New("not supported in channels")

func isNewsletter(jid types.JID) bool {
	return jid.Server == types.NewsletterServer
}

// mediaUploader is the subset of *whatsmeow.Client used to upload media.
type mediaUploader interface {
	Upload(ctx context.Context, plaintext []byte, appInfo whatsmeow.MediaType) (whatsmeow.UploadResponse, error)
	UploadNewsletter(ctx context.Context, data []byte, appInfo whatsmeow.MediaType) (whatsmeow.UploadResponse, error)
}

// uploadMedia uploads data for the given chat. Channel media is not encrypted:
// its response has no MediaKey or FileEncSHA256, so messages built from it
// leave those fields unset, as whatsmeow expects for newsletters.
func uploadMedia(ctx context.Context, client mediaUploader, to types.JID, data []byte, mediaType whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
	if !isNewsletter(to) {
		return client.Upload(ctx, data, mediaType)
	}
	uploaded, err := client.UploadNewsletter(ctx, data, mediaType)
	if err != nil {
		return uploaded, err
	}
	if uploaded.Handle == "" {
		return uploaded, errors.New("channel upload returned no media handle")
	}
	return uploaded, nil
}

// mediaSendExtra returns the SendMessage options for media uploaded with
// uploadMedia: channels need the upload handle (sent as media_id), other chats
// need nothing.
func mediaSendExtra(to types.JID, uploaded whatsmeow.UploadResponse) []whatsmeow.SendRequestExtra {
	if !isNewsletter(to) {
		return nil
	}
	return []whatsmeow.SendRequestExtra{{MediaHandle: uploaded.Handle}}
}

// mediaContextInfo marks channel media as unencrypted, as the official clients
// do; without it, receivers try to decrypt the file and show a broken
// placeholder. Other chats keep ci unchanged.
func mediaContextInfo(to types.JID, ci *waE2E.ContextInfo) *waE2E.ContextInfo {
	if !isNewsletter(to) {
		return ci
	}
	if ci == nil {
		ci = &waE2E.ContextInfo{}
	}
	ci.MediaDomainInfo = &waE2E.MediaDomainInfo{
		MediaKeyDomain: waE2E.MediaKeyDomain_MEDIA_KEY_DOMAIN_NON_E2EE.Enum(),
	}
	return ci
}

// quoteForChat drops the quote when posting to a channel, which has no replies.
func quoteForChat(to types.JID, q *Quote) *Quote {
	if q != nil && isNewsletter(to) {
		zap.L().Debug("quote ignored for channel", zap.Stringer("remote", to))
		return nil
	}
	return q
}

// rejectNewsletter fails fast for message types channels do not accept, so
// the caller gets a clear error instead of an obscure server rejection.
func rejectNewsletter(to types.JID, kind string) error {
	if isNewsletter(to) {
		return fmt.Errorf("%s: %w", kind, ErrNewsletterUnsupported)
	}
	return nil
}
