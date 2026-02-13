package whatsmiau

import (
	"context"
	"fmt"
	"io"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

type SendText struct {
	Text           string     `json:"text"`
	InstanceID     string     `json:"instance_id"`
	RemoteJID      *types.JID `json:"remote_jid"`
	QuoteMessageID string     `json:"quote_message_id"`
	QuoteMessage   string     `json:"quote_message"`
	Participant    string     `json:"participant"`
}

type SendTextResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) SendText(ctx context.Context, data *SendText) (*SendTextResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	contextInfo := buildQuotedContextInfo(data.QuoteMessageID, data.Participant)
	msg := &waE2E.Message{
		Conversation: &data.Text,
	}
	if contextInfo != nil {
		msg.Conversation = nil
		msg.ExtendedTextMessage = &waE2E.ExtendedTextMessage{
			Text:        proto.String(data.Text),
			ContextInfo: contextInfo,
		}
	}

	res, err := client.SendMessage(ctx, *data.RemoteJID, msg)
	if err != nil {
		return nil, err
	}

	return &SendTextResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}

type SendAudioRequest struct {
	AudioURL       string     `json:"text"`
	InstanceID     string     `json:"instance_id"`
	RemoteJID      *types.JID `json:"remote_jid"`
	QuoteMessageID string     `json:"quote_message_id"`
	QuoteMessage   string     `json:"quote_message"`
	Participant    string     `json:"participant"`
}

type SendAudioResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) SendAudio(ctx context.Context, data *SendAudioRequest) (*SendAudioResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	resAudio, err := s.getCtx(ctx, data.AudioURL)
	if err != nil {
		return nil, err
	}

	dataBytes, err := io.ReadAll(resAudio.Body)
	if err != nil {
		return nil, err
	}

	audioData, waveForm, secs, err := convertAudio(dataBytes, 64)
	if err != nil {
		return nil, err
	}

	uploaded, err := client.Upload(ctx, audioData, whatsmeow.MediaAudio)
	if err != nil {
		return nil, err
	}

	audio := waE2E.AudioMessage{
		URL:           proto.String(uploaded.URL),
		Mimetype:      proto.String("audio/ogg; codecs=opus"),
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uploaded.FileLength),
		Seconds:       proto.Uint32(uint32(secs)),
		PTT:           proto.Bool(true),
		MediaKey:      uploaded.MediaKey,
		FileEncSHA256: uploaded.FileEncSHA256,
		DirectPath:    proto.String(uploaded.DirectPath),
		Waveform:      waveForm,
		ContextInfo:   buildQuotedContextInfo(data.QuoteMessageID, data.Participant),
	}

	res, err := client.SendMessage(ctx, *data.RemoteJID, &waE2E.Message{
		AudioMessage: &audio,
	})
	if err != nil {
		return nil, err
	}

	return &SendAudioResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}

type SendDocumentRequest struct {
	InstanceID     string     `json:"instance_id"`
	MediaURL       string     `json:"media_url"`
	Caption        string     `json:"caption"`
	FileName       string     `json:"file_name"`
	RemoteJID      *types.JID `json:"remote_jid"`
	Mimetype       string     `json:"mimetype"`
	QuoteMessageID string     `json:"quote_message_id"`
	Participant    string     `json:"participant"`
}

type SendDocumentResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) SendDocument(ctx context.Context, data *SendDocumentRequest) (*SendDocumentResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	resMedia, err := s.getCtx(ctx, data.MediaURL)
	if err != nil {
		return nil, err
	}

	dataBytes, err := io.ReadAll(resMedia.Body)
	if err != nil {
		return nil, err
	}

	uploaded, err := client.Upload(ctx, dataBytes, whatsmeow.MediaDocument)
	if err != nil {
		return nil, err
	}

	doc := waE2E.DocumentMessage{
		URL:           proto.String(uploaded.URL),
		Mimetype:      proto.String(data.Mimetype),
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uploaded.FileLength),
		MediaKey:      uploaded.MediaKey,
		FileName:      &data.FileName,
		FileEncSHA256: uploaded.FileEncSHA256,
		DirectPath:    proto.String(uploaded.DirectPath),
		Caption:       proto.String(data.Caption),
		ContextInfo:   buildQuotedContextInfo(data.QuoteMessageID, data.Participant),
	}

	res, err := client.SendMessage(ctx, *data.RemoteJID, &waE2E.Message{
		DocumentMessage: &doc,
	})
	if err != nil {
		return nil, err
	}

	return &SendDocumentResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}

