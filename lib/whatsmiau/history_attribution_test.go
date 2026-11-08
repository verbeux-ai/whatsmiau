package whatsmiau

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/verbeux-ai/whatsmiau/models"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/proto/waWeb"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func historyAttributionContext() *waE2E.ContextInfo {
	return &waE2E.ContextInfo{
		ExternalAdReply: &waE2E.ContextInfo_ExternalAdReplyInfo{
			SourceType: proto.String("ad"), SourceID: proto.String("9007199254740993"),
			SourceURL: proto.String("https://example.invalid/ad"), CtwaClid: proto.String("CLICK-001"),
			MediaType: waE2E.ContextInfo_ExternalAdReplyInfo_IMAGE.Enum(),
			AdType:    waE2E.ContextInfo_ExternalAdReplyInfo_CTWA.Enum(),
			Thumbnail: []byte{1, 2, 3}, ContainsAutoReply: proto.Bool(false),
			AutomatedGreetingMessageShown: proto.Bool(false), GreetingMessageBody: proto.String("PRIVATE_GREETING"),
			CtaPayload: proto.String("PRIVATE_CTA"),
		},
		CtwaPayload: []byte("opaque"), CtwaSignals: proto.String("all,all"), ConversionData: []byte{3, 4},
		ConversionSource: proto.String("FB_Ads"), ConversionDelaySeconds: proto.Uint32(0),
		EntryPointConversionSource: proto.String("ctwa_ad"), EntryPointConversionApp: proto.String("instagram"),
		EntryPointConversionDelaySeconds: proto.Uint32(0), EntryPointConversionExternalSource: proto.String("FB_Ads"),
		EntryPointConversionExternalMedium: proto.String("unavailable"), AlwaysShowAdAttribution: proto.Bool(false),
		Utm:                 &waE2E.ContextInfo_UTMInfo{UtmSource: proto.String("facebook"), UtmCampaign: proto.String("campaign")},
		SmbClientCampaignID: proto.String("client-campaign"), SmbServerCampaignID: proto.String("server-campaign"),
		QuotedAd: &waE2E.ContextInfo_AdReplyInfo{
			AdvertiserName: proto.String("Advertiser"), Caption: proto.String("Ad caption"),
			MediaType: waE2E.ContextInfo_AdReplyInfo_IMAGE.Enum(), JPEGThumbnail: []byte{1, 2, 3},
		},
		StanzaID: proto.String("quoted-id"), Participant: proto.String("5511999999999@s.whatsapp.net"),
		MentionedJID: []string{"5511888888888@s.whatsapp.net"}, Expiration: proto.Uint32(3600),
		EphemeralSettingTimestamp: proto.Int64(1700000000), TrustBannerAction: proto.Uint32(1),
		DisappearingMode: &waE2E.DisappearingMode{InitiatedByMe: proto.Bool(false)},
		QuotedMessage:    &waE2E.Message{Conversation: proto.String("quoted text")},
	}
}

func historyMessageForAttribution(message *waE2E.Message) *waWeb.WebMessageInfo {
	return &waWeb.WebMessageInfo{
		Key: &waCommon.MessageKey{
			RemoteJID: proto.String("5511000000000@s.whatsapp.net"), ID: proto.String("ADS-HISTORY"),
			FromMe: proto.Bool(false), Participant: proto.String("5511888888888@s.whatsapp.net"),
		},
		Message: message, MessageTimestamp: proto.Uint64(1700000000), PushName: proto.String("Contact"),
		Status: waWeb.WebMessageInfo_DELIVERY_ACK.Enum(),
	}
}

func assertHistoricalAttribution(t *testing.T, data *WookMessageData) {
	t.Helper()
	if data == nil || data.ContextInfo == nil || data.Referral == nil || data.CtwaClid != "CLICK-001" {
		t.Fatal("history lost context, referral or click ID")
	}
	var expected, actual map[string]interface{}
	if err := json.Unmarshal([]byte(adAttributionContextJSON), &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mustMarshalAdWebhook(t, data.ContextInfo), &actual); err != nil {
		t.Fatal(err)
	}
	for field, want := range expected {
		if !reflect.DeepEqual(actual[field], want) {
			t.Errorf("historical context lost or changed %s: got %#v, want %#v", field, actual[field], want)
		}
	}
	if data.Referral.CtwaClid != "CLICK-001" || data.Referral.SourceId != "9007199254740993" || data.Referral.Thumbnail != "" {
		t.Fatal("history lost attribution aliases or duplicated thumbnail bytes")
	}
	if data.ContextInfo.QuotedMessage == nil || data.ContextInfo.QuotedMessage.Conversation != "quoted text" || data.ContextInfo.DisappearingMode == nil {
		t.Fatal("history lost quoted content or disappearing mode")
	}
}

