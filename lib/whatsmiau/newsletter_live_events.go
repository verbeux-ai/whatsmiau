package whatsmiau

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sort"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/verbeux-ai/whatsmiau/models"
	"github.com/verbeux-ai/whatsmiau/services"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// Channel reactions, poll votes and question responses are anonymous: the
// owner only receives running totals per post, in live updates. Each change
// to a total is emitted as a messages.upsert event carrying the new totals.
// The last totals seen are kept in Redis so that repeated snapshots are not
// emitted again.

const newsletterLiveStateTTL = 7 * 24 * time.Hour

// newsletterPostFetcher is the subset of *whatsmeow.Client used to look up a
// channel post (its message ID and poll options) by server ID.
type newsletterPostFetcher interface {
	GetNewsletterMessages(ctx context.Context, jid types.JID, params *whatsmeow.GetNewsletterMessagesParams) ([]*types.NewsletterMessage, error)
}

// newsletterPostInfo is what the events need to know about a channel post.
type newsletterPostInfo struct {
	ID      string   `json:"id"`
	Options []string `json:"options,omitempty"`
}

// newsletterCounts are the totals last seen for a channel post.
type newsletterCounts struct {
	Reactions map[string]int `json:"reactions,omitempty"`
	Votes     map[string]int `json:"votes,omitempty"`
	Responses int            `json:"responses,omitempty"`
}

func newsletterCountsKey(instanceID string, chat types.JID, serverID types.MessageServerID) string {
	return fmt.Sprintf("newsletter:live:%s:%s:%d", instanceID, chat.String(), serverID)
}

func newsletterPostKey(chat types.JID, serverID types.MessageServerID) string {
	return fmt.Sprintf("newsletter:post:%s:%d", chat.String(), serverID)
}

func (s *Whatsmiau) handleNewsletterLiveUpdate(id string, instance *models.Instance, e *events.NewsletterLiveUpdate, eventMap map[webhookConfigEvent]bool) {
	if !eventMap[webhookConfigMessagesUpsert] {
		return
	}
	client, ok := s.clients.Load(id)
	if !ok {
		return
	}
	for _, data := range newsletterLiveUpdateEvents(context.Background(), services.Redis(), client, id, e) {
		data.InstanceId = instance.ID
		s.emit(&WookEvent[WookMessageData]{
			Instance: instance.ID,
			Data:     data,
			DateTime: time.Unix(int64(data.MessageTimestamp), 0),
			Event:    WookMessagesUpsert,
		}, instance.Webhook)
	}
}

// newsletterLiveUpdateEvents compares a live update with the totals last seen
// and returns one message event per change: a reactionMessage per emoji whose
// count changed, a pollUpdateMessage when the votes changed and a
// questionResponseMessage when the number of responses changed.
func newsletterLiveUpdateEvents(ctx context.Context, rdb *redis.Client, fetcher newsletterPostFetcher, instanceID string, e *events.NewsletterLiveUpdate) []*WookMessageData {
	var out []*WookMessageData
	for _, msg := range e.Messages {
		if msg == nil || (msg.ReactionCounts == nil && msg.VoteCounts == nil && msg.ResponsesCount == 0) {
			continue // views and forwards only
		}

		countsKey := newsletterCountsKey(instanceID, e.JID, msg.MessageServerID)
		var previous newsletterCounts
		if raw, err := rdb.Get(ctx, countsKey).Bytes(); err == nil {
			_ = json.Unmarshal(raw, &previous)
		} else if !errors.Is(err, redis.Nil) {
			zap.L().Warn("failed to read channel totals", zap.String("key", countsKey), zap.Error(err))
		}

		current := previous
		if msg.ReactionCounts != nil {
			current.Reactions = msg.ReactionCounts
		}
		if msg.VoteCounts != nil {
			current.Votes = msg.VoteCounts
		}
		if msg.ResponsesCount > 0 {
			current.Responses = msg.ResponsesCount
		}

		changedReactions := changedKeys(previous.Reactions, current.Reactions)
		votesChanged := !maps.Equal(previous.Votes, current.Votes)
		responsesChanged := previous.Responses != current.Responses
		if len(changedReactions) == 0 && !votesChanged && !responsesChanged {
			continue
		}

		post := lookupNewsletterPost(ctx, rdb, fetcher, e.JID, msg.MessageServerID)
		target := &WookKey{RemoteJid: e.JID.String(), Id: post.ID, ServerId: int(msg.MessageServerID)}
		base := func(kind string) *WookMessageData {
			return &WookMessageData{
				Key: &WookKey{
					RemoteJid: e.JID.String(),
					Id:        fmt.Sprintf("%s.%d.%d", kind, msg.MessageServerID, e.Time.UnixMilli()),
				},
				Status:           "received",
				MessageTimestamp: int(e.Time.Unix()),
				Source:           "whatsapp",
			}
		}

		for _, code := range changedReactions {
			data := base("reaction")
			data.MessageType = "reactionMessage"
			data.Message = &WookMessageRaw{ReactionMessage: &ReactionMessageRaw{
				Key:            target,
				Text:           code,
				Count:          current.Reactions[code],
				ReactionCounts: current.Reactions,
			}}
			out = append(out, data)
		}
		if votesChanged {
			data := base("vote")
			data.MessageType = "pollUpdateMessage"
			data.Message = &WookMessageRaw{PollUpdateMessage: &WookPollUpdateMessageRaw{
				PollCreationMessageKey: target,
				VoteCounts:             pollVoteCounts(post.Options, current.Votes),
			}}
			out = append(out, data)
		}
		if responsesChanged {
			data := base("response")
			data.MessageType = "questionResponseMessage"
			data.Message = &WookMessageRaw{QuestionResponseMessage: &WookQuestionResponseMessageRaw{
				Key:            target,
				ResponsesCount: current.Responses,
			}}
			out = append(out, data)
		}

		if raw, err := json.Marshal(current); err == nil {
			if err := rdb.Set(ctx, countsKey, raw, newsletterLiveStateTTL).Err(); err != nil {
				zap.L().Warn("failed to store channel totals", zap.String("key", countsKey), zap.Error(err))
			}
		}
	}
	return out
}

