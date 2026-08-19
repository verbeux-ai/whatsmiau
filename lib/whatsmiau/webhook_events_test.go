package whatsmiau

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/models"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestWebhookEventMapNormalizesConfigurationAliases(t *testing.T) {
	tests := []struct {
		input    string
		expected webhookConfigEvent
	}{
		{input: "MESSAGES_UPSERT", expected: webhookConfigMessagesUpsert},
		{input: "messages.upsert", expected: webhookConfigMessagesUpsert},
		{input: "  Messages.Update  ", expected: webhookConfigMessagesUpdate},
		{input: "messages.delete", expected: webhookConfigMessagesDelete},
		{input: "messages.set", expected: webhookConfigMessagesSet},
		{input: "contacts.upsert", expected: webhookConfigContactsUpsert},
		{input: "group-participants.update", expected: webhookConfigGroupParticipantsUpdate},
		{input: "CONNECTION_UPDATE", expected: webhookConfigConnectionUpdate},
		{input: "connection.update", expected: webhookConfigConnectionUpdate},
		{input: "CALL", expected: webhookConfigCall},
		{input: "call", expected: webhookConfigCall},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			eventMap := webhookEventMap([]string{tt.input})
			if !eventMap[tt.expected] {
				t.Fatalf("expected normalized event %q in %#v", tt.expected, eventMap)
			}
		})
	}
}

func TestWebhookEventMapDoesNotEnableSupportedEventsForEmptyOrUnknownInput(t *testing.T) {
	supportedEvents := []webhookConfigEvent{
		webhookConfigMessagesUpsert,
		webhookConfigMessagesUpdate,
		webhookConfigMessagesDelete,
		webhookConfigMessagesSet,
		webhookConfigContactsUpsert,
		webhookConfigGroupParticipantsUpdate,
		webhookConfigConnectionUpdate,
		webhookConfigCall,
	}

	for _, input := range []string{"", "   ", "unknown.event"} {
		t.Run(input, func(t *testing.T) {
			eventMap := webhookEventMap([]string{input})
			for _, event := range supportedEvents {
				if eventMap[event] {
					t.Fatalf("input %q unexpectedly enabled supported event %q", input, event)
				}
			}
		})
	}
}

func TestWebhookPayloadEventIdentifiersRemainLowercase(t *testing.T) {
	tests := []struct {
		event    Wook
		expected string
	}{
		{event: WookMessagesUpsert, expected: "messages.upsert"},
		{event: WookMessagesUpdate, expected: "messages.update"},
		{event: WookMessagesDelete, expected: "messages.delete"},
		{event: WookMessagesSet, expected: "messages.set"},
		{event: WookContactsUpsert, expected: "contacts.upsert"},
		{event: WookGroupParticipantsUpdate, expected: "group-participants.update"},
		{event: WookConnectionUpdate, expected: "connection.update"},
		{event: WookCall, expected: "call"},
	}

	for _, tt := range tests {
		t.Run(string(tt.event), func(t *testing.T) {
			payload, err := json.Marshal(WookEvent[struct{}]{Event: tt.event})
			if err != nil {
				t.Fatalf("marshal webhook event: %v", err)
			}

			var envelope struct {
				Event string `json:"event"`
			}
			if err := json.Unmarshal(payload, &envelope); err != nil {
				t.Fatalf("unmarshal webhook event: %v", err)
			}
			if envelope.Event != tt.expected {
				t.Fatalf("expected event %q, got %q", tt.expected, envelope.Event)
			}
		})
	}
}