func TestHistoryMessagePreservesLiveAdAttribution(t *testing.T) {
	ci := historyAttributionContext()
	text := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("hello"), ContextInfo: ci}}
	cases := []struct {
		name, wantType string
		message        *waE2E.Message
	}{
		{"text", "conversation", text},
		{"empty text", "unknown", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{ContextInfo: ci}}},
		{"image", "imageMessage", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{ContextInfo: ci}}},
		{"video", "videoMessage", &waE2E.Message{VideoMessage: &waE2E.VideoMessage{ContextInfo: ci}}},
		{"audio", "audioMessage", &waE2E.Message{AudioMessage: &waE2E.AudioMessage{ContextInfo: ci}}},
		{"document", "documentMessage", &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{ContextInfo: ci}}},
		{"sticker", "stickerMessage", &waE2E.Message{StickerMessage: &waE2E.StickerMessage{ContextInfo: ci}}},
		{"list response", "listResponseMessage", &waE2E.Message{ListResponseMessage: &waE2E.ListResponseMessage{ContextInfo: ci}}},
		{"button response", "buttonsResponseMessage", &waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{ContextInfo: ci}}},
		{"ephemeral", "conversation", &waE2E.Message{EphemeralMessage: &waE2E.FutureProofMessage{Message: text}}},
		{"view once", "conversation", &waE2E.Message{ViewOnceMessage: &waE2E.FutureProofMessage{Message: text}}},
		{"view once v2", "conversation", &waE2E.Message{ViewOnceMessageV2: &waE2E.FutureProofMessage{Message: text}}},
		{"document with caption", "documentMessage", &waE2E.Message{DocumentWithCaptionMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{ContextInfo: ci}}}}},
		{"ephemeral with envelope context", "conversation", &waE2E.Message{
			MessageContextInfo: &waE2E.MessageContextInfo{MessageSecret: []byte("message-secret")},
			EphemeralMessage:   &waE2E.FutureProofMessage{Message: text},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := proto.Clone(tc.message)
			history := (&Whatsmiau{}).buildMessageDataFromHistory(historyMessageForAttribution(tc.message), "", "")
			assertHistoricalAttribution(t, history)
			live := convertAttributionTestMessage(t, proto.Clone(tc.message).(*waE2E.Message))
			if history.MessageType != tc.wantType {
				t.Fatalf("history messageType=%q, want %q", history.MessageType, tc.wantType)
			}
			if !bytes.Equal(mustMarshalAdWebhook(t, history.ContextInfo), mustMarshalAdWebhook(t, live.ContextInfo)) {
				t.Fatal("history and live events disagree on attribution context")
			}
			if history.Key.Id != "ADS-HISTORY" || history.Key.FromMe || history.MessageTimestamp != 1700000000 || history.PushName != "Contact" || history.Status != "DELIVERY_ACK" || history.Source != "unknown" {
				t.Fatal("ad attribution changed historical message identity or lifecycle metadata")
			}
			if !proto.Equal(original, tc.message) {
				t.Fatal("context conversion changed the original protocol message")
			}
		})
	}
}

func TestHistorySyncWebhookPreservesAdAttribution(t *testing.T) {
	message := historyMessageForAttribution(&waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text: proto.String("hello"), ContextInfo: historyAttributionContext(),
	}})
	service := &Whatsmiau{emitter: make(chan emitter, 1)}
	instance := &models.Instance{ID: "history-ads", SyncFullHistory: true, Webhook: models.InstanceWebhook{Url: "https://example.invalid/history"}}
	event := &events.HistorySync{Data: &waHistorySync.HistorySync{
		Progress: proto.Uint32(100), Conversations: []*waHistorySync.Conversation{{
			ID: message.Key.RemoteJID, Messages: []*waHistorySync.HistorySyncMsg{{Message: message}},
		}},
	}}
	service.handleHistorySyncEvent(instance.ID, instance, event, map[webhookConfigEvent]bool{webhookConfigMessagesSet: true})
	select {
	case emitted := <-service.emitter:
		var envelope WookEvent[[]WookMessageData]
		if err := json.Unmarshal(mustMarshalAdWebhook(t, emitted.data), &envelope); err != nil {
			t.Fatal(err)
		}
		if emitted.url != instance.Webhook.Url || envelope.Event != WookMessagesSet || envelope.Instance != instance.ID || envelope.Progress == nil || *envelope.Progress != 100 || envelope.IsLatest == nil || !*envelope.IsLatest || envelope.Data == nil || len(*envelope.Data) != 1 {
			t.Fatal("history webhook envelope or destination changed")
		}
		data := &(*envelope.Data)[0]
		assertHistoricalAttribution(t, data)
		if data.InstanceId != instance.ID || data.Message.Conversation != "hello" {
			t.Fatal("history webhook lost the instance ID or message content")
		}
	default:
		t.Fatal("messages.set webhook was not queued")
	}
}

func TestHistoryMessagesWithoutAdDoNotInventAttribution(t *testing.T) {
	for _, ci := range []*waE2E.ContextInfo{nil, {CtwaPayload: []byte("opaque"), ConversionSource: proto.String("FB_Ads")}} {
		data := (&Whatsmiau{}).buildMessageDataFromHistory(historyMessageForAttribution(&waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("hello"), ContextInfo: ci},
		}), "", "")
		if data == nil || data.Referral != nil || data.CtwaClid != "" {
			t.Fatal("history invented attribution without an external ad")
		}
		if ci == nil && data.ContextInfo != nil {
			t.Fatal("history without context must keep its existing JSON shape")
		}
		if ci != nil && (data.ContextInfo == nil || data.ContextInfo.CtwaPayload != "b3BhcXVl" || data.ContextInfo.ConversionSource != "FB_Ads") {
			t.Fatal("history discarded conversion metadata without an external ad")
		}
	}
}
