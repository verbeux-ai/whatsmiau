package whatsmiau

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// Channels (newsletters, "…@newsletter") receive plaintext messages: media is
// uploaded unencrypted through its own endpoint and referenced by a media
// handle on the send request, and posts have no replies or mentions.

// ErrNewsletterUnsupported is returned when a message type cannot be posted
// to a channel. Callers should treat it as a client error.
var ErrNewsletterUnsupported = errors.New("not supported in channels")

// ErrNewsletterOnly is returned when a channel-only message type, such as a
// quiz or a question, is sent to another chat. Callers should treat it as a
// client error.
var ErrNewsletterOnly = errors.New("only supported in channels")

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

// requireNewsletter fails fast for channel-only message types sent elsewhere.
func requireNewsletter(to types.JID, kind string) error {
	if !isNewsletter(to) {
		return fmt.Errorf("%s: %w", kind, ErrNewsletterOnly)
	}
	return nil
}

// buildNewsletterPoll builds a channel poll the way the official app does: a
// PollCreationMessageV3 without a message secret (the server rejects
// whatsmeow's BuildPollCreation in channels with error 479). A correct answer
// turns it into a single-choice quiz, sent as PollCreationMessageV5.
func buildNewsletterPoll(name string, values []string, selectableCount int, correctAnswer string) (*waE2E.Message, error) {
	options := make([]*waE2E.PollCreationMessage_Option, len(values))
	for i, value := range values {
		options[i] = &waE2E.PollCreationMessage_Option{OptionName: proto.String(value)}
	}

	if correctAnswer == "" {
		if selectableCount < 0 || selectableCount > len(values) {
			selectableCount = 0
		}
		return &waE2E.Message{PollCreationMessageV3: &waE2E.PollCreationMessage{
			Name:                   proto.String(name),
			Options:                options,
			SelectableOptionsCount: proto.Uint32(uint32(selectableCount)),
		}}, nil
	}

	if !slices.Contains(values, correctAnswer) {
		return nil, fmt.Errorf("correct answer %q is not one of the values", correctAnswer)
	}
	return &waE2E.Message{PollCreationMessageV5: &waE2E.PollCreationMessage{
		Name:                   proto.String(name),
		Options:                options,
		SelectableOptionsCount: proto.Uint32(1),
		PollType:               waE2E.PollType_QUIZ.Enum(),
		CorrectAnswer:          &waE2E.PollCreationMessage_Option{OptionName: proto.String(correctAnswer)},
	}}, nil
}

// buildQuestionMessage builds a channel question, which followers answer
// privately, as the official app sends it.
func buildQuestionMessage(text string) *waE2E.Message {
	return &waE2E.Message{QuestionMessage: &waE2E.FutureProofMessage{
		Message: &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:        proto.String(text),
			ContextInfo: &waE2E.ContextInfo{IsQuestion: proto.Bool(true)},
		}},
	}}
}

// rejectNewsletter fails fast for message types channels do not accept, so
// the caller gets a clear error instead of an obscure server rejection.
func rejectNewsletter(to types.JID, kind string) error {
	if isNewsletter(to) {
		return fmt.Errorf("%s: %w", kind, ErrNewsletterUnsupported)
	}
	return nil
}

// TextMessageType is the messageType reported for a sent text: a channel
// question is a questionMessage, as the webhook reports it.
func TextMessageType(question bool) string {
	if question {
		return "questionMessage"
	}
	return "conversation"
}

// PollMessageType is the messageType reported for a sent poll. Channel polls
// are built as V3 polls and quizzes as V5 (see buildNewsletterPoll), and the
// webhook reports them with those types.
func PollMessageType(to types.JID, correctAnswer string) string {
	if !isNewsletter(to) {
		return "pollCreationMessage"
	}
	if correctAnswer != "" {
		return "pollCreationMessageV5"
	}
	return "pollCreationMessageV3"
}
