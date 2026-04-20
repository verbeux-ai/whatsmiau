package whatsmiau

import (
	"errors"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

// ErrEmptyNumber is returned when FetchProfilePictureUrl is called with a
// blank number. Exported so the controller can map it to HTTP 400.
var ErrEmptyNumber = errors.New("number is empty")

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

type FetchProfilePictureUrlRequest struct {
	InstanceID string `json:"instance_id"`
	Number     string `json:"number"`
}

type FetchProfilePictureUrlResponse struct {
	Wuid              string `json:"wuid"`
	ProfilePictureURL string `json:"profilePictureUrl"`
}

// FetchProfilePictureUrl resolves a raw phone number to a WhatsApp JID and
// returns the full-resolution profile picture URL. Mirrors Evolution API's
// POST /chat/fetchProfilePictureUrl/{instance}.
func (s *Whatsmiau) FetchProfilePictureUrl(ctx context.Context, data *FetchProfilePictureUrlRequest) (*FetchProfilePictureUrlResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	number := strings.TrimSpace(data.Number)
	if number == "" {
		return nil, ErrEmptyNumber
	}

	// Accept already-JID inputs ("5511...@s.whatsapp.net", group JIDs, etc).
	var jid types.JID
	if strings.Contains(number, "@") {
		parsed, err := types.ParseJID(number)
		if err != nil {
			return nil, err
		}
		jid = parsed
	} else {
		digits := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, number)
		if digits == "" {
			return nil, ErrEmptyNumber
		}
		jid = types.NewJID(digits, types.DefaultUserServer)
		// Tries Brazilian 9th-digit fallback when applicable.
		jid = s.resolveJID(ctx, client, jid)
	}

	pic, err := client.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{
		Preview:     false,
		IsCommunity: false,
	})
	if err != nil {
		// whatsmeow returns ErrProfilePictureUnauthorized / Not404d via error.
		// Keep compat with Evolution: return empty URL instead of 500 when the
		// profile exists but the picture isn't visible.
		zap.L().Debug("FetchProfilePictureUrl: pic lookup failed",
			zap.String("instance", data.InstanceID),
			zap.String("jid", jid.String()),
			zap.Error(err))
	}

	resp := &FetchProfilePictureUrlResponse{Wuid: jid.String()}
	if pic != nil {
		resp.ProfilePictureURL = pic.URL
	}
	return resp, nil
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