// changedKeys returns, sorted, the keys whose value differs between a and b.
func changedKeys(a, b map[string]int) []string {
	var keys []string
	for k, v := range b {
		if a[k] != v {
			keys = append(keys, k)
		}
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// pollVoteCounts lists every option with its total, in the poll's order.
// Votes whose hash matches no known option are listed by hash only.
func pollVoteCounts(options []string, votes map[string]int) []WookPollVoteCount {
	counts := make([]WookPollVoteCount, 0, len(options))
	known := make(map[string]bool, len(options))
	for _, option := range options {
		sum := sha256.Sum256([]byte(option))
		hash := hex.EncodeToString(sum[:])
		known[hash] = true
		counts = append(counts, WookPollVoteCount{OptionName: option, OptionHash: hash, Count: votes[hash]})
	}
	var unknown []string
	for hash := range votes {
		if !known[hash] {
			unknown = append(unknown, hash)
		}
	}
	sort.Strings(unknown)
	for _, hash := range unknown {
		counts = append(counts, WookPollVoteCount{OptionHash: hash, Count: votes[hash]})
	}
	return counts
}

// lookupNewsletterPost returns the message ID and poll options of a channel
// post, fetching it once and caching it. On failure it returns what is known
// (possibly nothing): the events still carry the server ID.
func lookupNewsletterPost(ctx context.Context, rdb *redis.Client, fetcher newsletterPostFetcher, chat types.JID, serverID types.MessageServerID) newsletterPostInfo {
	key := newsletterPostKey(chat, serverID)
	var info newsletterPostInfo
	if raw, err := rdb.Get(ctx, key).Bytes(); err == nil && json.Unmarshal(raw, &info) == nil {
		return info
	}

	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	posts, err := fetcher.GetNewsletterMessages(callCtx, chat, &whatsmeow.GetNewsletterMessagesParams{Count: 1, Before: serverID + 1})
	if err != nil {
		zap.L().Warn("failed to fetch channel post", zap.Stringer("channel", chat), zap.Int("server_id", int(serverID)), zap.Error(err))
		return info
	}
	for _, post := range posts {
		if post == nil || post.MessageServerID != serverID {
			continue
		}
		info.ID = post.MessageID
		info.Options = pollOptionNames(post.Message)
	}
	if raw, err := json.Marshal(info); err == nil && info.ID != "" {
		_ = rdb.Set(ctx, key, raw, newsletterLiveStateTTL).Err()
	}
	return info
}

// rememberNewsletterPost caches a post sent by this instance, so its events
// need no lookup.
func rememberNewsletterPost(ctx context.Context, rdb *redis.Client, chat types.JID, serverID types.MessageServerID, id string, message *waE2E.Message) {
	if serverID == 0 || id == "" {
		return
	}
	raw, err := json.Marshal(newsletterPostInfo{ID: id, Options: pollOptionNames(message)})
	if err != nil {
		return
	}
	if err := rdb.Set(ctx, newsletterPostKey(chat, serverID), raw, newsletterLiveStateTTL).Err(); err != nil {
		zap.L().Warn("failed to cache channel post", zap.Stringer("channel", chat), zap.Error(err))
	}
}

func pollOptionNames(m *waE2E.Message) []string {
	for _, poll := range []*waE2E.PollCreationMessage{
		m.GetPollCreationMessage(), m.GetPollCreationMessageV2(), m.GetPollCreationMessageV3(),
		m.GetPollCreationMessageV5(), m.GetPollCreationMessageV6(),
	} {
		if poll == nil {
			continue
		}
		names := make([]string, 0, len(poll.GetOptions()))
		for _, option := range poll.GetOptions() {
			names = append(names, option.GetOptionName())
		}
		return names
	}
	return nil
}
