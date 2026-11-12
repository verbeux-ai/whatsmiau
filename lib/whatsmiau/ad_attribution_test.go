package whatsmiau

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/models"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestAdAttributionPreservesEveryProtocolAdField(t *testing.T) {
	ad := &waE2E.ContextInfo_ExternalAdReplyInfo{}
	message := ad.ProtoReflect()
	fields := message.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		var value protoreflect.Value
		switch field.Kind() {
		case protoreflect.StringKind:
			value = protoreflect.ValueOfString("ads-" + string(field.Name()))
		case protoreflect.BytesKind:
			value = protoreflect.ValueOfBytes([]byte("thumbnail"))
		case protoreflect.BoolKind:
			value = protoreflect.ValueOfBool(false)
		case protoreflect.Int32Kind:
			value = protoreflect.ValueOfInt32(0)
		case protoreflect.Uint32Kind:
			value = protoreflect.ValueOfUint32(0)
		case protoreflect.Uint64Kind:
			value = protoreflect.ValueOfUint64(0)
		case protoreflect.EnumKind:
			value = protoreflect.ValueOfEnum(field.Enum().Values().Get(1).Number())
		default:
			t.Fatalf("new ad field needs coverage: %s (%s)", field.Name(), field.Kind())
		}
		message.Set(field, value)
	}
	encoded, err := json.Marshal(convertExternalAdReply(ad))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != fields.Len() {
		t.Errorf("preserved %d of %d protocol ad fields: %s", len(got), fields.Len(), encoded)
	}
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		name := strings.ReplaceAll(strings.ReplaceAll(field.JSONName(), "URL", "Url"), "ID", "Id")
		value, exists := got[name]
		if !exists {
			t.Errorf("discarded ad field %s", name)
			continue
		}
		original := message.Get(field)
		var want interface{}
		switch field.Kind() {
		case protoreflect.StringKind:
			want = original.String()
		case protoreflect.BytesKind:
			want = base64.StdEncoding.EncodeToString(original.Bytes())
		case protoreflect.BoolKind:
			want = original.Bool()
		case protoreflect.EnumKind:
			want = string(field.Enum().Values().ByNumber(original.Enum()).Name())
		case protoreflect.Int32Kind:
			want = float64(original.Int())
		case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
			want = float64(original.Uint())
		}
		if !reflect.DeepEqual(value, want) {
			t.Errorf("changed %s: got %#v, want %#v", name, value, want)
		}
	}
	if got["sourceId"] != "ads-sourceID" || got["sourceUrl"] != "ads-sourceURL" || got["ctwaClid"] != "ads-ctwaClid" {
		t.Fatalf("Evolution API attribution names/values changed: %s", encoded)
	}
}

