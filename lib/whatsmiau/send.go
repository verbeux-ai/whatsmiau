package whatsmiau

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type Quote struct {
	MessageID   string
	Message     string
	Participant *types.JID
}

type SendText struct {
	Text             string     `json:"text"`
	InstanceID       string     `json:"instance_id"`
	RemoteJID        *types.JID `json:"remote_jid"`
	Quote            *Quote     `json:"quote,omitempty"`
	MentionsEveryOne bool       `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string   `json:"mentioned,omitempty"`
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

	if data.RemoteJID == nil {
		return nil, fmt.Errorf("remote_jid is required")
	}

	resolved := s.resolveJID(ctx, client, *data.RemoteJID)
	data.RemoteJID = &resolved

	mentioned, everyone, err := s.resolveMentions(resolved, data.MentionsEveryOne, data.Mentioned)
	if err != nil {
		return nil, err
	}

	// mentionsEveryOne uses the classic E2E path with ContextInfo.NonJIDMentions=1
	// (same approach mautrix-whatsapp uses for @room). The message body must
	// contain the literal "@all" for the client to render the highlight.
	res, err := client.SendMessage(ctx, *data.RemoteJID, buildSendTextMessage(data, mentioned, everyone))
	if err != nil {
		return nil, err
	}

	return &SendTextResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}

func buildContextInfo(q *Quote, mentionedJID []string, everyone bool) *waE2E.ContextInfo {
	var ci *waE2E.ContextInfo
	if q != nil && len(q.MessageID) > 0 {
		ci = &waE2E.ContextInfo{
			StanzaID: proto.String(q.MessageID),
		}
		if q.Participant != nil {
			ci.Participant = proto.String(q.Participant.String())
		}
		if len(q.Message) > 0 {
			ci.QuotedMessage = &waE2E.Message{
				Conversation: proto.String(q.Message),
			}
		}
	}
	if everyone {
		if ci == nil {
			ci = &waE2E.ContextInfo{}
		}
		ci.NonJIDMentions = proto.Uint32(1)
	}
	if len(mentionedJID) > 0 {
		if ci == nil {
			ci = &waE2E.ContextInfo{}
		}
		ci.MentionedJID = mentionedJID
	}
	return ci
}

func buildSendTextMessage(data *SendText, mentionedJID []string, everyone bool) *waE2E.Message {
	ci := buildContextInfo(data.Quote, mentionedJID, everyone)
	if ci == nil {
		return &waE2E.Message{
			Conversation: proto.String(data.Text),
		}
	}

	return &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:        proto.String(data.Text),
			ContextInfo: ci,
		},
	}
}

type EditMessageKey struct {
	ID          string
	RemoteJID   string
	FromMe      bool
	Participant string
}

type UpdateMessage struct {
	Text       string          `json:"text"`
	InstanceID string          `json:"instance_id"`
	RemoteJID  *types.JID      `json:"remote_jid"`
	Key        *EditMessageKey `json:"key"`
}

type UpdateMessageResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) UpdateMessage(ctx context.Context, data *UpdateMessage) (*UpdateMessageResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}
	if client.Store == nil || client.Store.ID == nil {
		return nil, fmt.Errorf("device is not connected")
	}
	if data.RemoteJID == nil {
		return nil, fmt.Errorf("remote_jid is required")
	}
	if data.Key == nil || len(data.Key.ID) == 0 {
		return nil, fmt.Errorf("key.id is required")
	}

	chat := s.resolveJID(ctx, client, *data.RemoteJID)

	res, err := client.SendMessage(ctx, chat, client.BuildEdit(chat, types.MessageID(data.Key.ID), &waE2E.Message{
		Conversation: proto.String(data.Text),
	}))
	if err != nil {
		return nil, err
	}

	return &UpdateMessageResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}

// resolveMentions builds the mention metadata for a message. mentionsEveryOne
// only applies to group chats and uses the native "@everyone" mechanism
// (ContextInfo.NonJIDMentions = 1), the same approach mautrix-whatsapp uses
// for @room bridging. mentioned is a list of phone numbers (or full JIDs) to
// mention individually via ContextInfo.MentionedJID. The message text is left
// untouched — mentions are delivered via ContextInfo.
func (s *Whatsmiau) resolveMentions(remoteJID types.JID, mentionsEveryOne bool, mentioned []string) ([]string, bool, error) {
	var jids []string
	if len(mentioned) > 0 {
		jids = make([]string, 0, len(mentioned))
		for _, m := range mentioned {
			if !strings.Contains(m, "@") {
				m += "@" + types.DefaultUserServer
			}
			jid, err := types.ParseJID(m)
			if err != nil {
				return nil, false, fmt.Errorf("invalid mentioned jid %q: %w", m, err)
			}
			jids = append(jids, jid.String())
		}
		zap.L().Info("mentioned resolved", zap.String("remote", remoteJID.String()), zap.Strings("mentionedJid", jids))
	}

	// mentionsEveryOne only applies to group chats: it uses the native
	// "@everyone" mechanism (ContextInfo.NonJIDMentions = 1), the same approach
	// mautrix-whatsapp uses for @room bridging. For any other destination it is
	// ignored (there is no group to mention everyone in). mentioned is processed
	// independently and can be combined with mentionsEveryOne.
	everyone := mentionsEveryOne && remoteJID.Server == types.GroupServer
	if everyone {
		zap.L().Info("mentionsEveryOne resolved", zap.String("group", remoteJID.String()), zap.Bool("nonJidMentions", true))
	}
	return jids, everyone, nil
}

type SendAudioRequest struct {
	AudioURL         string     `json:"text"`
	InstanceID       string     `json:"instance_id"`
	RemoteJID        *types.JID `json:"remote_jid"`
	Quote            *Quote     `json:"quote,omitempty"`
	MentionsEveryOne bool       `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string   `json:"mentioned,omitempty"`
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

	if data.RemoteJID == nil {
		return nil, fmt.Errorf("remote_jid is required")
	}

	dataBytes, err := s.fetchBytes(ctx, data.AudioURL)
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
	}

	resolved := s.resolveJID(ctx, client, *data.RemoteJID)
	data.RemoteJID = &resolved

	mentioned, everyone, err := s.resolveMentions(resolved, data.MentionsEveryOne, data.Mentioned)
	if err != nil {
		return nil, err
	}
	audio.ContextInfo = buildContextInfo(data.Quote, mentioned, everyone)

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
	InstanceID       string     `json:"instance_id"`
	MediaURL         string     `json:"media_url"`
	Caption          string     `json:"caption"`
	FileName         string     `json:"file_name"`
	RemoteJID        *types.JID `json:"remote_jid"`
	Mimetype         string     `json:"mimetype"`
	Quote            *Quote     `json:"quote,omitempty"`
	MentionsEveryOne bool       `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string   `json:"mentioned,omitempty"`
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

	if data.RemoteJID == nil {
		return nil, fmt.Errorf("remote_jid is required")
	}

	dataBytes, err := s.fetchBytes(ctx, data.MediaURL)
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
	}

	resolved := s.resolveJID(ctx, client, *data.RemoteJID)
	data.RemoteJID = &resolved

	mentioned, everyone, err := s.resolveMentions(resolved, data.MentionsEveryOne, data.Mentioned)
	if err != nil {
		return nil, err
	}
	doc.ContextInfo = buildContextInfo(data.Quote, mentioned, everyone)

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
	InstanceID       string     `json:"instance_id"`
	MediaURL         string     `json:"media_url"`
	Caption          string     `json:"caption"`
	RemoteJID        *types.JID `json:"remote_jid"`
	Mimetype         string     `json:"mimetype"`
	Quote            *Quote     `json:"quote,omitempty"`
	MentionsEveryOne bool       `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string   `json:"mentioned,omitempty"`
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

	if data.RemoteJID == nil {
		return nil, fmt.Errorf("remote_jid is required")
	}

	dataBytes, err := s.fetchBytes(ctx, data.MediaURL)
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
	}

	resolved := s.resolveJID(ctx, client, *data.RemoteJID)
	data.RemoteJID = &resolved

	mentioned, everyone, err := s.resolveMentions(resolved, data.MentionsEveryOne, data.Mentioned)
	if err != nil {
		return nil, err
	}
	doc.ContextInfo = buildContextInfo(data.Quote, mentioned, everyone)

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

	if data.RemoteJID == nil {
		return nil, fmt.Errorf("remote_jid is required")
	}

	if len(data.MessageID) <= 0 {
		return nil, fmt.Errorf("invalid message_id")
	}

	if client.Store == nil || client.Store.ID == nil {
		return nil, fmt.Errorf("device is not connected")
	}

	resolved := s.resolveJID(ctx, client, *data.RemoteJID)
	data.RemoteJID = &resolved

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

