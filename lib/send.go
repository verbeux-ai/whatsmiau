package lib

import (
	"context"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

type SendText struct {
	Text       string    `json:"text"`
	InstanceID string    `json:"instance_id"`
	RemoteJID  types.JID `json:"remote_jid"`
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

	res, err := client.SendMessage(ctx, data.RemoteJID, &waE2E.Message{
		Conversation: &data.Text,
	})
	if err != nil {
		return nil, err
	}

	return &SendTextResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}