func TestAdAttributionSurvivesReceiveConversionForTextMediaAndWrappers(t *testing.T) {
	ci := &waE2E.ContextInfo{
		ExternalAdReply: &waE2E.ContextInfo_ExternalAdReplyInfo{
			SourceType: proto.String("ad"), SourceID: proto.String("9007199254740993"),
			CtwaClid: proto.String("click-id"), ContainsAutoReply: proto.Bool(false),
		},
		EntryPointConversionSource: proto.String("ctwa_ad"),
		Utm:                        &waE2E.ContextInfo_UTMInfo{UtmSource: proto.String("facebook"), UtmCampaign: proto.String("campaign")},
		SmbClientCampaignID:        proto.String("client-campaign"), SmbServerCampaignID: proto.String("server-campaign"),
		AlwaysShowAdAttribution: proto.Bool(false),
		ConversionDelaySeconds:  proto.Uint32(0), EntryPointConversionDelaySeconds: proto.Uint32(0),
		QuotedAd: &waE2E.ContextInfo_AdReplyInfo{AdvertiserName: proto.String("Advertiser"), Caption: proto.String("Ad caption")},
	}
	cases := []struct {
		name, wantType string
		message        *waE2E.Message
	}{
		{"text", "conversation", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("hello"), ContextInfo: ci}}},
		{"empty text", "unknown", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{ContextInfo: ci}}},
		{"list response", "listResponseMessage", &waE2E.Message{ListResponseMessage: &waE2E.ListResponseMessage{ContextInfo: ci}}},
		{"button response", "conversation", &waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{ContextInfo: ci}}},
		{"image", "conversation", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{ContextInfo: ci}}},
		{"video", "conversation", &waE2E.Message{VideoMessage: &waE2E.VideoMessage{ContextInfo: ci}}},
		{"audio", "conversation", &waE2E.Message{AudioMessage: &waE2E.AudioMessage{ContextInfo: ci}}},
		{"document", "conversation", &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{ContextInfo: ci}}},
		{"sticker", "conversation", &waE2E.Message{StickerMessage: &waE2E.StickerMessage{ContextInfo: ci}}},
		{"ephemeral", "conversation", &waE2E.Message{EphemeralMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("hello"), ContextInfo: ci}}}}},
		{"view once", "conversation", &waE2E.Message{ViewOnceMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{ImageMessage: &waE2E.ImageMessage{ContextInfo: ci}}}}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			data := convertAttributionTestMessage(t, tt.message)
			if data.MessageType != tt.wantType {
				t.Fatalf("ad webhook changed legacy classification: got %s, want %s", data.MessageType, tt.wantType)
			}
			if data.CtwaClid != "click-id" || data.Referral != data.ContextInfo.ExternalAdReply {
				t.Fatal("aliases did not preserve original attribution")
			}
			context := data.ContextInfo
			if context.Utm.UtmCampaign != "campaign" || context.QuotedAd.Caption != "Ad caption" || context.SmbClientCampaignId != "client-campaign" || context.SmbServerCampaignId != "server-campaign" || context.AlwaysShowAdAttribution == nil || *context.AlwaysShowAdAttribution {
				t.Fatalf("campaign context was lost: %+v", context)
			}
			wire, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]interface{}
			if err := json.Unmarshal(wire, &body); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(body["referral"], body["contextInfo"].(map[string]interface{})["externalAdReply"]) {
				t.Fatal("referral differs from externalAdReply in webhook JSON")
			}
			if _, exists := body["key"].(map[string]interface{})["fromMe"]; exists {
				t.Fatal("incoming messages must preserve the legacy omitted false fromMe field")
			}
			wireContext := body["contextInfo"].(map[string]interface{})
			if wireContext["conversionDelaySeconds"] != float64(0) || wireContext["entryPointConversionDelaySeconds"] != float64(0) {
				t.Fatal("explicit zero conversion delays were discarded")
			}
		})
	}
}

func TestAdAttributionDoesNotInventReferralOrClickID(t *testing.T) {
	conversion := &waE2E.ContextInfo{
		ConversionSource:                   proto.String("FB_Ads"),
		ConversionData:                     []byte("conversion-data"),
		CtwaPayload:                        []byte("opaque-payload"),
		CtwaSignals:                        proto.String("all,all"),
		EntryPointConversionSource:         proto.String("ctwa_ad"),
		EntryPointConversionApp:            proto.String("instagram"),
		EntryPointConversionExternalSource: proto.String("FB_Ads"),
		EntryPointConversionExternalMedium: proto.String("unavailable"),
	}
	for name, message := range map[string]*waE2E.Message{
		"organic": {Conversation: proto.String("hello")},
		"conversion only": {ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("hello"), ContextInfo: conversion,
		}},
	} {
		t.Run(name, func(t *testing.T) {
			data := convertAttributionTestMessage(t, message)
			wire, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]interface{}
			if err := json.Unmarshal(wire, &body); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"referral", "ctwaClid"} {
				if _, exists := body[key]; exists {
					t.Errorf("invented %s", key)
				}
			}
			context := body["contextInfo"].(map[string]interface{})
			if _, exists := context["externalAdReply"]; exists {
				t.Fatal("invented externalAdReply")
			}
			if name == "conversion only" {
				for key, want := range map[string]interface{}{
					"conversionSource":                   "FB_Ads",
					"conversionData":                     base64.StdEncoding.EncodeToString(conversion.ConversionData),
					"ctwaPayload":                        base64.StdEncoding.EncodeToString(conversion.CtwaPayload),
					"ctwaSignals":                        "all,all",
					"entryPointConversionSource":         "ctwa_ad",
					"entryPointConversionApp":            "instagram",
					"entryPointConversionExternalSource": "FB_Ads",
					"entryPointConversionExternalMedium": "unavailable",
				} {
					if !reflect.DeepEqual(context[key], want) {
						t.Errorf("%s=%v, want %v", key, context[key], want)
					}
				}
			}
		})
	}
}

