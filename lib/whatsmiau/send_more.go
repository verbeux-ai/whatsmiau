package whatsmiau

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// --- SendVideo ---

type SendVideoRequest struct {
	InstanceID  string     `json:"instance_id"`
	MediaURL    string     `json:"media_url"`
	Caption     string     `json:"caption"`
	RemoteJID   *types.JID `json:"remote_jid"`
	Mimetype    string     `json:"mimetype"`
	GifPlayback bool       `json:"gif_playback"`
}

type SendVideoResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) SendVideo(ctx context.Context, data *SendVideoRequest) (*SendVideoResponse, error) {
	client, resolved, err := s.loadClientWithJID(ctx, data.InstanceID, data.RemoteJID)
	if err != nil {
		return nil, err
	}
	data.RemoteJID = &resolved

	dataBytes, err := s.fetchBytes(ctx, data.MediaURL)
	if err != nil {
		return nil, err
	}

	uploaded, err := client.Upload(ctx, dataBytes, whatsmeow.MediaVideo)
	if err != nil {
		return nil, err
	}

	if data.Mimetype == "" {
		data.Mimetype = "video/mp4"
	}

	video := waE2E.VideoMessage{
		URL:           proto.String(uploaded.URL),
		Mimetype:      proto.String(data.Mimetype),
		Caption:       proto.String(data.Caption),
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uploaded.FileLength),
		MediaKey:      uploaded.MediaKey,
		FileEncSHA256: uploaded.FileEncSHA256,
		DirectPath:    proto.String(uploaded.DirectPath),
	}
	if data.GifPlayback {
		video.GifPlayback = proto.Bool(true)
	}

	res, err := client.SendMessage(ctx, resolved, &waE2E.Message{VideoMessage: &video})
	if err != nil {
		return nil, err
	}

	return &SendVideoResponse{ID: res.ID, CreatedAt: res.Timestamp}, nil
}

// --- SendPtv (Round/Note Video) ---

type SendPtvRequest struct {
	InstanceID string     `json:"instance_id"`
	VideoURL   string     `json:"video_url"`
	RemoteJID  *types.JID `json:"remote_jid"`
}

type SendPtvResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) SendPtv(ctx context.Context, data *SendPtvRequest) (*SendPtvResponse, error) {
	client, resolved, err := s.loadClientWithJID(ctx, data.InstanceID, data.RemoteJID)
	if err != nil {
		return nil, err
	}
	data.RemoteJID = &resolved

	dataBytes, err := s.fetchBytes(ctx, data.VideoURL)
	if err != nil {
		return nil, err
	}

	uploaded, err := client.Upload(ctx, dataBytes, whatsmeow.MediaVideo)
	if err != nil {
		return nil, err
	}

	video := waE2E.VideoMessage{
		URL:             proto.String(uploaded.URL),
		Mimetype:        proto.String("video/mp4"),
		FileSHA256:      uploaded.FileSHA256,
		FileLength:      proto.Uint64(uploaded.FileLength),
		MediaKey:        uploaded.MediaKey,
		FileEncSHA256:   uploaded.FileEncSHA256,
		DirectPath:      proto.String(uploaded.DirectPath),
		VideoSourceType: waE2E.VideoMessage_USER_VIDEO.Enum(),
	}

	res, err := client.SendMessage(ctx, resolved, &waE2E.Message{PtvMessage: &video})
	if err != nil {
		return nil, err
	}

	return &SendPtvResponse{ID: res.ID, CreatedAt: res.Timestamp}, nil
}

// --- SendSticker ---

type SendStickerRequest struct {
	InstanceID string     `json:"instance_id"`
	StickerURL string     `json:"sticker_url"`
	RemoteJID  *types.JID `json:"remote_jid"`
}

type SendStickerResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) SendSticker(ctx context.Context, data *SendStickerRequest) (*SendStickerResponse, error) {
	client, resolved, err := s.loadClientWithJID(ctx, data.InstanceID, data.RemoteJID)
	if err != nil {
		return nil, err
	}
	data.RemoteJID = &resolved

	dataBytes, err := s.fetchBytes(ctx, data.StickerURL)
	if err != nil {
		return nil, err
	}

	uploaded, err := client.Upload(ctx, dataBytes, whatsmeow.MediaImage)
	if err != nil {
		return nil, err
	}

	sticker := waE2E.StickerMessage{
		URL:           proto.String(uploaded.URL),
		Mimetype:      proto.String("image/webp"),
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uploaded.FileLength),
		MediaKey:      uploaded.MediaKey,
		FileEncSHA256: uploaded.FileEncSHA256,
		DirectPath:    proto.String(uploaded.DirectPath),
	}

	res, err := client.SendMessage(ctx, resolved, &waE2E.Message{StickerMessage: &sticker})
	if err != nil {
		return nil, err
	}

	return &SendStickerResponse{ID: res.ID, CreatedAt: res.Timestamp}, nil
}

