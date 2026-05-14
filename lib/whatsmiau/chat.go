package whatsmiau

import (
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

const groupsCacheTTL = 5 * time.Minute

type groupsCacheEntry struct {
	data      []GroupInfo
	expiresAt time.Time
}

type GetGroupsRequest struct {
	InstanceID       string `json:"instance_id"`
	Refresh          bool   `json:"refresh"`
	WithParticipants bool   `json:"with_participants"`
	Page             int    `json:"page"`
	Limit            int    `json:"limit"`
}

type GroupParticipant struct {
	JID          string `json:"jid"`
	IsAdmin      bool   `json:"isAdmin"`
	IsSuperAdmin bool   `json:"isSuperAdmin"`
}

type GroupInfo struct {
	JID          string             `json:"jid"`
	Name         string             `json:"name"`
	Participants []GroupParticipant `json:"participants,omitempty"`
}

type GetGroupsResponse struct {
	Total  int         `json:"total"`
	Page   int         `json:"page"`
	Limit  int         `json:"limit"`
	Groups []GroupInfo `json:"groups"`
}

func (s *Whatsmiau) GetGroups(ctx context.Context, data *GetGroupsRequest) (*GetGroupsResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	var allGroups []GroupInfo

	if !data.Refresh {
		if entry, ok := s.groupsCache.Load(data.InstanceID); ok && time.Now().Before(entry.expiresAt) {
			allGroups = entry.data
		}
	}

	if allGroups == nil {
		groups, err := client.GetJoinedGroups(ctx)
		if err != nil {
			return nil, err
		}

		allGroups = make([]GroupInfo, 0, len(groups))
		for _, g := range groups {
			participants := make([]GroupParticipant, 0, len(g.Participants))
			for _, p := range g.Participants {
				participants = append(participants, GroupParticipant{
					JID:          p.JID.ToNonAD().String(),
					IsAdmin:      p.IsAdmin,
					IsSuperAdmin: p.IsSuperAdmin,
				})
			}
			allGroups = append(allGroups, GroupInfo{
				JID:          g.JID.String(),
				Name:         g.Name,
				Participants: participants,
			})
		}

		s.groupsCache.Store(data.InstanceID, groupsCacheEntry{
			data:      allGroups,
			expiresAt: time.Now().Add(groupsCacheTTL),
		})
	}

	total := len(allGroups)
	page := data.Page
	limit := data.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}

	start := (page - 1) * limit
	if start >= total {
		return &GetGroupsResponse{Total: total, Page: page, Limit: limit, Groups: []GroupInfo{}}, nil
	}
	end := start + limit
	if end > total {
		end = total
	}

	page_groups := allGroups[start:end]
	if !data.WithParticipants {
		stripped := make([]GroupInfo, len(page_groups))
		for i, g := range page_groups {
			stripped[i] = GroupInfo{JID: g.JID, Name: g.Name}
		}
		page_groups = stripped
	}

	return &GetGroupsResponse{
		Total:  total,
		Page:   page,
		Limit:  limit,
		Groups: page_groups,
	}, nil
}

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