// --- SendList ---

type SendListSection struct {
	Title string        `json:"title"`
	Rows  []SendListRow `json:"rows"`
}

type SendListRow struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	RowId       string `json:"row_id"`
}

type SendListRequest struct {
	InstanceID  string            `json:"instance_id"`
	RemoteJID   *types.JID        `json:"remote_jid"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	ButtonText  string            `json:"button_text"`
	FooterText  string            `json:"footer_text"`
	Sections    []SendListSection `json:"sections"`
}

type SendListResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) SendList(ctx context.Context, data *SendListRequest) (*SendListResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	if data.RemoteJID == nil {
		return nil, fmt.Errorf("remote_jid is required")
	}

	resolved := s.resolveJID(ctx, client, *data.RemoteJID)
	data.RemoteJID = &resolved

	// Build protobuf sections
	protoSections := make([]*waE2E.ListMessage_Section, 0, len(data.Sections))
	for _, sec := range data.Sections {
		rows := make([]*waE2E.ListMessage_Row, 0, len(sec.Rows))
		for _, row := range sec.Rows {
			rowTitle := strings.TrimSpace(row.Title)
			if rowTitle == "" {
				continue
			}
			rowID := strings.TrimSpace(row.RowId)
			if rowID == "" {
				rowID = rowTitle
			}
			protoRow := &waE2E.ListMessage_Row{
				RowID: proto.String(rowID),
				Title: proto.String(rowTitle),
			}
			if desc := strings.TrimSpace(row.Description); desc != "" {
				protoRow.Description = proto.String(desc)
			}
			rows = append(rows, protoRow)
		}
		if len(rows) == 0 {
			continue
		}
		section := &waE2E.ListMessage_Section{Rows: rows}
		if secTitle := strings.TrimSpace(sec.Title); secTitle != "" {
			section.Title = proto.String(secTitle)
		}
		protoSections = append(protoSections, section)
	}
	if len(protoSections) == 0 {
		return nil, fmt.Errorf("valid sections with rows are required")
	}

	buttonText := strings.TrimSpace(data.ButtonText)
	if buttonText == "" {
		buttonText = "Select"
	}

	listMsg := &waE2E.ListMessage{
		Description: proto.String(data.Description),
		ButtonText:  proto.String(buttonText),
		ListType:    waE2E.ListMessage_SINGLE_SELECT.Enum(),
		Sections:    protoSections,
	}
	if data.Title != "" {
		listMsg.Title = proto.String(data.Title)
	}
	if data.FooterText != "" {
		listMsg.FooterText = proto.String(data.FooterText)
	}

	// Wrap in FutureProofMessage (CRITICAL for rendering)
	message := &waE2E.Message{
		DocumentWithCaptionMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ListMessage: listMsg,
			},
		},
	}

	// Extra binary nodes for list
	extraNodes := []waBinary.Node{{
		Tag: "biz",
		Content: []waBinary.Node{{
			Tag: "list",
			Attrs: waBinary.Attrs{
				"type": "product_list",
				"v":    "2",
			},
		}},
	}}

	res, err := client.SendMessage(ctx, *data.RemoteJID, message, whatsmeow.SendRequestExtra{
		AdditionalNodes: &extraNodes,
	})
	if err != nil {
		return nil, err
	}

	return &SendListResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}

// --- SendButtons ---

type SendButtonItem struct {
	DisplayText string `json:"display_text"`
	Id          string `json:"id"`
}

type SendButtonsRequestData struct {
	InstanceID  string           `json:"instance_id"`
	RemoteJID   *types.JID       `json:"remote_jid"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Footer      string           `json:"footer"`
	Buttons     []SendButtonItem `json:"buttons"`
}

type SendButtonsResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) SendButtons(ctx context.Context, data *SendButtonsRequestData) (*SendButtonsResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	if data.RemoteJID == nil {
		return nil, fmt.Errorf("remote_jid is required")
	}

	resolved := s.resolveJID(ctx, client, *data.RemoteJID)
	data.RemoteJID = &resolved

	// Build buttons
	protoButtons := make([]*waE2E.ButtonsMessage_Button, 0, len(data.Buttons))
	for _, btn := range data.Buttons {
		title := strings.TrimSpace(btn.DisplayText)
		if title == "" {
			continue
		}
		buttonID := strings.TrimSpace(btn.Id)
		if buttonID == "" {
			buttonID = title
		}
		protoButtons = append(protoButtons, &waE2E.ButtonsMessage_Button{
			ButtonID: proto.String(buttonID),
			ButtonText: &waE2E.ButtonsMessage_Button_ButtonText{
				DisplayText: proto.String(title),
			},
			Type:           waE2E.ButtonsMessage_Button_RESPONSE.Enum(),
			NativeFlowInfo: &waE2E.ButtonsMessage_Button_NativeFlowInfo{},
		})
	}
	if len(protoButtons) == 0 {
		return nil, fmt.Errorf("valid buttons are required")
	}

	// Build ButtonsMessage
	buttonsMsg := &waE2E.ButtonsMessage{
		ContentText: proto.String(data.Description),
		HeaderType:  waE2E.ButtonsMessage_EMPTY.Enum(),
		Buttons:     protoButtons,
	}
	if data.Title != "" {
		buttonsMsg.HeaderType = waE2E.ButtonsMessage_TEXT.Enum()
		buttonsMsg.Header = &waE2E.ButtonsMessage_Text{Text: data.Title}
	}
	if data.Footer != "" {
		buttonsMsg.FooterText = proto.String(data.Footer)
	}

	// Wrap in FutureProofMessage (CRITICAL for rendering)
	message := &waE2E.Message{
		DocumentWithCaptionMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ButtonsMessage: buttonsMsg,
			},
		},
	}

	// Extra binary nodes for buttons
	extraNodes := []waBinary.Node{{
		Tag: "biz",
		Content: []waBinary.Node{{
			Tag: "interactive",
			Attrs: waBinary.Attrs{
				"type": "native_flow",
				"v":    "1",
			},
			Content: []waBinary.Node{{
				Tag: "native_flow",
				Attrs: waBinary.Attrs{
					"v":    "9",
					"name": "mixed",
				},
			}},
		}},
	}}

	res, err := client.SendMessage(ctx, *data.RemoteJID, message, whatsmeow.SendRequestExtra{
		AdditionalNodes: &extraNodes,
	})
	if err != nil {
		return nil, err
	}

	return &SendButtonsResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}