// --- SendLocation ---

type SendLocationRequest struct {
	InstanceID string     `json:"instance_id"`
	RemoteJID  *types.JID `json:"remote_jid"`
	Latitude   float64    `json:"latitude"`
	Longitude  float64    `json:"longitude"`
	Name       string     `json:"name"`
	Address    string     `json:"address"`
}

type SendLocationResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) SendLocation(ctx context.Context, data *SendLocationRequest) (*SendLocationResponse, error) {
	client, resolved, err := s.loadClientWithJID(ctx, data.InstanceID, data.RemoteJID)
	if err != nil {
		return nil, err
	}
	data.RemoteJID = &resolved

	loc := &waE2E.LocationMessage{
		DegreesLatitude:  proto.Float64(data.Latitude),
		DegreesLongitude: proto.Float64(data.Longitude),
	}
	if data.Name != "" {
		loc.Name = proto.String(data.Name)
	}
	if data.Address != "" {
		loc.Address = proto.String(data.Address)
	}

	res, err := client.SendMessage(ctx, resolved, &waE2E.Message{LocationMessage: loc})
	if err != nil {
		return nil, err
	}

	return &SendLocationResponse{ID: res.ID, CreatedAt: res.Timestamp}, nil
}

// --- SendContact ---

type SendContactItem struct {
	FullName     string `json:"full_name"`
	Wuid         string `json:"wuid"`
	PhoneNumber  string `json:"phone_number"`
	Organization string `json:"organization"`
	Email        string `json:"email"`
	URL          string `json:"url"`
}

type SendContactRequest struct {
	InstanceID string            `json:"instance_id"`
	RemoteJID  *types.JID        `json:"remote_jid"`
	Contacts   []SendContactItem `json:"contacts"`
}

type SendContactResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func contactToProto(c SendContactItem) *waE2E.ContactMessage {
	return &waE2E.ContactMessage{
		DisplayName: proto.String(c.FullName),
		Vcard:       proto.String(buildVCard(c.FullName, c.Wuid, c.PhoneNumber, c.Organization, c.Email, c.URL)),
	}
}

func (s *Whatsmiau) SendContact(ctx context.Context, data *SendContactRequest) (*SendContactResponse, error) {
	if len(data.Contacts) == 0 {
		return nil, fmt.Errorf("contacts is required")
	}

	client, resolved, err := s.loadClientWithJID(ctx, data.InstanceID, data.RemoteJID)
	if err != nil {
		return nil, err
	}
	data.RemoteJID = &resolved

	var msg *waE2E.Message
	if len(data.Contacts) == 1 {
		msg = &waE2E.Message{ContactMessage: contactToProto(data.Contacts[0])}
	} else {
		contacts := make([]*waE2E.ContactMessage, 0, len(data.Contacts))
		for _, c := range data.Contacts {
			contacts = append(contacts, contactToProto(c))
		}
		msg = &waE2E.Message{
			ContactsArrayMessage: &waE2E.ContactsArrayMessage{
				DisplayName: proto.String(data.Contacts[0].FullName),
				Contacts:    contacts,
			},
		}
	}

	res, err := client.SendMessage(ctx, resolved, msg)
	if err != nil {
		return nil, err
	}

	return &SendContactResponse{ID: res.ID, CreatedAt: res.Timestamp}, nil
}

// --- SendPoll ---

type SendPollRequest struct {
	InstanceID      string     `json:"instance_id"`
	RemoteJID       *types.JID `json:"remote_jid"`
	Name            string     `json:"name"`
	SelectableCount int        `json:"selectable_count"`
	Values          []string   `json:"values"`
}

type SendPollResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) SendPoll(ctx context.Context, data *SendPollRequest) (*SendPollResponse, error) {
	if len(data.Values) < 2 {
		return nil, fmt.Errorf("values requires at least 2 options")
	}

	client, resolved, err := s.loadClientWithJID(ctx, data.InstanceID, data.RemoteJID)
	if err != nil {
		return nil, err
	}
	data.RemoteJID = &resolved

	pollMsg := client.BuildPollCreation(data.Name, data.Values, data.SelectableCount)

	res, err := client.SendMessage(ctx, resolved, pollMsg)
	if err != nil {
		return nil, err
	}

	return &SendPollResponse{ID: res.ID, CreatedAt: res.Timestamp}, nil
}

