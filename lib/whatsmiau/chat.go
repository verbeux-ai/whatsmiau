package whatsmiau

import (
	"errors"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type ReadMessageRequest struct {
	MessageIDs []string   `json:"message_ids"`
	InstanceID string     `json:"instance_id"`
	RemoteJID  *types.JID `json:"remote_jid"`
	Sender     *types.JID `json:"sender"`
	// Played sends a "played" receipt instead of a "read" one: the blue
	// microphone on a voice note, as opposed to the blue ticks on the chat.
	// WhatsApp treats them as two different receipts, and marking an audio
	// message as read never turns its microphone blue.
	//
	// Only meaningful for audio/PTT messages; WhatsApp ignores it elsewhere.
	Played bool `json:"played"`
}

func (s *Whatsmiau) ReadMessage(data *ReadMessageRequest) error {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}

	sender := *data.RemoteJID
	if data.Sender != nil {
		sender = *data.Sender
	}

	// MarkRead defaults to types.ReceiptTypeRead and only overrides it from
	// this variadic, so "played" is unreachable unless it is passed here.
	var receiptType []types.ReceiptType
	if data.Played {
		receiptType = append(receiptType, types.ReceiptTypePlayed)
	}

	return client.MarkRead(context.TODO(), data.MessageIDs, time.Now(), *data.RemoteJID, sender, receiptType...)
}

type ChatPresenceRequest struct {
	InstanceID string                  `json:"instance_id"`
	RemoteJID  *types.JID              `json:"remote_jid"`
	Presence   types.ChatPresence      `json:"presence"`
	Media      types.ChatPresenceMedia `json:"media"`
}

func (s *Whatsmiau) ChatPresence(data *ChatPresenceRequest) error {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}

	return client.SendChatPresence(context.TODO(), *data.RemoteJID, data.Presence, data.Media)
}

type NumberExistsRequest struct {
	InstanceID string   `json:"instance_id"`
	Numbers    []string `json:"numbers"`
}

type NumberExistsResponse []Exists

type Exists struct {
	Exists bool   `json:"exists"`
	Jid    string `json:"jid"`
	Lid    string `json:"lid"`
	Number string `json:"number"`
}

func (s *Whatsmiau) NumberExists(ctx context.Context, data *NumberExistsRequest) (NumberExistsResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	resp, err := client.IsOnWhatsApp(context.TODO(), data.Numbers)
	if err != nil {
		return nil, err
	}

	var results []Exists
	for _, item := range resp {
		jid, lid := s.GetJidLid(ctx, data.InstanceID, item.JID)

		results = append(results, Exists{
			Exists: item.IsIn,
			Jid:    jid,
			Lid:    lid,
			Number: item.Query,
		})
	}

	return results, nil
}

func (s *Whatsmiau) resolveJID(ctx context.Context, client *whatsmeow.Client, jid types.JID) types.JID {
	if jid.Server != types.DefaultUserServer {
		return jid
	}

	alternate := buildBrazilianAlternate(jid.User)
	if alternate == "" {
		return jid
	}

	resp, err := client.IsOnWhatsApp(ctx, []string{jid.User, alternate})
	if err != nil {
		zap.L().Warn("resolveJID: failed to check number on WhatsApp", zap.String("number", jid.User), zap.Error(err))
		return jid
	}

	resolved := resolveExistingJID(jid, resp)
	if resolved != jid {
		zap.L().Debug(
			"resolveJID: contact JID resolved",
			zap.Stringer("from", jid),
			zap.Stringer("to", resolved),
		)
	}

	return resolved
}

func resolveExistingJID(fallback types.JID, resp []types.IsOnWhatsAppResponse) types.JID {
	for _, item := range resp {
		if item.IsIn && !item.JID.IsEmpty() {
			// Since whatsmeow v1.3, the canonical JID may use @lid. Preserve
			// the full JID: copying only User would reinterpret a LID as a PN.
			return item.JID.ToNonAD()
		}
	}

	return fallback
}

type DeleteMessageForEveryoneRequest struct {
	InstanceID     string     `json:"instance_id"`
	RemoteJID      *types.JID `json:"remote_jid"`
	MessageID      string     `json:"message_id"`
	FromMe         bool       `json:"from_me"`
	ParticipantJID *types.JID `json:"participant_jid,omitempty"`
}

func (s *Whatsmiau) DeleteMessageForEveryone(ctx context.Context, req *DeleteMessageForEveryoneRequest) error {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}
	if client.Store == nil || client.Store.ID == nil {
		return fmt.Errorf("device is not connected")
	}

	chat := s.resolveJID(ctx, client, *req.RemoteJID)

	var sender types.JID
	if req.FromMe {
		if chat.Server == types.GroupServer {
			sender = client.Store.ID.ToNonAD()
		} else {
			sender = types.EmptyJID
		}
	} else if chat.Server == types.GroupServer {
		sender = s.resolveJID(ctx, client, *req.ParticipantJID)
	} else {
		sender = chat
	}

	msg := client.BuildRevoke(chat, sender, types.MessageID(req.MessageID))
	_, err := client.SendMessage(ctx, chat, msg)
	return err
}

// FetchProfilePictureURL returns the full-size ('image') profile picture URL
// for a user or group JID, mirroring Evolution API's POST /chat/fetchProfilePictureUrl.
// It returns ("", nil) when the target has no picture or hid its picture; the
// caller maps that to a null field, keeping the contract Evolution-compatible.
func (s *Whatsmiau) FetchProfilePictureURL(ctx context.Context, instanceID string, jid types.JID) (string, error) {
	client, ok := s.clients.Load(instanceID)
	if !ok {
		return "", whatsmeow.ErrClientIsNil
	}
	if !client.IsConnected() {
		return "", fmt.Errorf("client not connected")
	}

	pic, err := client.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{Preview: false})
	if errors.Is(err, whatsmeow.ErrProfilePictureNotSet) || errors.Is(err, whatsmeow.ErrProfilePictureUnauthorized) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if pic == nil || pic.URL == "" {
		return "", nil
	}

	return pic.URL, nil
}
