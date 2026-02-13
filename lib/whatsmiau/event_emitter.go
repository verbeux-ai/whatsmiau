package whatsmiau

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-vcard"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/models"
	"github.com/verbeux-ai/whatsmiau/repositories/mongocontacts"
	"github.com/verbeux-ai/whatsmiau/repositories/mongomessages"
	"github.com/verbeux-ai/whatsmiau/services"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/proto/waWeb"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/protobuf/encoding/protojson"
)

type emitter struct {
	url  string
	data any
}

func (s *Whatsmiau) getInstance(id string) *models.Instance {
	ctx, c := context.WithTimeout(context.Background(), time.Second*5)
	defer c()

	res, err := s.repo.List(ctx, id)
	if err != nil {
		zap.L().Panic("failed to get instanceCached by instance", zap.Error(err))
	}

	if len(res) == 0 {
		zap.L().Warn("no instanceCached found by instance", zap.String("instance", id))
		return nil
	}

	s.instanceCache.Store(id, res[0])

	return &res[0]
}

func (s *Whatsmiau) getInstanceCached(id string) *models.Instance {
	instanceCached, ok := s.instanceCache.Load(id)
	if ok {
		return &instanceCached
	}

	ctx, c := context.WithTimeout(context.Background(), time.Second*5)
	defer c()

	res, err := s.repo.List(ctx, id)
	if err != nil {
		zap.L().Panic("failed to get instanceCached by instance", zap.Error(err))
	}

	if len(res) == 0 {
		zap.L().Warn("no instanceCached found by instance", zap.String("instance", id))
		return nil
	}

	s.instanceCache.Store(id, res[0])
	go func() {
		// expires in 10sec
		time.Sleep(time.Second * 10)
		s.instanceCache.Delete(id)
	}()

	return &res[0]
}