// --- SendStatus ---

type SendStatusRequest struct {
	InstanceID      string   `json:"instance_id"`
	Type            string   `json:"type"` // text | image | audio | video
	Content         string   `json:"content"`
	Caption         string   `json:"caption"`
	BackgroundColor string   `json:"background_color"`
	Font            int      `json:"font"`
	StatusJidList   []string `json:"status_jid_list"`
	AllContacts     bool     `json:"all_contacts"`
}

type SendStatusResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func statusFontFromInt(font int) waE2E.ExtendedTextMessage_FontType {
	switch font {
	case 1:
		return waE2E.ExtendedTextMessage_SYSTEM_TEXT
	case 2:
		return waE2E.ExtendedTextMessage_FB_SCRIPT
	case 3:
		return waE2E.ExtendedTextMessage_SYSTEM_BOLD
	case 4:
		return waE2E.ExtendedTextMessage_MORNINGBREEZE_REGULAR
	case 5:
		return waE2E.ExtendedTextMessage_CALISTOGA_REGULAR
	default:
		return waE2E.ExtendedTextMessage_SYSTEM
	}
}

func buildTextStatus(data *SendStatusRequest) *waE2E.Message {
	return &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:           proto.String(data.Content),
			BackgroundArgb: proto.Uint32(parseStatusBackgroundARGB(data.BackgroundColor)),
			Font:           statusFontFromInt(data.Font).Enum(),
		},
	}
}

func (s *Whatsmiau) buildImageStatus(ctx context.Context, client *whatsmeow.Client, data *SendStatusRequest) (*waE2E.Message, error) {
	bytesData, err := s.fetchBytes(ctx, data.Content)
	if err != nil {
		return nil, err
	}
	uploaded, err := client.Upload(ctx, bytesData, whatsmeow.MediaImage)
	if err != nil {
		return nil, err
	}
	mimetype, _ := extractMimetype(bytesData, uploaded.URL)
	if mimetype == "" {
		mimetype = "image/jpeg"
	}
	return &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			URL:           proto.String(uploaded.URL),
			Mimetype:      proto.String(mimetype),
			Caption:       proto.String(data.Caption),
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			DirectPath:    proto.String(uploaded.DirectPath),
		},
	}, nil
}

func (s *Whatsmiau) buildVideoStatus(ctx context.Context, client *whatsmeow.Client, data *SendStatusRequest) (*waE2E.Message, error) {
	bytesData, err := s.fetchBytes(ctx, data.Content)
	if err != nil {
		return nil, err
	}
	uploaded, err := client.Upload(ctx, bytesData, whatsmeow.MediaVideo)
	if err != nil {
		return nil, err
	}
	return &waE2E.Message{
		VideoMessage: &waE2E.VideoMessage{
			URL:           proto.String(uploaded.URL),
			Mimetype:      proto.String("video/mp4"),
			Caption:       proto.String(data.Caption),
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			DirectPath:    proto.String(uploaded.DirectPath),
		},
	}, nil
}

func (s *Whatsmiau) buildAudioStatus(ctx context.Context, client *whatsmeow.Client, data *SendStatusRequest) (*waE2E.Message, error) {
	bytesData, err := s.fetchBytes(ctx, data.Content)
	if err != nil {
		return nil, err
	}
	audioData, waveForm, secs, err := convertAudio(bytesData, 64)
	if err != nil {
		return nil, err
	}
	uploaded, err := client.Upload(ctx, audioData, whatsmeow.MediaAudio)
	if err != nil {
		return nil, err
	}
	return &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
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
		},
	}, nil
}

func (s *Whatsmiau) SendStatus(ctx context.Context, data *SendStatusRequest) (*SendStatusResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	var msg *waE2E.Message
	var err error
	switch strings.ToLower(data.Type) {
	case "text":
		msg = buildTextStatus(data)
	case "image":
		msg, err = s.buildImageStatus(ctx, client, data)
	case "video":
		msg, err = s.buildVideoStatus(ctx, client, data)
	case "audio":
		msg, err = s.buildAudioStatus(ctx, client, data)
	default:
		return nil, fmt.Errorf("invalid status type: %s", data.Type)
	}
	if err != nil {
		return nil, err
	}

	res, err := client.SendMessage(ctx, types.StatusBroadcastJID, msg)
	if err != nil {
		return nil, err
	}

	return &SendStatusResponse{ID: res.ID, CreatedAt: res.Timestamp}, nil
}