func TestMessageWebhookAcceptsCanonicalAndPayloadConfiguration(t *testing.T) {
	for _, configuredEvent := range []string{"MESSAGES_UPSERT", "messages.upsert"} {
		t.Run(configuredEvent, func(t *testing.T) {
			service := &Whatsmiau{
				clients: xsync.NewMap[string, *whatsmeow.Client](),
				emitter: make(chan emitter, 1),
			}
			service.clients.Store("instance-1", &whatsmeow.Client{})
			instance := &models.Instance{
				ID: "instance-1",
				Webhook: models.InstanceWebhook{
					Url:    "https://webhook.example/messages",
					Events: []string{configuredEvent},
				},
			}
			conversation := "hello"
			event := &events.Message{
				Info: types.MessageInfo{
					MessageSource: types.MessageSource{
						Chat:   types.NewJID("5511999999999", types.DefaultUserServer),
						Sender: types.NewJID("5511888888888", types.DefaultUserServer),
					},
					ID:        "message-1",
					Timestamp: time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC),
				},
				Message: &waE2E.Message{Conversation: &conversation},
			}

			service.handleMessageEvent("instance-1", instance, event, webhookEventMap(instance.Webhook.Events))

			select {
			case emitted := <-service.emitter:
				payload, ok := emitted.data.(*WookEvent[WookMessageData])
				if !ok {
					t.Fatalf("unexpected emitted data type %T", emitted.data)
				}
				if emitted.url != "https://webhook.example/messages" {
					t.Fatalf("unexpected webhook URL %q", emitted.url)
				}
				if payload.Event != WookMessagesUpsert {
					t.Fatalf("unexpected payload event %q", payload.Event)
				}
			case <-time.After(time.Second):
				t.Fatalf("expected message webhook for configuration %q", configuredEvent)
			}
		})
	}
}

func TestEmitConnectionUpdateAcceptsCanonicalAndPayloadConfiguration(t *testing.T) {
	for _, configuredEvent := range []string{"CONNECTION_UPDATE", "connection.update"} {
		t.Run(configuredEvent, func(t *testing.T) {
			enabled := true
			service := &Whatsmiau{
				instanceCache: xsync.NewMap[string, models.Instance](),
				emitter:       make(chan emitter, 1),
			}
			service.instanceCache.Store("instance-1", models.Instance{
				ID: "instance-1",
				Webhook: models.InstanceWebhook{
					Enabled: &enabled,
					Url:     "https://webhook.example/connection",
					Events:  []string{configuredEvent},
				},
			})

			service.emitConnectionUpdate("instance-1", "connecting", 0)

			select {
			case emitted := <-service.emitter:
				payload, ok := emitted.data.(*WookEvent[WookConnectionUpdateData])
				if !ok {
					t.Fatalf("unexpected emitted data type %T", emitted.data)
				}
				if emitted.url != "https://webhook.example/connection" {
					t.Fatalf("unexpected webhook URL %q", emitted.url)
				}
				if payload.Event != WookConnectionUpdate {
					t.Fatalf("unexpected payload event %q", payload.Event)
				}
			case <-time.After(time.Second):
				t.Fatalf("expected connection webhook for configuration %q", configuredEvent)
			}
		})
	}
}

func TestMessageDeleteWebhookUsesDeleteSubscription(t *testing.T) {
	for _, configuredEvent := range []string{"MESSAGES_DELETE", "messages.delete"} {
		t.Run(configuredEvent, func(t *testing.T) {
			service := &Whatsmiau{
				clients: xsync.NewMap[string, *whatsmeow.Client](),
				emitter: make(chan emitter, 1),
			}
			service.clients.Store("instance-1", &whatsmeow.Client{})
			instance := &models.Instance{
				ID: "instance-1",
				Webhook: models.InstanceWebhook{
					Url:    "https://webhook.example/messages",
					Events: []string{configuredEvent},
				},
			}
			revokeType := waE2E.ProtocolMessage_REVOKE
			messageID := "message-to-delete"
			remoteJID := "5511999999999@s.whatsapp.net"
			participant := "5511888888888@s.whatsapp.net"
			fromMe := false
			event := &events.Message{
				Info: types.MessageInfo{
					MessageSource: types.MessageSource{
						Chat:   types.NewJID("5511999999999", types.DefaultUserServer),
						Sender: types.NewJID("5511888888888", types.DefaultUserServer),
					},
				},
				Message: &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
					Type: &revokeType,
					Key: &waCommon.MessageKey{
						ID:          &messageID,
						RemoteJID:   &remoteJID,
						Participant: &participant,
						FromMe:      &fromMe,
					},
				}},
			}

			service.handleMessageEvent("instance-1", instance, event, webhookEventMap(instance.Webhook.Events))

			select {
			case emitted := <-service.emitter:
				payload, ok := emitted.data.(*WookEvent[WookMessageDeleteData])
				if !ok {
					t.Fatalf("unexpected emitted data type %T", emitted.data)
				}
				if payload.Event != WookMessagesDelete {
					t.Fatalf("unexpected payload event %q", payload.Event)
				}
				if payload.Data.Id != messageID || payload.Data.RemoteJid != remoteJID || payload.Data.Participant != participant {
					t.Fatalf("unexpected delete payload: %+v", payload.Data)
				}
				if payload.Data.Status != "DELETED" || payload.Data.InstanceId != "instance-1" || payload.Data.FromMe {
					t.Fatalf("unexpected delete metadata: %+v", payload.Data)
				}
			case <-time.After(time.Second):
				t.Fatalf("expected delete webhook for configuration %q", configuredEvent)
			}
		})
	}
}

