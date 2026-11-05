package whatsmiau

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/models"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

var webhookTestRedisOnce sync.Once

// useWebhookTestRedis points services.Redis at an in-memory server, since
// handleMessageEvent caches every poll it sees for vote decryption. The
// server lives for the whole test binary because services keeps the client.
func useWebhookTestRedis(t *testing.T) {
	t.Helper()
	webhookTestRedisOnce.Do(func() {
		server, err := miniredis.Run()
		if err != nil {
			t.Fatalf("start miniredis: %v", err)
		}
		env.Env.RedisURL = server.Addr()
		env.Env.RedisPassword = ""
		env.Env.RedisTLS = false
	})
}

// emitMessageJSON runs msg through handleMessageEvent as a post received from
// chat and returns the emitted messages.upsert data, decoded from its JSON.
func emitMessageJSON(t *testing.T, chat types.JID, msg *waE2E.Message) map[string]any {
	t.Helper()
	useWebhookTestRedis(t)

	service := &Whatsmiau{
		clients: xsync.NewMap[string, *whatsmeow.Client](),
		emitter: make(chan emitter, 1),
	}
	service.clients.Store("instance-1", &whatsmeow.Client{})
	instance := &models.Instance{
		ID: "instance-1",
		Webhook: models.InstanceWebhook{
			Url:    "https://webhook.example/messages",
			Events: []string{"MESSAGES_UPSERT"},
		},
	}
	event := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat, Sender: chat},
			ID:            "message-1",
			Timestamp:     time.Date(2026, time.November, 5, 12, 0, 0, 0, time.UTC),
		},
		// handleMessageEvent unwraps RawMessage, as whatsmeow delivers it.
		Message:    msg,
		RawMessage: msg,
	}

	service.handleMessageEvent("instance-1", instance, event, webhookEventMap(instance.Webhook.Events))

	var emitted emitter
	select {
	case emitted = <-service.emitter:
	case <-time.After(time.Second):
		t.Fatal("expected a messages.upsert webhook")
	}
	payload, ok := emitted.data.(*WookEvent[WookMessageData])
	if !ok {
		t.Fatalf("unexpected emitted data type %T", emitted.data)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	var decoded struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if decoded.Data == nil {
		t.Fatalf("payload has no data: %s", encoded)
	}
	return decoded.Data
}

func jsonObject(t *testing.T, v any, field string) map[string]any {
	t.Helper()
	obj, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%s: expected an object, got %#v", field, v)
	}
	return obj
}

func pollOptionNames(t *testing.T, poll map[string]any) []string {
	t.Helper()
	list, ok := poll["options"].([]any)
	if !ok {
		t.Fatalf("options: expected a list, got %#v", poll["options"])
	}
	names := make([]string, len(list))
	for i, item := range list {
		names[i], _ = jsonObject(t, item, "option")["optionName"].(string)
	}
	return names
}

func assertNames(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected options %q, got %q", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected options %q, got %q", want, got)
		}
	}
}

func TestChannelQuizWebhookKeepsOptionsAndCorrectAnswer(t *testing.T) {
	msg, err := buildNewsletterPoll("Which store has free shipping?", []string{"Store A", "Store B"}, 1, "Store B")
	if err != nil {
		t.Fatalf("build quiz: %v", err)
	}

	data := emitMessageJSON(t, testNewsletterJID, msg)

	if data["messageType"] != "pollCreationMessageV5" {
		t.Fatalf("expected messageType pollCreationMessageV5, got %#v", data["messageType"])
	}
	poll := jsonObject(t, jsonObject(t, data["message"], "message")["pollCreationMessage"], "pollCreationMessage")
	if poll["name"] != "Which store has free shipping?" {
		t.Fatalf("unexpected poll name %#v", poll["name"])
	}
	assertNames(t, pollOptionNames(t, poll), []string{"Store A", "Store B"})
	if poll["selectableOptionsCount"] != float64(1) {
		t.Fatalf("expected selectableOptionsCount 1, got %#v", poll["selectableOptionsCount"])
	}
	if poll["pollType"] != "QUIZ" {
		t.Fatalf("expected pollType QUIZ, got %#v", poll["pollType"])
	}
	answer := jsonObject(t, poll["correctAnswer"], "correctAnswer")
	if answer["optionName"] != "Store B" {
		t.Fatalf("expected correct answer Store B, got %#v", answer["optionName"])
	}
}