func (s *Whatsmiau) startEmitter() {
	for event := range s.emitter {
		data, err := json.Marshal(event.data)
		if err != nil {
			zap.L().Error("failed to marshal event", zap.Error(err))
			return
		}

		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, event.url, bytes.NewReader(data))
		if err != nil {
			zap.L().Error("failed to create request", zap.Error(err))
			return
		}

		req.Header.Set("Content-Type", "application/json")
		resp, err := s.httpClient.Do(req)
		if err != nil {
			zap.L().Error("failed to send request", zap.Error(err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			res, err := io.ReadAll(resp.Body)
			if err != nil {
				zap.L().Error("failed to read response body", zap.Error(err))
			} else {
				zap.L().Error("error doing request", zap.Any("response", string(res)), zap.String("url", event.url))
			}
		}
	}
}

func (s *Whatsmiau) emit(body any, url string) {
	s.emitter <- emitter{url, body}
}

func (s *Whatsmiau) Handle(id string) whatsmeow.EventHandler {
	return func(evt any) {
		if s == nil {
			zap.L().Error("nil whatsmiau receiver in event handler", zap.String("instance", id), zap.String("type", fmt.Sprintf("%T", evt)))
			return
		}

		// If handlerSemaphore is nil for any reason, proceed without concurrency limiting.
		if s.handlerSemaphore != nil {
			s.handlerSemaphore <- struct{}{}
		}

		go func() {
			if s.handlerSemaphore != nil {
				defer func() { <-s.handlerSemaphore }()
			}
			instance := s.getInstanceCached(id)
			if instance == nil {
				zap.L().Warn("no instance found for event", zap.String("instance", id))
				return
			}

			eventMap := make(map[string]bool)
			for _, event := range instance.Webhook.Events {
				eventMap[event] = true
			}

			switch e := evt.(type) {
			case *events.LoggedOut:
				s.handleLoggedOut(id)
			case *events.Message:
				s.handleMessageEvent(id, instance, e, eventMap)
			case *events.Receipt:
				s.handleReceiptEvent(id, instance, e, eventMap)
			case *events.BusinessName:
				s.handleBusinessNameEvent(id, instance, e, eventMap)
			case *events.Contact:
				s.handleContactEvent(id, instance, e, eventMap)
			case *events.Picture:
				s.handlePictureEvent(id, instance, e, eventMap)
			case *events.HistorySync:
				s.handleHistorySyncEvent(id, instance, e, eventMap)
			case *events.GroupInfo:
				s.handleGroupInfoEvent(id, instance, e, eventMap)
			case *events.PushName:
				s.handlePushNameEvent(id, instance, e, eventMap)
			default:
				zap.L().Debug("unknown event", zap.String("type", fmt.Sprintf("%T", evt)), zap.Any("raw", evt))
			}
		}()
	}
}

func (s *Whatsmiau) handleLoggedOut(id string) {
	client, ok := s.clients.Load(id)
	if ok {
		if err := s.deleteDeviceIfExists(context.Background(), client); err != nil {
			zap.L().Error("failed to delete device for instance", zap.String("instance", id), zap.Error(err))
			return
		}
	}

	s.clients.Delete(id)
}
func (s *Whatsmiau) handleMessageEvent(id string, instance *models.Instance, e *events.Message, eventMap map[string]bool) {
	if !eventMap["MESSAGES_UPSERT"] {
		return
	}

	if canIgnoreGroup(e, instance) {
		return
	}

	if canIgnoreMessage(e) {
		return
	}

	messageData := s.convertEventMessage(id, instance, e)
	if messageData == nil {
		zap.L().Error("failed to convert event", zap.String("id", id), zap.String("type", fmt.Sprintf("%T", e)), zap.Any("raw", e))
		return
	}

	messageData.InstanceId = instance.ID

	dateTime := time.Unix(int64(messageData.MessageTimestamp), 0)
	wookMessage := &WookEvent[WookMessageData]{
		Instance: instance.ID,
		Data:     messageData,
		DateTime: dateTime,
		Event:    WookMessagesUpsert,
	}

	if wookMessage.Data.Message != nil && len(wookMessage.Data.Message.Base64) > 0 {
		b64Temp := wookMessage.Data.Message.Base64
		wookMessage.Data.Message.Base64 = ""
		zap.L().Debug("message event", zap.String("instance", id), zap.Any("data", wookMessage.Data))
		wookMessage.Data.Message.Base64 = b64Temp
	} else if wookMessage.Data.Message != nil {
		zap.L().Debug("message event", zap.String("instance", id), zap.Any("data", wookMessage.Data))
	}

	// Store message for later fetch (history/backfill/debugging).
	if s.messageStore != nil && wookMessage.Data != nil && wookMessage.Data.Key != nil {
		if raw, err := json.Marshal(wookMessage.Data); err != nil {
			zap.L().Warn("failed to marshal message for store", zap.Error(err))
		} else {
			_ = s.messageStore.Upsert(
				context.Background(),
				instance.ID,
				wookMessage.Data.Key.RemoteJid,
				wookMessage.Data.Key.Id,
				int64(wookMessage.Data.MessageTimestamp),
				raw,
			)
		}
	}

	// Optional: persist directly to MongoDB (so the app doesn't depend on webhooks for storage).
	s.persistMongoMessage(instance, wookMessage.Data, true)

	s.emit(wookMessage, instance.Webhook.Url)
}

func (s *Whatsmiau) handleReceiptEvent(id string, instance *models.Instance, e *events.Receipt, eventMap map[string]bool) {
	if !eventMap["MESSAGES_UPDATE"] {
		return
	}

	if canIgnoreGroup(e, instance) {
		return
	}

	data := s.convertEventReceipt(id, e)
	if data == nil {
		return
	}

	for _, event := range data {
		wookData := &WookEvent[WookMessageUpdateData]{
			Instance: instance.ID,
			Data:     &event,
			DateTime: e.Timestamp,
			Event:    WookMessagesUpdate,
		}

		// Best-effort Mongo status update.
		s.persistMongoStatus(instance, &event)

		s.emit(wookData, instance.Webhook.Url)
	}
}

func (s *Whatsmiau) handleBusinessNameEvent(id string, instance *models.Instance, e *events.BusinessName, eventMap map[string]bool) {
	if !eventMap["CONTACTS_UPSERT"] {
		return
	}

	data := s.convertBusinessName(id, e)
	if data == nil {
		zap.L().Error("failed to convert business name", zap.String("id", id), zap.String("type", fmt.Sprintf("%T", e)), zap.Any("raw", e))
		return
	}

	wookData := &WookEvent[WookContactUpsertData]{
		Instance: instance.ID,
		Data:     &WookContactUpsertData{*data},
		DateTime: time.Now(),
		Event:    WookContactsUpsert,
	}

	// Optional: persist contact to Mongo + fanout.
	s.persistMongoContacts(instance, []WookContact{*data})

	s.emit(wookData, instance.Webhook.Url)
}

func (s *Whatsmiau) handleContactEvent(id string, instance *models.Instance, e *events.Contact, eventMap map[string]bool) {
	if !eventMap["CONTACTS_UPSERT"] {
		return
	}

	if canIgnoreGroup(e, instance) {
		return
	}

	data := s.convertContact(id, e)
	if data == nil {
		zap.L().Error("failed to convert contact", zap.String("id", id), zap.String("type", fmt.Sprintf("%T", e)), zap.Any("raw", e))
		return
	}

	wookData := &WookEvent[WookContactUpsertData]{
		Instance: instance.ID,
		Data:     &WookContactUpsertData{*data},
		DateTime: time.Now(),
		Event:    WookContactsUpsert,
	}

	s.persistMongoContacts(instance, []WookContact{*data})

	s.emit(wookData, instance.Webhook.Url)
}

func (s *Whatsmiau) handlePictureEvent(id string, instance *models.Instance, e *events.Picture, eventMap map[string]bool) {
	if !eventMap["CONTACTS_UPSERT"] {
		return
	}

	data := s.convertPicture(id, e)
	if data == nil {
		return
	}

	wookData := &WookEvent[WookContactUpsertData]{
		Instance: instance.ID,
		Data:     &WookContactUpsertData{*data},
		DateTime: e.Timestamp,
		Event:    WookContactsUpsert,
	}

	s.persistMongoContacts(instance, []WookContact{*data})

	s.emit(wookData, instance.Webhook.Url)
}

func (s *Whatsmiau) handleHistorySyncEvent(id string, instance *models.Instance, e *events.HistorySync, eventMap map[string]bool) {
	// 1) Store historical messages for later fetch
	// 2) Still emit contacts.upsert (pushnames) for the app
	if s.messageStore != nil || s.mongoMessages != nil {
		client, ok := s.clients.Load(id)
		if ok && e != nil && e.Data != nil {
			// Safety cap to avoid flooding on very large history syncs.
			stored := 0
			const maxStore = 5000
			for _, conv := range e.Data.GetConversations() {
				for _, hmsg := range conv.GetMessages() {
					if stored >= maxStore {
						break
					}
					wmi := hmsg.GetMessage()
					if wmi == nil || wmi.GetKey() == nil {
						continue
					}
					msg := s.convertHistorySyncWebMessage(id, instance, client, wmi)
					if msg == nil || msg.Key == nil {
						continue
					}

					// Persist to Mongo so the app can load historical messages after connection recreation.
					// Do NOT publish per-message events to Redis Streams here (would flood the UI on full sync).
					if s.mongoMessages != nil {
						s.persistMongoMessage(instance, msg, false)
					}

					raw, err := json.Marshal(msg)
					if err != nil {
						continue
					}
					if s.messageStore != nil {
						_ = s.messageStore.Upsert(
							context.Background(),
							instance.ID,
							msg.Key.RemoteJid,
							msg.Key.Id,
							int64(msg.MessageTimestamp),
							raw,
						)
					}
					stored++
				}
				if stored >= maxStore {
					break
				}
			}
			if stored > 0 {
				zap.L().Info("stored history sync messages", zap.String("instance", id), zap.Int("count", stored))

				// Publish a single event so the app can refresh the conversations list.
				if s.mongoMessages != nil {
					if meta, err := s.mongoMessages.ResolveMeta(context.Background(), instance.ID); err == nil && meta != nil {
						s.publishStream(context.Background(), meta.TenantID.Hex(), "history:sync", map[string]string{
							"instanceName": instance.ID,
							"count":        fmt.Sprintf("%d", stored),
						})
					}
				}
			}
		}
	}

	// Emit contacts.upsert from pushnames (existing behavior).
	if eventMap["CONTACTS_UPSERT"] {
		data := s.convertContactHistorySync(id, e.Data.GetPushnames(), e.Data.Conversations)
		if data != nil {
			wookData := &WookEvent[WookContactUpsertData]{
				Instance: instance.ID,
				Data:     &data,
				DateTime: time.Now(),
				Event:    WookContactsUpsert,
			}
			s.emit(wookData, instance.Webhook.Url)
		}
	}
}

func (s *Whatsmiau) convertHistorySyncWebMessage(id string, instance *models.Instance, client *whatsmeow.Client, wmi *waWeb.WebMessageInfo) *WookMessageData {
	if wmi == nil || wmi.GetKey() == nil || wmi.GetMessage() == nil {
		return nil
	}

	key := wmi.GetKey()
	jid := key.GetRemoteJID()

	participant := key.GetParticipant()
	if participant == "" {
		participant = jid
	}

	wookKey := &WookKey{
		RemoteJid:   jid,
		RemoteLid:   "",
		FromMe:      key.GetFromMe(),
		Id:          key.GetID(),
		Participant: participant,
	}

	status := "received"
	if key.GetFromMe() {
		status = "sent"
	}

	ts := int64(wmi.GetMessageTimestamp())
	if ts <= 0 {
		ts = time.Now().Unix()
	}

	messageType, raw, ci := s.parseWAMessage(wmi.GetMessage())

	// Upload media to configured storage (MinIO/S3/GCS) when possible.
	switch messageType {
	case "imageMessage":
		if img := wmi.GetMessage().GetImageMessage(); img != nil {
			raw.MediaURL, raw.Base64, raw.MediaKey = s.uploadMessageFile(context.Background(), instance, client, img, img.GetMimetype(), "")
		}
	case "audioMessage":
		if aud := wmi.GetMessage().GetAudioMessage(); aud != nil {
			raw.MediaURL, raw.Base64, raw.MediaKey = s.uploadMessageFile(context.Background(), instance, client, aud, aud.GetMimetype(), "")
		}
	case "documentMessage":
		if doc := wmi.GetMessage().GetDocumentMessage(); doc != nil {
			raw.MediaURL, raw.Base64, raw.MediaKey = s.uploadMessageFile(context.Background(), instance, client, doc, doc.GetMimetype(), doc.GetFileName())
		}
	case "videoMessage":
		if vid := wmi.GetMessage().GetVideoMessage(); vid != nil {
			raw.MediaURL, raw.Base64, raw.MediaKey = s.uploadMessageFile(context.Background(), instance, client, vid, vid.GetMimetype(), "")
		}
	}

	// Minimal context info (enough for quoted/mentions later, if needed).
	var messageContext WookMessageContextInfo
	if ci != nil {
		messageContext.StanzaId = ci.GetStanzaID()
		messageContext.Participant = ci.GetParticipant()
		messageContext.Expiration = int(ci.GetExpiration())
		messageContext.MentionedJid = ci.GetMentionedJID()
	}

	return &WookMessageData{
		Key:              wookKey,
		PushName:         strings.TrimSpace(wmi.GetPushName()),
		Status:           status,
		Message:          raw,
		ContextInfo:      &messageContext,
		MessageType:      messageType,
		MessageTimestamp: int(ts),
		InstanceId:       instance.ID,
		Source:           "whatsapp",
	}
}

func (s *Whatsmiau) handleGroupInfoEvent(id string, instance *models.Instance, e *events.GroupInfo, eventMap map[string]bool) {
	if !eventMap["CONTACTS_UPSERT"] {
		return
	}

	if instance.GroupsIgnore {
		return
	}

	data := s.convertGroupInfo(id, e)
	if data == nil {
		zap.L().Debug("failed to convert group info", zap.String("id", id), zap.String("type", fmt.Sprintf("%T", e)), zap.Any("raw", e))
		return
	}

	wookData := &WookEvent[WookContactUpsertData]{
		Instance: instance.ID,
		Data:     &WookContactUpsertData{*data},
		DateTime: time.Now(),
		Event:    WookContactsUpsert,
	}

	s.emit(wookData, instance.Webhook.Url)
}

func (s *Whatsmiau) handlePushNameEvent(id string, instance *models.Instance, e *events.PushName, eventMap map[string]bool) {
	if !eventMap["CONTACTS_UPSERT"] {
		return
	}

	if canIgnoreGroup(e, instance) {
		return
	}

	data := s.convertPushName(id, e)
	if data == nil {
		zap.L().Error("failed to convert pushname", zap.String("id", id), zap.String("type", fmt.Sprintf("%T", e)), zap.Any("raw", e))
		return
	}

	wookData := &WookEvent[WookContactUpsertData]{
		Instance: instance.ID,
		Data:     &WookContactUpsertData{*data},
		DateTime: time.Now(),
		Event:    WookContactsUpsert,
	}

	s.emit(wookData, instance.Webhook.Url)
}

// parseWAMessage converts a raw waE2E.Message into our internal representation.
// It only inspects the content of the protobuf message itself –
// media upload (URL/Base64 generation) is handled later by the caller.
func (s *Whatsmiau) parseWAMessage(m *waE2E.Message) (string, *WookMessageRaw, *waE2E.ContextInfo) {
	if m == nil {
		return "unknown", &WookMessageRaw{}, nil
	}
	m = unwrapNestedMessage(m)

	var messageType string
	raw := &WookMessageRaw{}
	var ci *waE2E.ContextInfo

	// === Prioritize action-like messages ===
	if r := m.GetReactionMessage(); r != nil {
		messageType = "reactionMessage"
		reactionKey := &WookKey{}
		if rk := r.GetKey(); rk != nil {
			reactionKey.RemoteJid = rk.GetRemoteJID()
			reactionKey.FromMe = rk.GetFromMe()
			reactionKey.Id = rk.GetID()
			reactionKey.Participant = rk.GetParticipant()
		}
		raw.ReactionMessage = &ReactionMessageRaw{
			Text:              r.GetText(),
			SenderTimestampMs: i64(r.GetSenderTimestampMS()),
			Key:               reactionKey,
		}
	} else if lr := m.GetListResponseMessage(); lr != nil {
		messageType = "listResponseMessage"
		listType := lr.GetListType().String()
		var selectedRowID string
		if ssr := lr.GetSingleSelectReply(); ssr != nil {
			selectedRowID = ssr.GetSelectedRowID()
		}
		raw.ListResponseMessage = &WookListMessageRaw{
			ListType: listType,
			SingleSelectReply: &WookListMessageRawListSingleSelectReply{
				SelectedRowId: selectedRowID,
			},
		}
	} else if img := m.GetImageMessage(); img != nil {
		messageType = "imageMessage"
		ci = img.GetContextInfo()
		raw.ImageMessage = &WookImageMessageRaw{
			Url:               img.GetURL(),
			Mimetype:          img.GetMimetype(),
			FileSha256:        b64(img.GetFileSHA256()),
			FileLength:        u64(img.GetFileLength()),
			Height:            int(img.GetHeight()),
			Width:             int(img.GetWidth()),
			Caption:           img.GetCaption(),
			MediaKey:          b64(img.GetMediaKey()),
			FileEncSha256:     b64(img.GetFileEncSHA256()),
			DirectPath:        img.GetDirectPath(),
			MediaKeyTimestamp: i64(img.GetMediaKeyTimestamp()),
			JpegThumbnail:     b64(img.GetJPEGThumbnail()),
			ViewOnce:          img.GetViewOnce(),
		}
	} else if aud := m.GetAudioMessage(); aud != nil {
		messageType = "audioMessage"
		ci = aud.GetContextInfo()
		raw.AudioMessage = &WookAudioMessageRaw{
			Url:               aud.GetURL(),
			Mimetype:          aud.GetMimetype(),
			FileSha256:        b64(aud.GetFileSHA256()),
			FileLength:        u64(aud.GetFileLength()),
			Seconds:           int(aud.GetSeconds()),
			Ptt:               aud.GetPTT(),
			MediaKey:          b64(aud.GetMediaKey()),
			FileEncSha256:     b64(aud.GetFileEncSHA256()),
			DirectPath:        aud.GetDirectPath(),
			MediaKeyTimestamp: i64(aud.GetMediaKeyTimestamp()),
			Waveform:          b64(aud.GetWaveform()),
			ViewOnce:          aud.GetViewOnce(),
		}
	} else if doc := m.GetDocumentMessage(); doc != nil {
		messageType = "documentMessage"
		ci = doc.GetContextInfo()
		raw.DocumentMessage = &WookDocumentMessageRaw{
			Url:               doc.GetURL(),
			Mimetype:          doc.GetMimetype(),
			Title:             doc.GetTitle(),
			FileSha256:        b64(doc.GetFileSHA256()),
			FileLength:        u64(doc.GetFileLength()),
			PageCount:         int(doc.GetPageCount()),
			MediaKey:          b64(doc.GetMediaKey()),
			FileName:          doc.GetFileName(),
			FileEncSha256:     b64(doc.GetFileEncSHA256()),
			DirectPath:        doc.GetDirectPath(),
			MediaKeyTimestamp: i64(doc.GetMediaKeyTimestamp()),
			ContactVcard:      doc.GetContactVcard(),
			JpegThumbnail:     b64(doc.GetJPEGThumbnail()),
			Caption:           doc.GetCaption(),
		}
	} else if video := m.GetVideoMessage(); video != nil {
		messageType = "videoMessage"
		raw.VideoMessage = &WookVideoMessageRaw{
			Url:           video.GetURL(),
			Mimetype:      video.GetMimetype(),
			Caption:       video.GetCaption(),
			FileSha256:    b64(video.GetFileSHA256()),
			FileLength:    u64(video.GetFileLength()),
			Seconds:       video.GetSeconds(),
			MediaKey:      b64(video.GetMediaKey()),
			FileEncSha256: b64(video.GetFileEncSHA256()),
			JPEGThumbnail: b64(video.GetJPEGThumbnail()),
			GIFPlayback:   video.GetGifPlayback(),
		}
		ci = video.GetContextInfo()
	} else if contact := m.GetContactMessage(); contact != nil {
		card, err := vcard.NewDecoder(strings.NewReader(contact.GetVcard())).Decode()
		if err != nil {
			zap.L().Error("decode card error", zap.Error(err))
		}

		messageType = "contactMessage"
		raw.ContactMessage = &ContactMessageRaw{
			VCard:        contact.GetVcard(),
			DisplayName:  contact.GetDisplayName(),
			DecodedVcard: card,
		}
		ci = contact.GetContextInfo()
	} else if contactArray := m.GetContactsArrayMessage(); contactArray != nil {
		messageType = "contactsArrayMessage"
		var contacts []ContactMessageRaw
		for _, contact := range contactArray.Contacts {
			card, err := vcard.NewDecoder(strings.NewReader(contact.GetVcard())).Decode()
			if err != nil {
				zap.L().Error("decode card error", zap.Error(err))
			}

			contacts = append(contacts, ContactMessageRaw{
				VCard:        contact.GetVcard(),
				DisplayName:  contact.GetDisplayName(),
				DecodedVcard: card,
			})
		}
		raw.ContactsArrayMessage = &ContactsArrayMessageRaw{
			DisplayName: contactArray.GetDisplayName(),
			Contacts:    contacts,
		}
		ci = contactArray.GetContextInfo()
	} else if sticker := m.GetStickerMessage(); sticker != nil {
		messageType = "stickerMessage"
		raw.Conversation = "[Sticker]"
	} else if loc := m.GetLocationMessage(); loc != nil {
		messageType = "locationMessage"
		name := strings.TrimSpace(loc.GetName())
		if name == "" {
			name = "[Localizacao]"
		}
		raw.Conversation = name
	} else if m.GetLiveLocationMessage() != nil {
		messageType = "liveLocationMessage"
		raw.Conversation = "[Localizacao ao vivo]"
	} else if poll := m.GetPollCreationMessage(); poll != nil {
		messageType = "pollCreationMessage"
		name := strings.TrimSpace(poll.GetName())
		if name == "" {
			name = "[Enquete]"
		}
		raw.Conversation = name
	} else if conv := strings.TrimSpace(m.GetConversation()); conv != "" {
		messageType = "conversation"
		raw.Conversation = conv
	} else if et := m.GetExtendedTextMessage(); et != nil && len(et.GetText()) > 0 {
		messageType = "conversation"
		raw.Conversation = et.GetText()
		ci = et.GetContextInfo()
	} else {
		if env.Env.DebugRawMsgs && m != nil {
			// Print the raw protobuf JSON to help mapping new message types (ephemeral/viewOnce/sticker/etc).
			b, err := protojson.MarshalOptions{
				UseProtoNames:   true,
				EmitUnpopulated: false,
			}.Marshal(m)
			if err != nil {
				zap.L().Warn("unknown message: failed to marshal proto", zap.Error(err))
			} else {
				const max = 20000
				rawJSON := string(b)
				if len(rawJSON) > max {
					rawJSON = rawJSON[:max] + "...(truncated)"
				}
				zap.L().Warn("unknown message: raw proto", zap.String("raw", rawJSON))
			}
		}
		messageType = "unknown"
	}

	return messageType, raw, ci
}

func unwrapNestedMessage(m *waE2E.Message) *waE2E.Message {
	current := m
	for i := 0; i < 8 && current != nil; i++ {
		var next *waE2E.Message

		switch {
		case current.GetDeviceSentMessage() != nil:
			next = current.GetDeviceSentMessage().GetMessage()
		case current.GetEphemeralMessage() != nil:
			next = current.GetEphemeralMessage().GetMessage()
		case current.GetViewOnceMessage() != nil:
			next = current.GetViewOnceMessage().GetMessage()
		case current.GetViewOnceMessageV2() != nil:
			next = current.GetViewOnceMessageV2().GetMessage()
		case current.GetViewOnceMessageV2Extension() != nil:
			next = current.GetViewOnceMessageV2Extension().GetMessage()
		case current.GetDocumentWithCaptionMessage() != nil:
			next = current.GetDocumentWithCaptionMessage().GetMessage()
		case current.GetEditedMessage() != nil:
			next = current.GetEditedMessage().GetMessage()
		case current.GetGroupMentionedMessage() != nil:
			next = current.GetGroupMentionedMessage().GetMessage()
		}

		if next == nil {
			break
		}
		current = next
	}
	return current
}

func (s *Whatsmiau) convertContactHistorySync(id string, event []*waHistorySync.Pushname, conversations []*waHistorySync.Conversation) WookContactUpsertData {
	resultMap := make(map[string]WookContact)
	for _, pushName := range event {

		if len(pushName.GetPushname()) == 0 {
			continue
		}

		if dt := strings.Split(pushName.GetPushname(), "@"); len(dt) == 2 && (dt[1] == "g.us" || dt[1] == "s.whatsapp.net") {
			continue
		}

		jid, err := types.ParseJID(pushName.GetID())
		if err != nil {
			zap.L().Error("failed to parse jid", zap.String("pushname", pushName.GetPushname()))
			continue
		}

		jidParsed, lid := s.GetJidLid(context.Background(), id, jid)

		resultMap[jidParsed] = WookContact{
			RemoteJid:  jidParsed,
			PushName:   pushName.GetPushname(),
			InstanceId: id,
			RemoteLid:  lid,
		}
	}

	for _, conversation := range conversations {
		name := conversation.GetName()
		if len(name) == 0 {
			name = conversation.GetDisplayName()
		}
		if len(name) == 0 {
			name = conversation.GetUsername()
		}
		if len(name) == 0 {
			continue
		}
		if dt := strings.Split(name, "@"); len(dt) == 2 && (dt[1] == "g.us" || dt[1] == "s.whatsapp.net") {
			continue
		}

		jid, err := types.ParseJID(conversation.GetID())
		if err != nil {
			zap.L().Error("failed to parse jid", zap.String("name", conversation.GetName()))
			continue
		}
		jidParsed, lid := s.GetJidLid(context.Background(), id, jid)

		resultMap[conversation.GetID()] = WookContact{
			RemoteJid:  jidParsed,
			PushName:   name,
			InstanceId: id,
			RemoteLid:  lid,
		}
	}

	var result []WookContact
	for _, c := range resultMap {
		jid, err := types.ParseJID(c.RemoteJid)
		if err != nil {
			continue
		}

		url, _, err := s.getPic(id, jid)
		if err != nil {
			zap.L().Error("failed to get pic", zap.Error(err))
		}

		c.ProfilePicUrl = url
		result = append(result, c)
	}

	return result
}

func (s *Whatsmiau) convertEventMessage(id string, instance *models.Instance, evt *events.Message) *WookMessageData {
	ctx, c := context.WithTimeout(context.Background(), time.Second*60)
	defer c()

	client, ok := s.clients.Load(id)
	if !ok {
		zap.L().Warn("no client for event", zap.String("id", id))
		return nil
	}

	if evt == nil || evt.Message == nil {
		return nil
	}

	jid, lid := s.GetJidLid(ctx, id, evt.Info.Chat)
	senderJid, _ := s.GetJidLid(ctx, id, evt.Info.Sender)

	// Always unwrap to work with the real content
	e := evt.UnwrapRaw()
	m := e.Message

	// Build the key
	key := &WookKey{
		RemoteJid:   jid,
		RemoteLid:   lid,
		FromMe:      e.Info.IsFromMe,
		Id:          e.Info.ID,
		Participant: senderJid,
	}

	// Determine status
	status := "received"
	if e.Info.IsFromMe {
		status = "sent"
	}

	// Timestamp
	ts := e.Info.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	// Convert the WA protobuf message into our internal raw structure
	messageType, raw, ci := s.parseWAMessage(m)

	// Upload media (URL / Base64) when needed
	switch messageType {
	case "imageMessage":
		if img := m.GetImageMessage(); img != nil {
			raw.MediaURL, raw.Base64, raw.MediaKey = s.uploadMessageFile(ctx, instance, client, img, img.GetMimetype(), "")
		}
	case "audioMessage":
		if aud := m.GetAudioMessage(); aud != nil {
			raw.MediaURL, raw.Base64, raw.MediaKey = s.uploadMessageFile(ctx, instance, client, aud, aud.GetMimetype(), "")
		}
	case "documentMessage":
		if doc := m.GetDocumentMessage(); doc != nil {
			raw.MediaURL, raw.Base64, raw.MediaKey = s.uploadMessageFile(ctx, instance, client, doc, doc.GetMimetype(), doc.GetFileName())
		}
	case "videoMessage":
		if vid := m.GetVideoMessage(); vid != nil {
			raw.MediaURL, raw.Base64, raw.MediaKey = s.uploadMessageFile(ctx, instance, client, vid, vid.GetMimetype(), "")
		}
	}

	// Map MessageContextInfo (quoted, mentions, disappearing mode, external ad reply)
	var messageContext WookMessageContextInfo
	if ci != nil {
		messageContext.EphemeralSettingTimestamp = i64(ci.GetEphemeralSettingTimestamp())
		messageContext.StanzaId = ci.GetStanzaID()
		messageContext.Participant = ci.GetParticipant()
		messageContext.Expiration = int(ci.GetExpiration())
		messageContext.MentionedJid = ci.GetMentionedJID()
		messageContext.ConversionSource = ci.GetConversionSource()
		messageContext.ConversionData = b64(ci.GetConversionData())
		messageContext.ConversionDelaySeconds = int(ci.GetConversionDelaySeconds())
		messageContext.EntryPointConversionSource = ci.GetEntryPointConversionSource()
		messageContext.EntryPointConversionApp = ci.GetEntryPointConversionApp()
		messageContext.EntryPointConversionDelaySeconds = int(ci.GetEntryPointConversionDelaySeconds())
		messageContext.TrustBannerAction = ci.GetTrustBannerAction()

		if dm := ci.GetDisappearingMode(); dm != nil {
			messageContext.DisappearingMode = &ContextInfoDisappearingMode{
				Initiator:     dm.GetInitiator().String(),
				Trigger:       dm.GetTrigger().String(),
				InitiatedByMe: dm.GetInitiatedByMe(),
			}
		}

		if ear := ci.GetExternalAdReply(); ear != nil {
			messageType = "conversation"
			messageContext.ExternalAdReply = &WookMessageContextInfoExternalAdReply{
				Title:                 ear.GetTitle(),
				Body:                  ear.GetBody(),
				MediaType:             ear.GetMediaType().String(),
				ThumbnailUrl:          ear.GetThumbnailURL(),
				Thumbnail:             b64(ear.GetThumbnail()),
				SourceType:            ear.GetSourceType(),
				SourceId:              ear.GetSourceID(),
				SourceUrl:             ear.GetSourceURL(),
				ContainsAutoReply:     ear.GetContainsAutoReply(),
				RenderLargerThumbnail: ear.GetRenderLargerThumbnail(),
				ShowAdAttribution:     ear.GetShowAdAttribution(),
				CtwaClid:              ear.GetCtwaClid(),
			}
		}

		if qm := ci.GetQuotedMessage(); qm != nil {
			_, qmRaw, _ := s.parseWAMessage(qm)
			messageContext.QuotedMessage = qmRaw
		}
	}

	return &WookMessageData{
		Key:              key,
		PushName:         strings.TrimSpace(e.Info.PushName),
		Status:           status,
		Message:          raw,
		ContextInfo:      &messageContext,
		MessageType:      messageType,
		MessageTimestamp: int(ts.Unix()),
		InstanceId:       id,
		Source:           "whatsapp",
	}
}

func (s *Whatsmiau) convertEventReceipt(id string, evt *events.Receipt) []WookMessageUpdateData {
	var status WookMessageUpdateStatus
	switch evt.Type {
	case types.ReceiptTypeRead:
		status = MessageStatusRead
	case types.ReceiptTypeDelivered:
		status = MessageStatusDeliveryAck
	default:
		return nil
	}

	chatJid, chatLid := s.GetJidLid(context.Background(), id, evt.Chat)
	participantJid, _ := s.GetJidLid(context.Background(), id, evt.Sender)

	var result []WookMessageUpdateData
	for _, messageID := range evt.MessageIDs {
		result = append(result, WookMessageUpdateData{
			MessageId:   messageID,
			KeyId:       messageID,
			RemoteJid:   chatJid,
			RemoteLid:   chatLid,
			FromMe:      evt.IsFromMe,
			Participant: participantJid,
			Status:      status,
			InstanceId:  id,
		})
	}

	return result
}

func (s *Whatsmiau) uploadMessageFile(ctx context.Context, instance *models.Instance, client *whatsmeow.Client, fileMessage whatsmeow.DownloadableMessage, mimetype, fileName string) (string, string, string) {
	var (
		b64Result string
		urlResult string
		keyResult string
		ext       string
	)

	tmpFile, err := os.CreateTemp("", "file-*")
	if err != nil {
		panic(err)
	}

	defer os.Remove(tmpFile.Name())
	if err := client.DownloadToFile(ctx, fileMessage, tmpFile); err != nil {
		// History sync often contains media that is no longer retrievable from WhatsApp's CDN (403).
		// This should not stop message persistence; we just skip uploading media.
		if strings.Contains(err.Error(), "status code 403") || strings.Contains(err.Error(), "403") {
			zap.L().Debug("media download forbidden (likely old history)", zap.Error(err))
		} else {
			zap.L().Error("failed to download media", zap.Error(err))
		}
		return "", "", ""
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		zap.L().Error("failed to seek image", zap.Error(err))
	}

	ext = extractExtFromFile(fileName, mimetype, tmpFile)
	if instance.Webhook.Base64 != nil && *instance.Webhook.Base64 {
		data, err := io.ReadAll(tmpFile)
		if err != nil {
			zap.L().Error("failed to read image", zap.Error(err))
		} else {
			b64Result = base64.StdEncoding.EncodeToString(data)
		}
	}
	if s.fileStorage != nil {
		if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
			zap.L().Error("failed to seek image", zap.Error(err))
		}

		objectKey := uuid.NewString() + "." + ext
		urlResult, keyResult, err = s.fileStorage.Upload(ctx, objectKey, mimetype, tmpFile)
		if err != nil {
			zap.L().Error("failed to upload image", zap.Error(err))
		}
	}

	return urlResult, b64Result, keyResult
}