const adAttributionContextJSON = `{
  "externalAdReply": {
    "sourceType": "ad",
    "sourceId": "9007199254740993",
    "sourceUrl": "https://example.invalid/ad",
    "mediaType": "IMAGE",
    "adType": "CTWA",
    "thumbnail": "AQID",
    "ctwaClid": "CLICK-001",
    "containsAutoReply": false,
    "automatedGreetingMessageShown": false,
    "greetingMessageBody": "PRIVATE_GREETING",
    "ctaPayload": "PRIVATE_CTA"
  },
  "ctwaPayload": "b3BhcXVl",
  "conversionData": "AwQ=",
  "conversionDelaySeconds": 0,
  "entryPointConversionDelaySeconds": 0,
  "entryPointConversionSource": "ctwa_ad",
  "alwaysShowAdAttribution": false,
  "utm": {
    "utmSource": "facebook",
    "utmCampaign": "campaign"
  },
  "smbClientCampaignId": "client-campaign",
  "smbServerCampaignId": "server-campaign",
  "quotedAd": {
    "advertiserName": "Advertiser",
    "caption": "Ad caption",
    "mediaType": "IMAGE",
    "jpegThumbnail": "AQID"
  }
}`

// Preserve the API's existing enum names and Base64 representation.
func TestAdAttributionPreservesExistingAPIContractForUpsert(t *testing.T) {
	var expected map[string]interface{}
	if err := json.Unmarshal([]byte(adAttributionContextJSON), &expected); err != nil {
		t.Fatal(err)
	}
	context := &waE2E.ContextInfo{
		ExternalAdReply: &waE2E.ContextInfo_ExternalAdReplyInfo{
			SourceType: proto.String("ad"), SourceID: proto.String("9007199254740993"),
			SourceURL: proto.String("https://example.invalid/ad"), CtwaClid: proto.String("CLICK-001"),
			MediaType: waE2E.ContextInfo_ExternalAdReplyInfo_MediaType(1).Enum(),
			AdType:    waE2E.ContextInfo_ExternalAdReplyInfo_AdType(0).Enum(),
			Thumbnail: []byte{1, 2, 3}, ContainsAutoReply: proto.Bool(false),
			AutomatedGreetingMessageShown: proto.Bool(false), GreetingMessageBody: proto.String("PRIVATE_GREETING"),
			CtaPayload: proto.String("PRIVATE_CTA"),
		},
		CtwaPayload: []byte("opaque"), ConversionData: []byte{3, 4},
		ConversionDelaySeconds: proto.Uint32(0), EntryPointConversionDelaySeconds: proto.Uint32(0),
		EntryPointConversionSource: proto.String("ctwa_ad"), AlwaysShowAdAttribution: proto.Bool(false),
		Utm:                 &waE2E.ContextInfo_UTMInfo{UtmSource: proto.String("facebook"), UtmCampaign: proto.String("campaign")},
		SmbClientCampaignID: proto.String("client-campaign"), SmbServerCampaignID: proto.String("server-campaign"),
		QuotedAd: &waE2E.ContextInfo_AdReplyInfo{
			AdvertiserName: proto.String("Advertiser"), Caption: proto.String("Ad caption"),
			MediaType: waE2E.ContextInfo_AdReplyInfo_MediaType(1).Enum(), JPEGThumbnail: []byte{1, 2, 3},
		},
		StanzaID: proto.String("quoted-id"), Participant: proto.String("5511999999999@s.whatsapp.net"),
		MentionedJID:  []string{"5511888888888@s.whatsapp.net"},
		QuotedMessage: &waE2E.Message{Conversation: proto.String("quoted text")},
	}
	text := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("hello"), ContextInfo: context}}
	for name, message := range map[string]*waE2E.Message{
		"plain":     text,
		"ephemeral": {EphemeralMessage: &waE2E.FutureProofMessage{Message: text}},
		"view once": {ViewOnceMessage: &waE2E.FutureProofMessage{Message: text}},
	} {
		t.Run(name, func(t *testing.T) {
			data := convertAttributionTestMessage(t, message)
			if data == nil || data.MessageType != "conversation" || data.Message.Conversation != "hello" {
				t.Fatalf("upsert lost message content: %#v", data)
			}
			encoded, err := json.Marshal(data.ContextInfo)
			if err != nil {
				t.Fatal(err)
			}
			var actual map[string]interface{}
			if err := json.Unmarshal(encoded, &actual); err != nil {
				t.Fatal(err)
			}
			for key, want := range expected {
				if !reflect.DeepEqual(actual[key], want) {
					t.Errorf("upsert %s=%#v, API contract=%#v", key, actual[key], want)
				}
			}
			if data.CtwaClid != "CLICK-001" || data.Referral == nil || data.Referral.Thumbnail != "" {
				t.Fatalf("upsert has incorrect compact attribution: %#v", data.Referral)
			}
			if data.ContextInfo.ExternalAdReply.Thumbnail == "" || context.ExternalAdReply.Thumbnail == nil {
				t.Fatal("compaction removed the canonical or original thumbnail")
			}
			if data.ContextInfo.StanzaId != "quoted-id" || data.ContextInfo.QuotedMessage.Conversation != "quoted text" || len(data.ContextInfo.MentionedJid) != 1 {
				t.Fatalf("upsert lost quotes or mentions: %#v", data.ContextInfo)
			}
		})
	}
}

