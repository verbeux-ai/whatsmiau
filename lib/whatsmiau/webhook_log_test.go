package whatsmiau

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/models"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"
)

func TestMessageEventDebugLogRedactsAdMetadataAndPreservesWebhook(t *testing.T) {
	for _, withAd := range []bool{true, false} {
		name := "conversion only"
		if withAd {
			name = "external ad and referral"
		}
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			logger := zap.New(zapcore.NewCore(
				zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
				zapcore.AddSync(&output), zap.DebugLevel,
			))
			restore := zap.ReplaceGlobals(logger)
			defer restore()

			service := &Whatsmiau{clients: xsync.NewMap[string, *whatsmeow.Client](), emitter: make(chan emitter, 1)}
			service.clients.Store("ads-log", &whatsmeow.Client{})
			context := &waE2E.ContextInfo{
				ConversionSource: proto.String("FB_Ads"),
				ConversionData:   []byte("PRIVATE_CONVERSION"),
				CtwaPayload:      []byte("PRIVATE_CTWA"),
			}
			if withAd {
				context.ExternalAdReply = &waE2E.ContextInfo_ExternalAdReplyInfo{
					SourceType: proto.String("ad"), SourceID: proto.String("9007199254740993"),
					CtwaClid: proto.String("CLICK-001"), ContainsAutoReply: proto.Bool(false),
					GreetingMessageBody: proto.String("PRIVATE_GREETING"), CtaPayload: proto.String("PRIVATE_CTA"),
				}
			}
			message := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("hello"), ContextInfo: context}}
			jid := types.NewJID("5511000000000", types.DefaultUserServer)
			event := &events.Message{
				Message: message, RawMessage: message,
				Info: types.MessageInfo{
					MessageSource: types.MessageSource{Chat: jid, Sender: jid},
					ID:            "ADS-LOG", Timestamp: time.Unix(1700000000, 0),
				},
			}
			instance := &models.Instance{ID: "ads-log", Webhook: models.InstanceWebhook{Url: "https://example.invalid/hook"}}
			original := mustMarshalAdWebhook(t, service.convertEventMessage(instance.ID, instance, event))

			service.handleMessageEvent(instance.ID, instance, event, map[webhookConfigEvent]bool{webhookConfigMessagesUpsert: true})
			select {
			case emitted := <-service.emitter:
				webhook, ok := emitted.data.(*WookEvent[WookMessageData])
				if !ok || !bytes.Equal(original, mustMarshalAdWebhook(t, webhook.Data)) {
					t.Fatal("logging changed the data queued for webhook delivery")
				}
			default:
				t.Fatal("message was not queued for webhook delivery")
			}

			var logged map[string]interface{}
			for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n")) {
				var entry map[string]interface{}
				if err := json.Unmarshal(line, &entry); err != nil {
					t.Fatal(err)
				}
				if entry["msg"] == "message event" {
					logged, _ = entry["data"].(map[string]interface{})
				}
			}
			if logged == nil {
				t.Fatal("message debug log was not emitted")
			}
			var expected map[string]interface{}
			if err := json.Unmarshal(original, &expected); err != nil {
				t.Fatal(err)
			}
			logContext := expected["contextInfo"].(map[string]interface{})
			delete(logContext, "ctwaPayload")
			delete(logContext, "conversionData")
			if withAd {
				for _, ad := range []map[string]interface{}{logContext["externalAdReply"].(map[string]interface{}), expected["referral"].(map[string]interface{})} {
					delete(ad, "greetingMessageBody")
					delete(ad, "ctaPayload")
				}
			}
			if !reflect.DeepEqual(logged, expected) {
				t.Fatal("debug log must omit private ad metadata and preserve other fields")
			}
		})
	}
}

func TestMessageLogSanitizationPreservesOriginalMediaAndSeparateReferral(t *testing.T) {
	message := adWebhookForSizeTest(t).Data
	message.Message.Base64 = "PRIVATE_MEDIA"
	message.Referral.GreetingMessageBody = "PRIVATE_REFERRAL_GREETING"
	message.Referral.CtaPayload = "PRIVATE_REFERRAL_CTA"
	before := mustMarshalAdWebhook(t, message)

	logged := sanitizeMessageDataForLog(message)
	if logged.Message.Base64 != "" || logged.ContextInfo.CtwaPayload != "" || logged.ContextInfo.ConversionData != "" {
		t.Fatal("log copy retained media or conversion payloads")
	}
	for _, ad := range []*WookMessageContextInfoExternalAdReply{logged.ContextInfo.ExternalAdReply, logged.Referral} {
		if ad.GreetingMessageBody != "" || ad.CtaPayload != "" || ad.CtwaClid != "CLICK-001" {
			t.Fatal("log copy retained private ad content or lost the click identifier")
		}
	}
	if !bytes.Equal(before, mustMarshalAdWebhook(t, message)) {
		t.Fatal("log sanitization mutated the original media, context or referral")
	}
}

func TestMessageLogSanitizationHandlesAbsentFields(t *testing.T) {
	for _, message := range []*WookMessageData{
		nil,
		{},
		{Message: &WookMessageRaw{Conversation: "hello"}},
		{ContextInfo: &WookMessageContextInfo{}},
	} {
		before := mustMarshalAdWebhook(t, message)
		if !bytes.Equal(before, mustMarshalAdWebhook(t, sanitizeMessageDataForLog(message))) {
			t.Fatal("sanitization changed a message without private ad content or media bytes")
		}
	}
}
