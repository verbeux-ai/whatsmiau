package lib

import (
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
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

	return client.MarkRead(data.MessageIDs, time.Now(), *data.RemoteJID, sender)
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

	return client.SendChatPresence(*data.RemoteJID, data.Presence, data.Media)
}

type NumberExistsRequest struct {
	InstanceID string   `json:"instance_id"`
	Numbers    []string `json:"numbers"`
}

type NumberExistsResponse map[string]Exists

type Exists struct {
	Exists bool   `json:"exists"`
	Jid    string `json:"jid"`
}

func (s *Whatsmiau) NumberExists(data *NumberExistsRequest) (NumberExistsResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	resp, err := client.IsOnWhatsApp(data.Numbers)
	if err != nil {
		return nil, err
	}

	resultsMap := make(map[string]Exists)
	for _, item := range resp {
		resultsMap[item.Query] = Exists{
			Exists: item.IsIn,
			Jid:    item.JID.String(),
		}
	}

	return resultsMap, nil
}