type SendImageRequest struct {
	InstanceID     string     `json:"instance_id"`
	MediaURL       string     `json:"media_url"`
	Caption        string     `json:"caption"`
	RemoteJID      *types.JID `json:"remote_jid"`
	Mimetype       string     `json:"mimetype"`
	QuoteMessageID string     `json:"quote_message_id"`
	Participant    string     `json:"participant"`
}
type SendImageResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) SendImage(ctx context.Context, data *SendImageRequest) (*SendImageResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	resMedia, err := s.getCtx(ctx, data.MediaURL)
	if err != nil {
		return nil, err
	}

	dataBytes, err := io.ReadAll(resMedia.Body)
	if err != nil {
		return nil, err
	}

	uploaded, err := client.Upload(ctx, dataBytes, whatsmeow.MediaImage)
	if err != nil {
		return nil, err
	}

	if data.Mimetype == "" {
		data.Mimetype, err = extractMimetype(dataBytes, uploaded.URL)
	}

	doc := waE2E.ImageMessage{
		URL:           proto.String(uploaded.URL),
		Mimetype:      proto.String(data.Mimetype),
		Caption:       proto.String(data.Caption),
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uploaded.FileLength),
		MediaKey:      uploaded.MediaKey,
		FileEncSHA256: uploaded.FileEncSHA256,
		DirectPath:    proto.String(uploaded.DirectPath),
		ContextInfo:   buildQuotedContextInfo(data.QuoteMessageID, data.Participant),
	}

	res, err := client.SendMessage(ctx, *data.RemoteJID, &waE2E.Message{
		ImageMessage: &doc,
	})
	if err != nil {
		return nil, err
	}

	return &SendImageResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}

func buildQuotedContextInfo(quoteMessageID string, participant string) *waE2E.ContextInfo {
	if quoteMessageID == "" {
		return nil
	}
	ctx := &waE2E.ContextInfo{
		StanzaID: proto.String(quoteMessageID),
	}
	if participant != "" {
		ctx.Participant = proto.String(participant)
	}
	return ctx
}

type SendReactionRequest struct {
	InstanceID string     `json:"instance_id"`
	Reaction   string     `json:"reaction"`
	RemoteJID  *types.JID `json:"remote_jid"`
	MessageID  string     `json:"message_id"`
	FromMe     bool       `json:"from_me"`
}

type SendReactionResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) SendReaction(ctx context.Context, data *SendReactionRequest) (*SendReactionResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	if len(data.Reaction) <= 0 {
		return nil, fmt.Errorf("empty reaction, len: %d", len(data.Reaction))
	}

	if len(data.MessageID) <= 0 {
		return nil, fmt.Errorf("invalid message_id")
	}

	if client.Store == nil || client.Store.ID == nil {
		return nil, fmt.Errorf("device is not connected")
	}

	sender := data.RemoteJID
	if data.FromMe {
		sender = client.Store.ID
	}

	doc := client.BuildReaction(*data.RemoteJID, *sender, data.MessageID, data.Reaction)
	res, err := client.SendMessage(ctx, *data.RemoteJID, doc)
	if err != nil {
		return nil, err
	}

	return &SendReactionResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}

type RevokeMessageRequest struct {
	InstanceID string     `json:"instance_id"`
	RemoteJID  *types.JID `json:"remote_jid"`
	MessageID  string     `json:"message_id"`
	FromMe     bool       `json:"from_me"`
}

type RevokeMessageResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) RevokeMessage(ctx context.Context, data *RevokeMessageRequest) (*RevokeMessageResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	if len(data.MessageID) <= 0 {
		return nil, fmt.Errorf("invalid message_id")
	}

	if client.Store == nil || client.Store.ID == nil {
		return nil, fmt.Errorf("device is not connected")
	}

	sender := data.RemoteJID
	if data.FromMe {
		sender = client.Store.ID
	}

	doc := client.BuildRevoke(*data.RemoteJID, *sender, data.MessageID)
	res, err := client.SendMessage(ctx, *data.RemoteJID, doc)
	if err != nil {
		return nil, err
	}

	return &RevokeMessageResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}
