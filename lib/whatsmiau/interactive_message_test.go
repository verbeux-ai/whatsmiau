package whatsmiau

import (
	"encoding/json"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestParseWAMessageInteractiveNativeFlow(t *testing.T) {
	buttonParams := `{"display_text":"Open site","url":"https://example.test","merchant_url":"https://example.test"}`
	contextInfo := &waE2E.ContextInfo{
		StanzaID:    proto.String("quoted-message-id"),
		Participant: proto.String("123@s.whatsapp.net"),
	}
	message := &waE2E.Message{
		InteractiveMessage: &waE2E.InteractiveMessage{
			Header:      &waE2E.InteractiveMessage_Header{Title: proto.String("Visit our site")},
			Body:        &waE2E.InteractiveMessage_Body{Text: proto.String("Tap below to continue")},
			Footer:      &waE2E.InteractiveMessage_Footer{Text: proto.String("Powered by Whatsmiau")},
			ContextInfo: contextInfo,
			InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
				NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
					MessageVersion: proto.Int32(1),
					Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
						{
							Name:             proto.String("cta_url"),
							ButtonParamsJSON: proto.String(buttonParams),
						},
						{
							Name:             proto.String("cta_url"),
							ButtonParamsJSON: proto.String(`{"display_text":"Docs","url":"https://docs.example.test"}`),
						},
					},
				},
			},
		},
	}

	messageType, raw, gotContextInfo := (&Whatsmiau{}).parseWAMessage(message)
	if messageType != "interactiveMessage" {
		t.Fatalf("unexpected message type: got %q, want %q", messageType, "interactiveMessage")
	}
	if raw == nil || raw.InteractiveMessage == nil {
		t.Fatal("expected parsed interactive message")
	}
	if gotContextInfo != contextInfo {
		t.Fatal("expected interactive message context info to be preserved")
	}

	interactive := raw.InteractiveMessage
	if got := interactive.Header.Title; got != "Visit our site" {
		t.Fatalf("unexpected header title: %q", got)
	}
	if got := interactive.Body.Text; got != "Tap below to continue" {
		t.Fatalf("unexpected body text: %q", got)
	}
	if got := interactive.Footer.Text; got != "Powered by Whatsmiau" {
		t.Fatalf("unexpected footer text: %q", got)
	}
	if interactive.NativeFlowMessage == nil {
		t.Fatal("expected native flow message")
	}
	if got := interactive.NativeFlowMessage.MessageVersion; got != 1 {
		t.Fatalf("unexpected native flow message version: %d", got)
	}
	if got := len(interactive.NativeFlowMessage.Buttons); got != 2 {
		t.Fatalf("unexpected native flow button count: got %d, want 2", got)
	}
	if got := interactive.NativeFlowMessage.Buttons[0]; got.Name != "cta_url" || got.ButtonParamsJSON != buttonParams {
		t.Fatalf("first button was not preserved: %#v", got)
	}
	if got := interactive.NativeFlowMessage.Buttons[1]; got.Name != "cta_url" || got.ButtonParamsJSON != `{"display_text":"Docs","url":"https://docs.example.test"}` {
		t.Fatalf("second button was not preserved in order: %#v", got)
	}

	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal parsed interactive message: %v", err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode parsed interactive message JSON: %v", err)
	}
	var webhookInteractive struct {
		NativeFlowMessage struct {
			Buttons []struct {
				Name             string `json:"name"`
				ButtonParamsJSON string `json:"buttonParamsJson"`
			} `json:"buttons"`
		} `json:"nativeFlowMessage"`
	}
	if err := json.Unmarshal(payload["interactiveMessage"], &webhookInteractive); err != nil {
		t.Fatalf("decode webhook interactive message JSON: %v", err)
	}
	if got := webhookInteractive.NativeFlowMessage.Buttons; len(got) != 2 || got[0].ButtonParamsJSON != buttonParams {
		t.Fatalf("webhook JSON does not preserve button parameters: %#v", got)
	}
}

func TestParseWAMessageInteractiveWithoutNativeFlow(t *testing.T) {
	message := &waE2E.Message{
		InteractiveMessage: &waE2E.InteractiveMessage{
			Body: &waE2E.InteractiveMessage_Body{Text: proto.String("Interactive content")},
		},
	}

	messageType, raw, contextInfo := (&Whatsmiau{}).parseWAMessage(message)
	if messageType != "interactiveMessage" {
		t.Fatalf("unexpected message type: got %q, want %q", messageType, "interactiveMessage")
	}
	if raw == nil || raw.InteractiveMessage == nil {
		t.Fatal("expected parsed interactive message")
	}
	if got := raw.InteractiveMessage.Body.Text; got != "Interactive content" {
		t.Fatalf("unexpected body text: %q", got)
	}
	if raw.InteractiveMessage.NativeFlowMessage != nil {
		t.Fatal("did not expect a native flow message")
	}
	if contextInfo != nil {
		t.Fatal("did not expect context info")
	}
}
