package whatsmiau

import (
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
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

type GroupParticipantInfo struct {
	JID          string `json:"jid"`
	PhoneNumber  string `json:"phoneNumber,omitempty"`
	LID          string `json:"lid,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	IsAdmin      bool   `json:"isAdmin"`
	IsSuperAdmin bool   `json:"isSuperAdmin"`
}

type GroupInfoResponse struct {
	GroupJid          string                 `json:"groupJid"`
	Subject           string                 `json:"subject"`
	OwnerJid          string                 `json:"ownerJid,omitempty"`
	OwnerPn           string                 `json:"ownerPn,omitempty"`
	Topic             string                 `json:"topic,omitempty"`
	TopicID           string                 `json:"topicId,omitempty"`
	IsAnnounce        bool                   `json:"isAnnounce"`
	IsLocked          bool                   `json:"isLocked"`
	IsEphemeral       bool                   `json:"isEphemeral"`
	DisappearingTimer uint32                 `json:"disappearingTimer"`
	MemberCount       int                    `json:"memberCount"`
	Participants      []GroupParticipantInfo `json:"participants,omitempty"`
	ProfilePicUrl     string                 `json:"profilePicUrl,omitempty"`
}

type GroupSummary struct {
	GroupJid      string `json:"groupJid"`
	Subject       string `json:"subject"`
	MemberCount   int    `json:"memberCount"`
	ProfilePicUrl string `json:"profilePicUrl,omitempty"`
}

func (s *Whatsmiau) GetAllContacts(ctx context.Context, instanceID string) ([]WookContact, error) {
	client, ok := s.clients.Load(instanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	contacts, err := client.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return nil, err
	}

	var result []WookContact
	for jid, info := range contacts {
		jidStr, lidStr := s.GetJidLid(ctx, instanceID, jid)

		name := info.PushName
		if name == "" {
			name = info.BusinessName
		}
		if name == "" {
			name = info.FullName
		}
		if name == "" {
			name = info.FirstName
		}

		result = append(result, WookContact{
			RemoteJid:  jidStr,
			RemoteLid:  lidStr,
			PushName:   name,
			InstanceId: instanceID,
		})
	}

	return result, nil
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

func (s *Whatsmiau) GetGroups(ctx context.Context, instanceID string) ([]GroupSummary, error) {
	client, ok := s.clients.Load(instanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	groups, err := client.GetJoinedGroups(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]GroupSummary, 0, len(groups))
	for _, g := range groups {
		if g == nil || g.JID.IsEmpty() || !strings.HasSuffix(g.JID.String(), "@g.us") {
			continue
		}
		result = append(result, GroupSummary{
			GroupJid:    g.JID.String(),
			Subject:     g.Name,
			MemberCount: g.ParticipantCount,
		})
	}

	return result, nil
}

func (s *Whatsmiau) GetGroupInfo(ctx context.Context, instanceID string, groupJid types.JID, includeParticipants bool) (*GroupInfoResponse, error) {
	client, ok := s.clients.Load(instanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	info, err := client.GetGroupInfo(ctx, groupJid)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	picURL := ""
	if pic, err := client.GetProfilePictureInfo(ctx, info.JID, &whatsmeow.GetProfilePictureParams{
		Preview:     false,
		IsCommunity: false,
	}); err == nil && pic != nil {
		picURL = pic.URL
	}

	var participants []GroupParticipantInfo
	if includeParticipants {
		participants = make([]GroupParticipantInfo, 0, len(info.Participants))
		for _, p := range info.Participants {
			participants = append(participants, GroupParticipantInfo{
				JID:          p.JID.String(),
				PhoneNumber:  p.PhoneNumber.String(),
				LID:          p.LID.String(),
				DisplayName:  p.DisplayName,
				IsAdmin:      p.IsAdmin,
				IsSuperAdmin: p.IsSuperAdmin,
			})
		}
	}

	return &GroupInfoResponse{
		GroupJid:          info.JID.String(),
		Subject:           info.Name,
		OwnerJid:          info.OwnerJID.String(),
		OwnerPn:           info.OwnerPN.String(),
		Topic:             info.Topic,
		TopicID:           info.TopicID,
		IsAnnounce:        info.IsAnnounce,
		IsLocked:          info.IsLocked,
		IsEphemeral:       info.IsEphemeral,
		DisappearingTimer: info.DisappearingTimer,
		MemberCount:       info.ParticipantCount,
		Participants:      participants,
		ProfilePicUrl:     picURL,
	}, nil
}