func TestChannelQuestionWebhookKeepsText(t *testing.T) {
	data := emitMessageJSON(t, testNewsletterJID, buildQuestionMessage("Which store should we cover next?"))

	if data["messageType"] != "questionMessage" {
		t.Fatalf("expected messageType questionMessage, got %#v", data["messageType"])
	}
	message := jsonObject(t, data["message"], "message")
	if message["conversation"] != "Which store should we cover next?" {
		t.Fatalf("expected question text, got %#v", message["conversation"])
	}
}

func TestPollWebhooksWithoutQuizFieldsAreUnchanged(t *testing.T) {
	group := types.NewJID("120363000000000001", types.GroupServer)
	poll := &waE2E.PollCreationMessage{
		Name: proto.String("Lunch?"),
		Options: []*waE2E.PollCreationMessage_Option{
			{OptionName: proto.String("Yes")},
			{OptionName: proto.String("No")},
		},
		SelectableOptionsCount: proto.Uint32(1),
	}
	channelPoll, err := buildNewsletterPoll("Lunch?", []string{"Yes", "No"}, 1, "")
	if err != nil {
		t.Fatalf("build poll: %v", err)
	}
	// A V5 poll without quiz data, as other clients may send it.
	bareV5 := &waE2E.Message{PollCreationMessageV5: &waE2E.PollCreationMessage{
		Name:    proto.String("Lunch?"),
		Options: poll.Options,
	}}

	cases := []struct {
		name        string
		chat        types.JID
		msg         *waE2E.Message
		messageType string
	}{
		{"v1", group, &waE2E.Message{PollCreationMessage: poll}, "pollCreationMessage"},
		{"v3", group, &waE2E.Message{PollCreationMessageV3: poll}, "pollCreationMessageV3"},
		{"channel v3", testNewsletterJID, channelPoll, "pollCreationMessageV3"},
		{"bare v5", testNewsletterJID, bareV5, "pollCreationMessageV5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := emitMessageJSON(t, tc.chat, tc.msg)
			if data["messageType"] != tc.messageType {
				t.Fatalf("expected messageType %s, got %#v", tc.messageType, data["messageType"])
			}
			got := jsonObject(t, jsonObject(t, data["message"], "message")["pollCreationMessage"], "pollCreationMessage")
			if got["name"] != "Lunch?" {
				t.Fatalf("unexpected poll name %#v", got["name"])
			}
			assertNames(t, pollOptionNames(t, got), []string{"Yes", "No"})
			for _, field := range []string{"pollType", "correctAnswer"} {
				if _, ok := got[field]; ok {
					t.Fatalf("%s must be omitted for a regular poll, got %#v", field, got[field])
				}
			}
		})
	}
}

func TestTextWebhookIsUnchanged(t *testing.T) {
	text := "hello"
	for name, msg := range map[string]*waE2E.Message{
		"conversation":  {Conversation: &text},
		"extended text": {ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: &text}},
	} {
		t.Run(name, func(t *testing.T) {
			data := emitMessageJSON(t, testNewsletterJID, msg)
			if data["messageType"] != "conversation" {
				t.Fatalf("expected messageType conversation, got %#v", data["messageType"])
			}
			if jsonObject(t, data["message"], "message")["conversation"] != "hello" {
				t.Fatalf("expected text hello, got %#v", data["message"])
			}
		})
	}
}
