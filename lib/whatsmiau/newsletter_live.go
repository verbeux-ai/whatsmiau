package whatsmiau

import (
	"context"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
)

// WhatsApp only sends channel reaction and vote updates to clients that
// subscribed to the channel's live updates, and each subscription expires
// after a server-chosen duration. Instances keep a subscription to every
// channel they own or administer while connected.

const (
	// newsletterLiveDefaultTTL is used when the server returns no duration.
	newsletterLiveDefaultTTL = 5 * time.Minute
	// newsletterLiveMinRenew bounds renewals so a tiny duration cannot spin.
	newsletterLiveMinRenew = 30 * time.Second
)

// newsletterLiveClient is the subset of *whatsmeow.Client used to keep live
// update subscriptions.
type newsletterLiveClient interface {
	GetSubscribedNewsletters(ctx context.Context) ([]*types.NewsletterMetadata, error)
	NewsletterSubscribeLiveUpdates(ctx context.Context, jid types.JID) (time.Duration, error)
}

var newsletterLiveCancels sync.Map // instance id -> context.CancelFunc

// startNewsletterLiveUpdates (re)starts the subscription loop for an instance.
func (s *Whatsmiau) startNewsletterLiveUpdates(id string) {
	client, ok := s.clients.Load(id)
	if !ok {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	if previous, loaded := newsletterLiveCancels.Swap(id, cancel); loaded {
		previous.(context.CancelFunc)()
	}
	go runNewsletterLiveUpdates(ctx, id, client)
}

// stopNewsletterLiveUpdates stops the subscription loop for an instance.
func stopNewsletterLiveUpdates(id string) {
	if cancel, ok := newsletterLiveCancels.LoadAndDelete(id); ok {
		cancel.(context.CancelFunc)()
	}
}

func runNewsletterLiveUpdates(ctx context.Context, id string, client newsletterLiveClient) {
	for {
		wait := subscribeAdminNewsletters(ctx, id, client)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// subscribeAdminNewsletters subscribes to live updates of every channel the
// instance owns or administers and returns how long to wait before renewing.
func subscribeAdminNewsletters(ctx context.Context, id string, client newsletterLiveClient) time.Duration {
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	list, err := client.GetSubscribedNewsletters(callCtx)
	if err != nil {
		zap.L().Warn("failed to list channels for live updates", zap.String("instance", id), zap.Error(err))
		return newsletterLiveMinRenew
	}

	renew := newsletterLiveDefaultTTL
	for _, meta := range list {
		if meta == nil || meta.ViewerMeta == nil {
			continue
		}
		if role := meta.ViewerMeta.Role; role != types.NewsletterRoleOwner && role != types.NewsletterRoleAdmin {
			continue
		}
		ttl, err := client.NewsletterSubscribeLiveUpdates(callCtx, meta.ID)
		if err != nil {
			zap.L().Warn("failed to subscribe to channel live updates",
				zap.String("instance", id), zap.Stringer("channel", meta.ID), zap.Error(err))
			continue
		}
		if ttl > 0 && ttl < renew {
			renew = ttl
		}
	}
	return max(renew*4/5, newsletterLiveMinRenew)
}