// --- SendPixPayment ---

type SendPixPaymentRequest struct {
	InstanceID   string     `json:"instance_id"`
	RemoteJID    *types.JID `json:"remote_jid"`
	PixKey       string     `json:"pix_key"`
	PixKeyType   string     `json:"pix_key_type"`
	MerchantName string     `json:"merchant_name"`
	DisplayText  string     `json:"display_text"`
	Currency     string     `json:"currency"`
}

type SendPixPaymentResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) SendPixPayment(ctx context.Context, data *SendPixPaymentRequest) (*SendPixPaymentResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	if data.RemoteJID == nil {
		return nil, fmt.Errorf("remote_jid is required")
	}

	resolved := s.resolveJID(ctx, client, *data.RemoteJID)
	data.RemoteJID = &resolved

	currency := data.Currency
	if currency == "" {
		currency = "BRL"
	}
	displayText := data.DisplayText
	if displayText == "" {
		displayText = "Pagar com PIX"
	}
	referenceID := fmt.Sprintf("PIX%d", time.Now().UnixMilli())

	// Map Evolution API keyType to WhatsApp protocol key_type
	keyType := strings.ToUpper(data.PixKeyType)
	if keyType == "RANDOM" {
		keyType = "EVP"
	}

	buttonParams := map[string]interface{}{
		"display_text": displayText,
		"currency":     currency,
		"total_amount": map[string]interface{}{
			"value":  0,
			"offset": 100,
		},
		"reference_id": referenceID,
		"type":         "physical-goods",
		"order": map[string]interface{}{
			"status": "pending",
			"subtotal": map[string]interface{}{
				"value":  0,
				"offset": 100,
			},
			"order_type": "ORDER",
			"items": []map[string]interface{}{
				{
					"retailer_id": "0",
					"product_id":  "0",
					"name":        displayText,
					"amount": map[string]interface{}{
						"value":  0,
						"offset": 100,
					},
					"quantity": 1,
				},
			},
		},
		"payment_settings": []map[string]interface{}{
			{
				"type": "pix_static_code",
				"pix_static_code": map[string]interface{}{
					"merchant_name": data.MerchantName,
					"key":           data.PixKey,
					"key_type":      keyType,
				},
			},
		},
	}

	buttonParamsJSON, err := json.Marshal(buttonParams)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}

	interactiveMsg := &waE2E.InteractiveMessage{
		InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
			NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
				MessageVersion: proto.Int32(1),
				Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
					{
						Name:             proto.String("payment_info"),
						ButtonParamsJSON: proto.String(string(buttonParamsJSON)),
					},
				},
			},
		},
	}

	// PIX: InteractiveMessage directly (NO FutureProofMessage wrapper)
	message := &waE2E.Message{InteractiveMessage: interactiveMsg}

	// Biz node: simple native_flow_name attribute (flat, 1 level)
	extraNodes := []waBinary.Node{{
		Tag: "biz",
		Attrs: waBinary.Attrs{
			"native_flow_name": "payment_info",
		},
	}}

	res, err := client.SendMessage(ctx, *data.RemoteJID, message, whatsmeow.SendRequestExtra{
		AdditionalNodes: &extraNodes,
	})
	if err != nil {
		return nil, err
	}

	return &SendPixPaymentResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}
