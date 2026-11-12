package whatsmiau

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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
				CtwaSignals:      proto.String("PRIVATE_SIGNALS"),
			}
			if withAd {
				context.ExternalAdReply = &waE2E.ContextInfo_ExternalAdReplyInfo{
					SourceType: proto.String("ad"), SourceID: proto.String("9007199254740993"),
					CtwaClid: proto.String("CLICK-001"), ContainsAutoReply: proto.Bool(false),
					GreetingMessageBody: proto.String("PRIVATE_GREETING"), CtaPayload: proto.String("PRIVATE_CTA"),
					Title: proto.String("PRIVATE_TITLE"), Body: proto.String("PRIVATE_BODY"),
					Thumbnail: bytes.Repeat([]byte{255}, 512*1024), ThumbnailURL: proto.String("https://example.invalid/PRIVATE_THUMBNAIL"),
					MediaURL: proto.String("https://example.invalid/PRIVATE_MEDIA"), SourceURL: proto.String("https://example.invalid/PRIVATE_SOURCE"),
				}
				context.QuotedAd = &waE2E.ContextInfo_AdReplyInfo{
					AdvertiserName: proto.String("PRIVATE_ADVERTISER"), Caption: proto.String("PRIVATE_CAPTION"),
					JPEGThumbnail: bytes.Repeat([]byte{255}, 3*1024*1024), MediaType: waE2E.ContextInfo_AdReplyInfo_IMAGE.Enum(),
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
			if bytes.Contains(output.Bytes(), []byte("PRIVATE_")) || len(output.Bytes()) >= 8*1024 {
				t.Fatalf("debug log retained private ad content or grew too large: %d bytes", len(output.Bytes()))
			}
			var expected map[string]interface{}
			if err := json.Unmarshal(original, &expected); err != nil {
				t.Fatal(err)
			}
			logContext := expected["contextInfo"].(map[string]interface{})
			delete(logContext, "ctwaPayload")
			delete(logContext, "conversionData")
			delete(logContext, "ctwaSignals")
			if withAd {
				for _, ad := range []map[string]interface{}{logContext["externalAdReply"].(map[string]interface{}), expected["referral"].(map[string]interface{})} {
					for _, field := range []string{"greetingMessageBody", "ctaPayload", "title", "body", "thumbnail", "thumbnailUrl", "mediaUrl", "sourceUrl"} {
						delete(ad, field)
					}
				}
				logContext["quotedAd"] = map[string]interface{}{"mediaType": "IMAGE"}
			}
			if !reflect.DeepEqual(logged, expected) {
				t.Fatal("debug log must omit private ad metadata and preserve other fields")
			}
		})
	}
}

func TestMessageLogSanitizationBoundsAdMetadataWithoutMutatingWebhook(t *testing.T) {
	for _, large := range []string{strings.Repeat("x", 1024), strings.Repeat("€", 1024)} {
		message := adWebhookForSizeTest(t).Data
		message.CtwaClid = large
		for _, ad := range []*WookMessageContextInfoExternalAdReply{message.ContextInfo.ExternalAdReply, message.Referral} {
			ad.SourceType = large
			ad.SourceId = large
			ad.CtwaClid = large
			ad.MediaType = large
			ad.AdType = large
			ad.SourceApp = large
		}
		context := message.ContextInfo
		context.ConversionSource = large
		context.EntryPointConversionSource = large
		context.EntryPointConversionApp = large
		context.EntryPointConversionExternalSource = large
		context.EntryPointConversionExternalMedium = large
		context.SmbClientCampaignId = large
		context.SmbServerCampaignId = large
		context.Utm.UtmSource = large
		context.Utm.UtmCampaign = large
		context.QuotedAd.MediaType = large
		before := mustMarshalAdWebhook(t, message)

		logged := sanitizeMessageDataForLog(message)
		logContext := logged.ContextInfo
		fields := []string{
			logged.CtwaClid, logContext.ConversionSource, logContext.EntryPointConversionSource,
			logContext.EntryPointConversionApp, logContext.EntryPointConversionExternalSource,
			logContext.EntryPointConversionExternalMedium, logContext.SmbClientCampaignId,
			logContext.SmbServerCampaignId, logContext.Utm.UtmSource, logContext.Utm.UtmCampaign,
			logContext.QuotedAd.MediaType,
		}
		for _, ad := range []*WookMessageContextInfoExternalAdReply{logContext.ExternalAdReply, logged.Referral} {
			fields = append(fields, ad.SourceType, ad.SourceId, ad.CtwaClid, ad.MediaType, ad.AdType, ad.SourceApp)
		}
		for _, field := range fields {
			if len(field) > 256 || !utf8.ValidString(field) || !strings.HasSuffix(field, "...") {
				t.Fatalf("ad log field was not bounded safely: bytes=%d validUTF8=%v", len(field), utf8.ValidString(field))
			}
		}
		if !bytes.Equal(before, mustMarshalAdWebhook(t, message)) {
			t.Fatal("bounding ad log metadata changed the original webhook")
		}
	}
}

func TestMessageLogSanitizationKeepsOnlyApprovedAdFields(t *testing.T) {
	ad := &WookMessageContextInfoExternalAdReply{}
	fields := reflect.ValueOf(ad).Elem()
	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		switch field.Kind() {
		case reflect.String:
			field.SetString("ad-metadata")
		case reflect.Pointer:
			field.Set(reflect.New(field.Type().Elem()))
		}
	}
	allowed := map[string]bool{
		"sourceType": true, "sourceId": true, "ctwaClid": true, "sourceApp": true,
		"mediaType": true, "adType": true,
		"containsAutoReply": true, "renderLargerThumbnail": true, "showAdAttribution": true,
	}
	message := &WookMessageData{
		ContextInfo: &WookMessageContextInfo{ExternalAdReply: ad, QuotedAd: &WookMessageQuotedAd{
			AdvertiserName: "private advertiser", Caption: "private caption", JpegThumbnail: "private thumbnail", MediaType: "IMAGE",
		}},
		Referral: ad,
	}
	before := mustMarshalAdWebhook(t, message)
	logged := sanitizeMessageDataForLog(message)
	for _, sanitized := range []*WookMessageContextInfoExternalAdReply{logged.ContextInfo.ExternalAdReply, logged.Referral} {
		var actual map[string]interface{}
		if err := json.Unmarshal(mustMarshalAdWebhook(t, sanitized), &actual); err != nil {
			t.Fatal(err)
		}
		for field := range actual {
			if !allowed[field] {
				t.Errorf("unapproved ad field remains in debug log: %s", field)
			}
		}
		if len(actual) != len(allowed) {
			t.Errorf("lost approved ad log fields: got %d, want %d", len(actual), len(allowed))
		}
	}
	if got := string(mustMarshalAdWebhook(t, logged.ContextInfo.QuotedAd)); got != `{"mediaType":"IMAGE"}` {
		t.Fatalf("quoted ad retained content in debug log: %s", got)
	}
	if !bytes.Equal(before, mustMarshalAdWebhook(t, message)) {
		t.Fatal("ad log allowlist changed the original webhook")
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
