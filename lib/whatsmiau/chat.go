package whatsmiau

import (
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

	return client.MarkRead(context.TODO(), data.MessageIDs, time.Now(), *data.RemoteJID, sender)
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

	for _, item := range resp {
		if item.IsIn {
			resolved := jid
			resolved.User = item.JID.User
			if resolved.User != jid.User {
				zap.L().Debug("resolveJID: brazilian number resolved", zap.String("from", jid.User), zap.String("to", resolved.User))
			}
			return resolved
		}
	}

	return jid
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
	if req.RemoteJID == nil {
		return fmt.Errorf("remote_jid is required")
	}
	if req.MessageID == "" {
		return fmt.Errorf("message id is required")
	}

	chat := s.resolveJID(ctx, client, *req.RemoteJID)

	var sender types.JID
	if req.FromMe {
		sender = types.EmptyJID
	} else if chat.Server == types.GroupServer {
		if req.ParticipantJID == nil || req.ParticipantJID.IsEmpty() {
			return fmt.Errorf("participant is required when deleting another user's message in a group")
		}
		sender = s.resolveJID(ctx, client, *req.ParticipantJID)
	} else {
		sender = chat
	}

	msg := client.BuildRevoke(chat, sender, types.MessageID(req.MessageID))
	_, err := client.SendMessage(ctx, chat, msg)
	return err
}