func (s *Whatsmiau) convertContact(id string, evt *events.Contact) *WookContact {
	url, _, err := s.getPic(id, evt.JID)
	if err != nil {
		zap.L().Error("failed to get pic", zap.Error(err))
	}

	name := evt.Action.GetFirstName()
	if name == "" {
		name = evt.Action.GetFullName()
	}
	if name == "" {
		name = evt.Action.GetUsername()
	}
	if name == "" {
		return nil
	}

	if dt := strings.Split(name, "@"); len(dt) == 2 && (dt[1] == "g.us" || dt[1] == "s.whatsapp.net") {
		return nil
	}

	jid, lid := s.GetJidLid(context.Background(), id, evt.JID)
	return &WookContact{
		RemoteJid:     jid,
		RemoteLid:     lid,
		PushName:      name,
		ProfilePicUrl: url,
		InstanceId:    id,
	}
}

func (s *Whatsmiau) convertGroupInfo(id string, evt *events.GroupInfo) *WookContact {
	url, _, err := s.getPic(id, evt.JID)
	if err != nil {
		zap.L().Error("failed to get pic", zap.Error(err))
	}

	if evt.Name == nil || len(evt.Name.Name) == 0 {
		return nil
	}

	if dt := strings.Split(evt.Name.Name, "@"); len(dt) == 2 && (dt[1] == "g.us" || dt[1] == "s.whatsapp.net") {
		return nil
	}

	jid, lid := s.GetJidLid(context.Background(), id, evt.JID)

	return &WookContact{
		RemoteJid:     jid,
		PushName:      evt.Name.Name,
		ProfilePicUrl: url,
		InstanceId:    id,
		RemoteLid:     lid,
	}
}

