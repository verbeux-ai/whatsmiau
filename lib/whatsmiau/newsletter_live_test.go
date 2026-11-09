package whatsmiau

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/models"
	"github.com/verbeux-ai/whatsmiau/services"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func optionHash(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])
}

func newLiveTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	server := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: server.Addr()})
}

type fakePostFetcher struct {
	posts map[types.MessageServerID]*types.NewsletterMessage
	err   error
	calls int
}

func (f *fakePostFetcher) GetNewsletterMessages(_ context.Context, _ types.JID, params *whatsmeow.GetNewsletterMessagesParams) ([]*types.NewsletterMessage, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if post, ok := f.posts[params.Before-1]; ok {
		return []*types.NewsletterMessage{post}, nil
	}
	return nil, nil
}

func liveUpdate(at time.Time, messages ...*types.NewsletterMessage) *events.NewsletterLiveUpdate {
	return &events.NewsletterLiveUpdate{JID: testNewsletterJID, Time: at, Messages: messages}
}

var liveTestTime = time.Date(2026, time.November, 9, 16, 13, 30, 0, time.UTC)

func TestChannelReactionEmitsReactionMessageOncePerChange(t *testing.T) {
	ctx := context.Background()
	rdb := newLiveTestRedis(t)
	fetcher := &fakePostFetcher{posts: map[types.MessageServerID]*types.NewsletterMessage{
		100: {MessageServerID: 100, MessageID: "3EB0AB141B977C02969E4D"},
	}}

	first := newsletterLiveUpdateEvents(ctx, rdb, fetcher, "instance", liveUpdate(liveTestTime,
		&types.NewsletterMessage{MessageServerID: 100, ViewsCount: 2, ReactionCounts: map[string]int{"😂": 1}}))
	if len(first) != 1 {
		t.Fatalf("expected 1 event, got %d", len(first))
	}
	data := first[0]
	if data.MessageType != "reactionMessage" {
		t.Fatalf("expected reactionMessage, got %q", data.MessageType)
	}
	reaction := data.Message.ReactionMessage
	if reaction.Text != "😂" || reaction.Count != 1 || reaction.ReactionCounts["😂"] != 1 {
		t.Fatalf("unexpected reaction %+v", reaction)
	}
	if reaction.Key.Id != "3EB0AB141B977C02969E4D" || reaction.Key.ServerId != 100 || reaction.Key.RemoteJid != testNewsletterJID.String() {
		t.Fatalf("unexpected target key %+v", reaction.Key)
	}
	if data.Key.RemoteJid != testNewsletterJID.String() || data.Key.Id == "" || data.MessageTimestamp != int(liveTestTime.Unix()) {
		t.Fatalf("unexpected event key %+v at %d", data.Key, data.MessageTimestamp)
	}

	// WhatsApp repeats the totals in later snapshots: nothing changed.
	repeat := newsletterLiveUpdateEvents(ctx, rdb, fetcher, "instance", liveUpdate(liveTestTime.Add(time.Minute),
		&types.NewsletterMessage{MessageServerID: 100, ViewsCount: 4, ReactionCounts: map[string]int{"😂": 1}}))
	if len(repeat) != 0 {
		t.Fatalf("expected no event for a repeated snapshot, got %d", len(repeat))
	}

	// A second emoji changes only that emoji.
	second := newsletterLiveUpdateEvents(ctx, rdb, fetcher, "instance", liveUpdate(liveTestTime.Add(2*time.Minute),
		&types.NewsletterMessage{MessageServerID: 100, ReactionCounts: map[string]int{"😂": 1, "🙏": 2}}))
	if len(second) != 1 || second[0].Message.ReactionMessage.Text != "🙏" || second[0].Message.ReactionMessage.Count != 2 {
		t.Fatalf("expected one 🙏 event with count 2, got %+v", second)
	}
	if fetcher.calls != 1 {
		t.Fatalf("expected the post to be fetched once, got %d", fetcher.calls)
	}
}

