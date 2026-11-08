package whatsmiau

import (
	"encoding/json"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestMessageKeyFromMePreservesLegacyJSON(t *testing.T) {
	for _, fromMe := range []bool{false, true} {
		var key map[string]interface{}
		if err := json.Unmarshal(mustMarshalAdWebhook(t, &WookKey{Id: "message-id", FromMe: fromMe}), &key); err != nil {
			t.Fatal(err)
		}
		value, present := key["fromMe"]
		if present != fromMe || (fromMe && value != true) || key["id"] != "message-id" {
			t.Fatalf("legacy key JSON changed: fromMe=%v encoded=%v", fromMe, key)
		}
	}
}

func TestMessageUpsertPreservesLegacyClassificationAndAdMetadata(t *testing.T) {
	ad := historyAttributionContext()
	cases := []struct {
		name, wantType, bodyField string
		message                   *waE2E.Message
		wantAd                    bool
	}{
		{"ordinary text", "conversation", "conversation", &waE2E.Message{Conversation: proto.String("hello")}, false},
		{"empty text", "unknown", "", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{}}, false},
		{"empty text with quote", "unknown", "", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String("quoted-id")}}}, false},
		{"empty text with ad", "unknown", "", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{ContextInfo: ad}}, true},
		{"text with ad", "conversation", "conversation", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("hello"), ContextInfo: ad}}, true},
		{"image without ad", "imageMessage", "imageMessage", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: proto.String("photo")}}, false},
		{"image with ad", "conversation", "imageMessage", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: proto.String("photo"), ContextInfo: ad}}, true},
		{"video without ad", "videoMessage", "videoMessage", &waE2E.Message{VideoMessage: &waE2E.VideoMessage{}}, false},
		{"video with ad", "conversation", "videoMessage", &waE2E.Message{VideoMessage: &waE2E.VideoMessage{ContextInfo: ad}}, true},
		{"audio without ad", "audioMessage", "audioMessage", &waE2E.Message{AudioMessage: &waE2E.AudioMessage{}}, false},
		{"audio with ad", "conversation", "audioMessage", &waE2E.Message{AudioMessage: &waE2E.AudioMessage{ContextInfo: ad}}, true},
		{"document without ad", "documentMessage", "documentMessage", &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{}}, false},
		{"document with ad", "conversation", "documentMessage", &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{ContextInfo: ad}}, true},
		{"sticker without ad", "stickerMessage", "stickerMessage", &waE2E.Message{StickerMessage: &waE2E.StickerMessage{}}, false},
		{"sticker with ad", "conversation", "stickerMessage", &waE2E.Message{StickerMessage: &waE2E.StickerMessage{ContextInfo: ad}}, true},
		{"list response with ad", "listResponseMessage", "listResponseMessage", &waE2E.Message{ListResponseMessage: &waE2E.ListResponseMessage{ContextInfo: ad}}, true},
		{"button response with ad", "conversation", "buttonsResponseMessage", &waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{ContextInfo: ad, SelectedButtonID: proto.String("button-id")}}, true},
		{"interactive with ad", "conversation", "interactiveMessage", &waE2E.Message{InteractiveMessage: &waE2E.InteractiveMessage{ContextInfo: ad}}, true},
		{"empty text with encrypted comment", "encCommentMessage", "encCommentMessage", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{}, EncCommentMessage: &waE2E.EncCommentMessage{EncPayload: []byte{1}}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := convertAttributionTestMessage(t, tc.message)
			if data.MessageType != tc.wantType {
				t.Fatalf("messageType=%q, legacy type=%q", data.MessageType, tc.wantType)
			}
			var wire struct {
				Key     map[string]interface{} `json:"key"`
				Message map[string]interface{} `json:"message"`
			}
			if err := json.Unmarshal(mustMarshalAdWebhook(t, data), &wire); err != nil {
				t.Fatal(err)
			}
			if _, present := wire.Key["fromMe"]; present {
				t.Fatal("incoming upsert must preserve the omitted false fromMe field")
			}
			if tc.bodyField != "" && wire.Message[tc.bodyField] == nil {
				t.Fatalf("legacy classification removed the actual %s content", tc.bodyField)
			}
			if tc.wantAd {
				assertHistoricalAttribution(t, data)
			} else if data.Referral != nil || data.CtwaClid != "" {
				t.Fatal("ordinary message gained ad attribution")
			}
		})
	}
}