func (s *Whatsmiau) convertPushName(id string, evt *events.PushName) *WookContact {
	url, _, err := s.getPic(id, evt.JID)
	if err != nil {
		zap.L().Error("failed to get pic", zap.Error(err))
	}

	name := evt.NewPushName
	if len(name) == 0 {
		name = evt.OldPushName
	}

	if name == "" {
		return nil
	}

	if dt := strings.Split(name, "@"); len(dt) == 2 && (dt[1] == "g.us" || dt[1] == "s.whatsapp.net") {
		return nil
	}

	jid, lid := s.GetJidLid(context.Background(), id, evt.JID)

	return &WookContact{
		RemoteJid:     jid,
		PushName:      evt.NewPushName,
		InstanceId:    id,
		ProfilePicUrl: url,
		RemoteLid:     lid,
	}
}

func (s *Whatsmiau) convertPicture(id string, evt *events.Picture) *WookContact {
	url, b64, err := s.getPic(id, evt.JID)
	if err != nil {
		zap.L().Error("failed to get pic", zap.Error(err))
	}

	if len(url) <= 0 {
		return nil
	}

	jid, lid := s.GetJidLid(context.Background(), id, evt.JID)

	return &WookContact{
		RemoteJid:     jid,
		InstanceId:    id,
		Base64Pic:     b64,
		ProfilePicUrl: url,
		RemoteLid:     lid,
	}
}