func TestAdAttributionPreservesBase64Encoding(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		want string
	}{
		{"absent", nil, ""},
		{"empty", []byte{}, ""},
		{"bytes", []byte{1, 2, 3}, "AQID"},
		{"zero byte", []byte{0}, "AA=="},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := convertAttributionTestMessage(t, &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text: proto.String("hello"), ContextInfo: &waE2E.ContextInfo{CtwaPayload: tc.data, ConversionData: tc.data},
			}})
			encoded, err := json.Marshal(data.ContextInfo)
			if err != nil {
				t.Fatal(err)
			}
			var context map[string]interface{}
			if err := json.Unmarshal(encoded, &context); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"ctwaPayload", "conversionData"} {
				value, present := context[field]
				if tc.want == "" {
					if present {
						t.Errorf("empty %s should be omitted: %s", field, encoded)
					}
				} else if value != tc.want {
					t.Errorf("%s=%v, want Base64 string %q", field, value, tc.want)
				}
			}
		})
	}
}

func TestAdAttributionThumbnailKeepsLegacySizeAndEncoding(t *testing.T) {
	thumbnail := bytes.Repeat([]byte{255}, 512*1024)
	data := convertAttributionTestMessage(t, &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text: proto.String("hello"), ContextInfo: &waE2E.ContextInfo{ExternalAdReply: &waE2E.ContextInfo_ExternalAdReplyInfo{
			MediaType: waE2E.ContextInfo_ExternalAdReplyInfo_IMAGE.Enum(), Thumbnail: thumbnail,
			CtwaClid: proto.String("CLICK-001"), SourceType: proto.String("ad"),
		}},
	}})
	if data.ContextInfo.ExternalAdReply.MediaType != "IMAGE" || data.ContextInfo.ExternalAdReply.Thumbnail != base64.StdEncoding.EncodeToString(thumbnail) {
		t.Fatal("legacy thumbnail encoding changed")
	}
	if data.CtwaClid != "CLICK-001" || data.Referral == nil || data.Referral.CtwaClid != "CLICK-001" || data.Referral.Thumbnail != "" {
		t.Fatal("tracking was lost or thumbnail was duplicated")
	}
	wire, err := json.Marshal(WookEvent[WookMessageData]{Instance: "ads-test", Event: WookMessagesUpsert, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if len(wire) > 1024*1024 {
		t.Fatalf("512 KiB thumbnail expanded webhook beyond 1 MiB: %d bytes", len(wire))
	}
	t.Logf("512 KiB thumbnail produces %d-byte webhook with tracking preserved", len(wire))
}

func convertAttributionTestMessage(t *testing.T, message *waE2E.Message) *WookMessageData {
	t.Helper()
	service := &Whatsmiau{clients: xsync.NewMap[string, *whatsmeow.Client]()}
	service.clients.Store("ads-test", &whatsmeow.Client{})
	jid := types.NewJID("5511000000000", types.DefaultUserServer)
	data := service.convertEventMessage("ads-test", &models.Instance{}, &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: jid, Sender: jid},
			ID:            "ADS-CONTRACT", Timestamp: time.Unix(1700000000, 0),
		},
		Message: message, RawMessage: message,
	})
	if data == nil {
		t.Fatal("message conversion returned no data")
	}
	return data
}