func TestChannelVotesEmitPollUpdateWithOptionTotals(t *testing.T) {
	ctx := context.Background()
	rdb := newLiveTestRedis(t)
	fetcher := &fakePostFetcher{}
	quiz := &waE2E.Message{PollCreationMessageV5: &waE2E.PollCreationMessage{
		Name: proto.String("Which store has free shipping?"),
		Options: []*waE2E.PollCreationMessage_Option{
			{OptionName: proto.String("Store A")}, {OptionName: proto.String("Store B")},
		},
	}}
	// Polls sent through the API are remembered, so votes need no lookup.
	rememberNewsletterPost(ctx, rdb, testNewsletterJID, 108, "3EB0434AD0A896356585F6", quiz)

	out := newsletterLiveUpdateEvents(ctx, rdb, fetcher, "instance", liveUpdate(liveTestTime,
		&types.NewsletterMessage{MessageServerID: 108, VoteCounts: map[string]int{optionHash("Store B"): 1}}))
	if len(out) != 1 || out[0].MessageType != "pollUpdateMessage" {
		t.Fatalf("expected one pollUpdateMessage, got %+v", out)
	}
	update := out[0].Message.PollUpdateMessage
	if update.PollCreationMessageKey.Id != "3EB0434AD0A896356585F6" || update.PollCreationMessageKey.ServerId != 108 {
		t.Fatalf("unexpected poll key %+v", update.PollCreationMessageKey)
	}
	want := []WookPollVoteCount{
		{OptionName: "Store A", OptionHash: optionHash("Store A"), Count: 0},
		{OptionName: "Store B", OptionHash: optionHash("Store B"), Count: 1},
	}
	if !slices.Equal(update.VoteCounts, want) {
		t.Fatalf("expected %+v, got %+v", want, update.VoteCounts)
	}
	if fetcher.calls != 0 {
		t.Fatalf("expected no lookup for a remembered poll, got %d", fetcher.calls)
	}

	// A changed vote emits the new totals.
	changed := newsletterLiveUpdateEvents(ctx, rdb, fetcher, "instance", liveUpdate(liveTestTime.Add(time.Minute),
		&types.NewsletterMessage{MessageServerID: 108, VoteCounts: map[string]int{optionHash("Store A"): 1, optionHash("Store B"): 1}}))
	if len(changed) != 1 || changed[0].Message.PollUpdateMessage.VoteCounts[0].Count != 1 {
		t.Fatalf("expected new totals, got %+v", changed)
	}
}