func (s *Whatsmiau) convertBusinessName(id string, evt *events.BusinessName) *WookContact {
	url, b64, err := s.getPic(id, evt.JID)
	if err != nil {
		zap.L().Error("failed to get pic", zap.Error(err))
	}

	name := evt.NewBusinessName
	if name == "" {
		name = evt.OldBusinessName
	}
	if name == "" && evt.Message != nil {
		name = evt.Message.PushName
	}
	if name == "" && evt.Message != nil && evt.Message.VerifiedName != nil && evt.Message.VerifiedName.Details != nil {
		name = evt.Message.VerifiedName.Details.GetVerifiedName()
	}

	if dt := strings.Split(name, "@"); len(dt) == 2 && (dt[1] == "g.us" || dt[1] == "s.whatsapp.net") {
		return nil
	}

	jid, lid := s.GetJidLid(context.Background(), id, evt.JID)

	return &WookContact{
		RemoteJid:     jid,
		InstanceId:    id,
		Base64Pic:     b64,
		ProfilePicUrl: url,
		PushName:      name,
		RemoteLid:     lid,
	}
}

func (s *Whatsmiau) getPic(id string, jid types.JID) (string, string, error) {
	client, ok := s.clients.Load(id)
	if !ok || client == nil {
		zap.L().Warn("no client for event", zap.String("id", id))
		return "", "", fmt.Errorf("no client for event %s", id)
	}

	pic, err := client.GetProfilePictureInfo(context.TODO(), jid, &whatsmeow.GetProfilePictureParams{
		Preview:     true,
		IsCommunity: false,
	})
	if err != nil {
		return "", "", nil
	}

	if pic == nil {
		return "", "", err
	}

	res, err := s.httpClient.Get(pic.URL)
	if err != nil {
		zap.L().Error("get profile picture error", zap.String("id", id), zap.Error(err))
		return "", "", err
	}

	picRaw, err := io.ReadAll(res.Body)
	if err != nil {
		zap.L().Error("get profile picture error", zap.String("id", id), zap.Error(err))
		return "", "", err
	}

	return pic.URL, base64.StdEncoding.EncodeToString(picRaw), nil
}