func TestReceiptWebhookEmitsEachMessageUpdate(t *testing.T) {
	service := &Whatsmiau{
		clients: xsync.NewMap[string, *whatsmeow.Client](),
		emitter: make(chan emitter, 2),
	}
	service.clients.Store("instance-1", &whatsmeow.Client{})
	instance := &models.Instance{
		ID: "instance-1",
		Webhook: models.InstanceWebhook{
			Url:    "https://webhook.example/messages",
			Events: []string{"MESSAGES_UPDATE"},
		},
	}
	timestamp := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
	event := &events.Receipt{
		MessageSource: types.MessageSource{
			Chat:     types.NewJID("5511999999999", types.DefaultUserServer),
			Sender:   types.NewJID("5511888888888", types.DefaultUserServer),
			IsFromMe: true,
		},
		MessageIDs: []types.MessageID{"message-1", "message-2"},
		Timestamp:  timestamp,
		Type:       types.ReceiptTypeRead,
	}

	service.handleReceiptEvent("instance-1", instance, event, webhookEventMap(instance.Webhook.Events))

	for _, expectedID := range event.MessageIDs {
		select {
		case emitted := <-service.emitter:
			payload, ok := emitted.data.(*WookEvent[WookMessageUpdateData])
			if !ok {
				t.Fatalf("unexpected emitted data type %T", emitted.data)
			}
			if payload.Event != WookMessagesUpdate || !payload.DateTime.Equal(timestamp) {
				t.Fatalf("unexpected webhook envelope: %+v", payload)
			}
			if payload.Data.MessageId != expectedID || payload.Data.Status != MessageStatusRead || !payload.Data.FromMe {
				t.Fatalf("unexpected update payload: %+v", payload.Data)
			}
		case <-time.After(time.Second):
			t.Fatalf("expected update webhook for message %q", expectedID)
		}
	}
}

func TestWebhookSubscriptionsRemainIsolated(t *testing.T) {
	enabled := true
	service := &Whatsmiau{
		clients:       xsync.NewMap[string, *whatsmeow.Client](),
		instanceCache: xsync.NewMap[string, models.Instance](),
		emitter:       make(chan emitter, 1),
	}
	service.instanceCache.Store("instance-1", models.Instance{
		ID: "instance-1",
		Webhook: models.InstanceWebhook{
			Enabled: &enabled,
			Url:     "https://webhook.example/events",
			Events:  []string{"MESSAGES_UPSERT"},
		},
	})

	service.emitConnectionUpdate("instance-1", "connecting", 0)

	select {
	case emitted := <-service.emitter:
		t.Fatalf("message subscription unexpectedly emitted connection event: %#v", emitted)
	default:
	}

	conversation := "hello"
	message := &events.Message{
		Info: types.MessageInfo{MessageSource: types.MessageSource{
			Chat:   types.NewJID("5511999999999", types.DefaultUserServer),
			Sender: types.NewJID("5511888888888", types.DefaultUserServer),
		}},
		Message: &waE2E.Message{Conversation: &conversation},
	}
	instance := &models.Instance{
		ID: "instance-1",
		Webhook: models.InstanceWebhook{
			Url:    "https://webhook.example/events",
			Events: []string{"CONNECTION_UPDATE"},
		},
	}

	service.handleMessageEvent("instance-1", instance, message, webhookEventMap(instance.Webhook.Events))

	select {
	case emitted := <-service.emitter:
		t.Fatalf("connection subscription unexpectedly emitted message event: %#v", emitted)
	default:
	}
}