func TestChannelVotesOnPostsMadeElsewhereAreLookedUp(t *testing.T) {
	fetcher := &fakePostFetcher{posts: map[types.MessageServerID]*types.NewsletterMessage{
		105: {MessageServerID: 105, MessageID: "A50E923B8FEB46DD86D756141C51FC87", Message: &waE2E.Message{
			PollCreationMessageV3: &waE2E.PollCreationMessage{Options: []*waE2E.PollCreationMessage_Option{
				{OptionName: proto.String("Um")}, {OptionName: proto.String("Dois")},
			}},
		}},
	}}
	out := newsletterLiveUpdateEvents(context.Background(), newLiveTestRedis(t), fetcher, "instance", liveUpdate(liveTestTime,
		&types.NewsletterMessage{MessageServerID: 105, VoteCounts: map[string]int{optionHash("Um"): 1, "ff": 3}}))
	if len(out) != 1 {
		t.Fatalf("expected 1 event, got %d", len(out))
	}
	got := out[0].Message.PollUpdateMessage.VoteCounts
	want := []WookPollVoteCount{
		{OptionName: "Um", OptionHash: optionHash("Um"), Count: 1},
		{OptionName: "Dois", OptionHash: optionHash("Dois"), Count: 0},
		{OptionHash: "ff", Count: 3}, // unknown option, kept by hash
	}
	if !slices.Equal(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestChannelQuestionResponsesEmitResponseCount(t *testing.T) {
	ctx := context.Background()
	rdb := newLiveTestRedis(t)
	fetcher := &fakePostFetcher{posts: map[types.MessageServerID]*types.NewsletterMessage{
		109: {MessageServerID: 109, MessageID: "3EB093C7F39442D9C77E82"},
	}}
	out := newsletterLiveUpdateEvents(ctx, rdb, fetcher, "instance", liveUpdate(liveTestTime,
		&types.NewsletterMessage{MessageServerID: 109, ResponsesCount: 1, ViewsCount: 3}))
	if len(out) != 1 || out[0].MessageType != "questionResponseMessage" {
		t.Fatalf("expected one questionResponseMessage, got %+v", out)
	}
	response := out[0].Message.QuestionResponseMessage
	if response.ResponsesCount != 1 || response.Key.Id != "3EB093C7F39442D9C77E82" || response.Key.ServerId != 109 {
		t.Fatalf("unexpected response %+v", response)
	}

	// A snapshot without responses_count keeps the last total.
	views := newsletterLiveUpdateEvents(ctx, rdb, fetcher, "instance", liveUpdate(liveTestTime.Add(time.Minute),
		&types.NewsletterMessage{MessageServerID: 109, ViewsCount: 5}))
	if len(views) != 0 {
		t.Fatalf("expected no event for views only, got %+v", views)
	}
	next := newsletterLiveUpdateEvents(ctx, rdb, fetcher, "instance", liveUpdate(liveTestTime.Add(2*time.Minute),
		&types.NewsletterMessage{MessageServerID: 109, ResponsesCount: 2}))
	if len(next) != 1 || next[0].Message.QuestionResponseMessage.ResponsesCount != 2 {
		t.Fatalf("expected a count of 2, got %+v", next)
	}
}

func TestChannelEventsKeepServerIDWhenLookupFails(t *testing.T) {
	fetcher := &fakePostFetcher{err: errors.New("offline")}
	out := newsletterLiveUpdateEvents(context.Background(), newLiveTestRedis(t), fetcher, "instance", liveUpdate(liveTestTime,
		&types.NewsletterMessage{MessageServerID: 100, ReactionCounts: map[string]int{"👍": 1}}))
	if len(out) != 1 {
		t.Fatalf("expected 1 event, got %d", len(out))
	}
	if key := out[0].Message.ReactionMessage.Key; key.ServerId != 100 || key.Id != "" {
		t.Fatalf("expected the server ID without a message ID, got %+v", key)
	}
}

func TestChannelLiveUpdateWebhookJSON(t *testing.T) {
	useWebhookTestRedis(t)
	ctx := context.Background()
	rememberNewsletterPost(ctx, services.Redis(), testNewsletterJID, 107, "3EB08E1CF74864014ECD88", &waE2E.Message{
		PollCreationMessageV3: &waE2E.PollCreationMessage{Options: []*waE2E.PollCreationMessage_Option{
			{OptionName: proto.String("Loja A")}, {OptionName: proto.String("Loja B")},
		}},
	})

	service := &Whatsmiau{clients: xsync.NewMap[string, *whatsmeow.Client](), emitter: make(chan emitter, 4)}
	service.clients.Store("instance-json", &whatsmeow.Client{})
	instance := &models.Instance{ID: "instance-json", Webhook: models.InstanceWebhook{
		Url: "https://webhook.example/messages", Events: []string{"MESSAGES_UPSERT"},
	}}
	service.handleNewsletterLiveUpdate("instance-json", instance, liveUpdate(liveTestTime,
		&types.NewsletterMessage{MessageServerID: 107, VoteCounts: map[string]int{optionHash("Loja B"): 1}},
	), webhookEventMap(instance.Webhook.Events))

	var emitted emitter
	select {
	case emitted = <-service.emitter:
	case <-time.After(time.Second):
		t.Fatal("expected a messages.upsert webhook")
	}
	encoded, err := json.Marshal(emitted.data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload struct {
		Event string `json:"event"`
		Data  struct {
			MessageType string `json:"messageType"`
			InstanceId  string `json:"instanceId"`
			Message     struct {
				PollUpdateMessage struct {
					PollCreationMessageKey struct {
						RemoteJid string `json:"remoteJid"`
						Id        string `json:"id"`
						ServerId  int    `json:"serverId"`
					} `json:"pollCreationMessageKey"`
					VoteCounts []struct {
						OptionName string `json:"optionName"`
						Count      int    `json:"count"`
					} `json:"voteCounts"`
				} `json:"pollUpdateMessage"`
			} `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Event != string(WookMessagesUpsert) || payload.Data.MessageType != "pollUpdateMessage" || payload.Data.InstanceId != "instance-json" {
		t.Fatalf("unexpected payload: %s", encoded)
	}
	update := payload.Data.Message.PollUpdateMessage
	if update.PollCreationMessageKey.Id != "3EB08E1CF74864014ECD88" || update.PollCreationMessageKey.ServerId != 107 {
		t.Fatalf("unexpected poll key: %s", encoded)
	}
	if len(update.VoteCounts) != 2 || update.VoteCounts[1].OptionName != "Loja B" || update.VoteCounts[1].Count != 1 {
		t.Fatalf("unexpected vote counts: %s", encoded)
	}
}

type fakeLiveClient struct {
	list       []*types.NewsletterMetadata
	listErr    error
	ttl        time.Duration
	subscribed []types.JID
}

func (f *fakeLiveClient) GetSubscribedNewsletters(context.Context) ([]*types.NewsletterMetadata, error) {
	return f.list, f.listErr
}

func (f *fakeLiveClient) NewsletterSubscribeLiveUpdates(_ context.Context, jid types.JID) (time.Duration, error) {
	f.subscribed = append(f.subscribed, jid)
	return f.ttl, nil
}

func TestLiveUpdatesSubscribeOnlyToAdministeredChannels(t *testing.T) {
	channel := func(id string, role types.NewsletterRole) *types.NewsletterMetadata {
		return &types.NewsletterMetadata{
			ID:         types.NewJID(id, types.NewsletterServer),
			ViewerMeta: &types.NewsletterViewerMetadata{Role: role},
		}
	}
	client := &fakeLiveClient{ttl: 90 * time.Second, list: []*types.NewsletterMetadata{
		channel("1", types.NewsletterRoleOwner),
		channel("2", types.NewsletterRoleAdmin),
		channel("3", types.NewsletterRoleSubscriber),
		{ID: types.NewJID("4", types.NewsletterServer)},
	}}

	wait := subscribeAdminNewsletters(context.Background(), "instance", client)

	want := []types.JID{types.NewJID("1", types.NewsletterServer), types.NewJID("2", types.NewsletterServer)}
	if !slices.Equal(client.subscribed, want) {
		t.Fatalf("expected subscriptions %v, got %v", want, client.subscribed)
	}
	if wait != 72*time.Second {
		t.Fatalf("expected renewal before the 90s subscription expires (72s), got %v", wait)
	}
}

func TestLiveUpdatesRenewalBounds(t *testing.T) {
	failing := &fakeLiveClient{listErr: errors.New("offline")}
	if wait := subscribeAdminNewsletters(context.Background(), "instance", failing); wait != newsletterLiveMinRenew {
		t.Fatalf("expected a retry after %v, got %v", newsletterLiveMinRenew, wait)
	}
	none := &fakeLiveClient{}
	if wait := subscribeAdminNewsletters(context.Background(), "instance", none); wait != newsletterLiveDefaultTTL*4/5 {
		t.Fatalf("expected the default renewal, got %v", wait)
	}
}

func TestSentMessageTypes(t *testing.T) {
	group := types.NewJID("120363000000000001", types.GroupServer)
	cases := []struct{ got, want string }{
		{TextMessageType(false), "conversation"},
		{TextMessageType(true), "questionMessage"},
		{PollMessageType(group, ""), "pollCreationMessage"},
		{PollMessageType(testNewsletterJID, ""), "pollCreationMessageV3"},
		{PollMessageType(testNewsletterJID, "Store B"), "pollCreationMessageV5"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("expected %q, got %q", c.want, c.got)
		}
	}
}