func (s *Whatsmiau) persistMongoMessage(instance *models.Instance, data *WookMessageData, publish bool) {
	if s == nil || s.mongoMessages == nil || instance == nil || data == nil || data.Key == nil {
		return
	}

	// Track our own pushName to avoid polluting contact records with "self" names.
	if data.Key.FromMe && data.PushName != "" && s.selfPushName != nil {
		s.selfPushName.Store(instance.ID, data.PushName)
	}

	body, msgType, mediaURL, mediaKey, mediaMime, mediaFileName := deriveBodyFromWook(data)

	quotedID := ""
	if data.ContextInfo != nil && data.ContextInfo.StanzaId != "" {
		quotedID = data.ContextInfo.StanzaId
	}

	ts := time.Unix(int64(data.MessageTimestamp), 0)
	if ts.IsZero() || ts.Unix() <= 0 {
		ts = time.Now()
	}

	status := ""
	if data.Key.FromMe {
		status = "sent"
	} else {
		status = "received"
	}

	if err := s.mongoMessages.UpsertMessage(context.Background(), mongomessages.MessageUpsert{
		InstanceName:    instance.ID,
		RemoteJid:       data.Key.RemoteJid,
		RemoteLid:       data.Key.RemoteLid,
		MessageID:       data.Key.Id,
		FromMe:          data.Key.FromMe,
		PushName:        data.PushName,
		Participant:     data.Key.Participant,
		MessageType:     msgType,
		Body:            body,
		MediaKey:        mediaKey,
		MediaURL:        mediaURL,
		MediaMimetype:   mediaMime,
		MediaFileName:   mediaFileName,
		QuotedMessageID: quotedID,
		Status:          status,
		Timestamp:       ts,
	}); err != nil {
		zap.L().Warn("mongo upsert message failed", zap.Error(err), zap.String("instance", instance.ID), zap.String("remoteJid", data.Key.RemoteJid), zap.String("messageId", data.Key.Id))
	}

	// If this is an incoming message, use its pushName to keep Contact.pushName correct.
	// (Contact events can be inconsistent; message events are the most reliable signal.)
	if !data.Key.FromMe && s.mongoContacts != nil && data.PushName != "" {
		isGroup := strings.HasSuffix(data.Key.RemoteJid, "@g.us")
		phone := ""
		if !isGroup {
			phone = strings.Split(data.Key.RemoteJid, "@")[0]
		}
		meta, err := s.mongoContacts.UpsertContact(context.Background(), mongocontacts.ContactUpsert{
			InstanceName:  instance.ID,
			RemoteJid:     data.Key.RemoteJid,
			RemoteLid:     data.Key.RemoteLid,
			PushName:      data.PushName,
			ProfilePicUrl: "",
			IsGroup:       isGroup,
			Phone:         phone,
			UpdatedAt:     time.Now(),
		})
		if err == nil && meta != nil {
			s.publishStream(context.Background(), meta.TenantID.Hex(), "contact:update", map[string]string{
				"remoteJid":     data.Key.RemoteJid,
				"pushName":      data.PushName,
				"profilePicUrl": "",
			})
		}
	}

	if publish {
		// Fanout event to Redis Streams for the backend consumer (Socket.IO).
		meta, err := s.mongoMessages.ResolveMeta(context.Background(), instance.ID)
		if err == nil && meta != nil {
			s.publishStream(context.Background(), meta.TenantID.Hex(), "message:new", map[string]string{
				"instanceName": instance.ID,
				"messageId":    data.Key.Id,
				"remoteJid":    data.Key.RemoteJid,
			})
		}
	}
}

func (s *Whatsmiau) persistMongoStatus(instance *models.Instance, upd *WookMessageUpdateData) {
	if s == nil || s.mongoMessages == nil || instance == nil || upd == nil {
		return
	}
	// Only relevant for our own outgoing messages.
	if !upd.FromMe {
		return
	}

	status := ""
	switch upd.Status {
	case MessageStatusDeliveryAck:
		status = "delivered"
	case MessageStatusRead:
		status = "read"
	default:
		return
	}

	if err := s.mongoMessages.UpdateStatus(context.Background(), instance.ID, upd.MessageId, status); err != nil {
		zap.L().Warn("mongo update status failed", zap.Error(err), zap.String("instance", instance.ID), zap.String("messageId", upd.MessageId), zap.String("status", status))
	}

	meta, err := s.mongoMessages.ResolveMeta(context.Background(), instance.ID)
	if err == nil && meta != nil {
		s.publishStream(context.Background(), meta.TenantID.Hex(), "message:status", map[string]string{
			"instanceName": instance.ID,
			"messageId":    upd.MessageId,
			"status":       status,
		})
	}
}

func (s *Whatsmiau) persistMongoContacts(instance *models.Instance, contacts []WookContact) {
	if s == nil || s.mongoContacts == nil || instance == nil || len(contacts) == 0 {
		return
	}
	selfName := ""
	if s.selfPushName != nil {
		if v, ok := s.selfPushName.Load(instance.ID); ok {
			selfName = v
		}
	}
	for _, c := range contacts {
		isGroup := strings.HasSuffix(c.RemoteJid, "@g.us")
		phone := ""
		if !isGroup {
			phone = strings.Split(c.RemoteJid, "@")[0]
		}

		// Avoid overwriting a contact's name with our own pushName.
		pushName := c.PushName
		if selfName != "" && pushName == selfName {
			pushName = ""
		}

		meta, err := s.mongoContacts.UpsertContact(context.Background(), mongocontacts.ContactUpsert{
			InstanceName:  instance.ID,
			RemoteJid:     c.RemoteJid,
			RemoteLid:     c.RemoteLid,
			PushName:      pushName,
			ProfilePicUrl: c.ProfilePicUrl,
			IsGroup:       isGroup,
			Phone:         phone,
			UpdatedAt:     time.Now(),
		})
		if err == nil && meta != nil {
			s.publishStream(context.Background(), meta.TenantID.Hex(), "contact:update", map[string]string{
				"remoteJid":     c.RemoteJid,
				"pushName":      c.PushName,
				"profilePicUrl": c.ProfilePicUrl,
			})
		}
	}
}

func (s *Whatsmiau) publishStream(ctx context.Context, tenantID, typ string, fields map[string]string) {
	if !env.Env.StreamEnabled {
		return
	}
	rdb := services.Redis()
	if rdb == nil {
		return
	}
	key := env.Env.StreamKey
	if key == "" {
		key = "rz:events"
	}
	maxLen := int64(env.Env.StreamMaxLen)
	if maxLen <= 0 {
		maxLen = 10000
	}

	values := make(map[string]interface{}, 2+len(fields))
	values["tenantId"] = tenantID
	values["type"] = typ
	for k, v := range fields {
		values[k] = v
	}

	args := &redis.XAddArgs{
		Stream: key,
		Values: values,
	}
	if env.Env.StreamMaxLenApprox {
		args.MaxLenApprox = maxLen
	} else {
		args.MaxLen = maxLen
	}
	_ = rdb.XAdd(ctx, args).Err()
}

func deriveBodyFromWook(data *WookMessageData) (body, messageType, mediaURL, mediaKey, mediaMimetype, mediaFileName string) {
	if data == nil || data.Message == nil {
		return "[Mensagem]", "unknown", "", "", "", ""
	}

	// Mirror the Node webhook deriveBody logic.
	if data.Message.Conversation != "" {
		msgType := "conversation"
		if data.MessageType != "" && data.MessageType != "unknown" {
			msgType = data.MessageType
		}
		return data.Message.Conversation, msgType, "", "", "", ""
	}
	if data.Message.ImageMessage != nil {
		caption := data.Message.ImageMessage.Caption
		if caption == "" {
			caption = "[Imagem]"
		}
		url := data.Message.MediaURL
		key := data.Message.MediaKey
		if url == "" {
			url = data.Message.ImageMessage.Url
		}
		return caption, "imageMessage", url, key, data.Message.ImageMessage.Mimetype, ""
	}
	if data.Message.AudioMessage != nil {
		url := data.Message.MediaURL
		key := data.Message.MediaKey
		if url == "" {
			url = data.Message.AudioMessage.Url
		}
		return "[Audio]", "audioMessage", url, key, data.Message.AudioMessage.Mimetype, ""
	}
	if data.Message.VideoMessage != nil {
		caption := data.Message.VideoMessage.Caption
		if caption == "" {
			caption = "[Video]"
		}
		url := data.Message.MediaURL
		key := data.Message.MediaKey
		if url == "" {
			url = data.Message.VideoMessage.Url
		}
		return caption, "videoMessage", url, key, data.Message.VideoMessage.Mimetype, ""
	}
	if data.Message.DocumentMessage != nil {
		name := data.Message.DocumentMessage.FileName
		if name == "" {
			name = "[Documento]"
		}
		url := data.Message.MediaURL
		key := data.Message.MediaKey
		if url == "" {
			url = data.Message.DocumentMessage.Url
		}
		return name, "documentMessage", url, key, data.Message.DocumentMessage.Mimetype, data.Message.DocumentMessage.FileName
	}
	if data.Message.ContactMessage != nil {
		return "[Contato]", "contactMessage", "", "", "", ""
	}
	if data.Message.ReactionMessage != nil {
		return data.Message.ReactionMessage.Text, "reactionMessage", "", "", "", ""
	}
	return "[Mensagem]", "unknown", "", "", "", ""
}
