package whatsmiau

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

const (
	// syncTimeout is how long to wait for the phone to respond per page.
	syncTimeout = 30 * time.Second
	syncDefaultCount = 50
	maxSyncMessages = 500
)

var ErrSyncTimeout = errors.New("timeout: phone did not respond in time")

type pendingSyncWaiter struct {
	chat   types.JID
	sendID string
	ch     chan *events.HistorySync
}

type SyncChatMessagesRequest struct {
	InstanceID string
	Chat       types.JID
	Count      int
	Since      *time.Time
	ID         string
	FromMe     *bool
}

func historySyncMatchesChat(evt *events.HistorySync, chat types.JID) bool {
	if evt == nil || evt.Data == nil {
		return false
	}
	for _, conv := range evt.Data.GetConversations() {
		if conv.GetID() == chat.String() {
			return true
		}
	}
	return false
}

func historySyncMatchesSend(evt *events.HistorySync, sendID string) bool {
	if evt == nil || evt.Notification == nil || sendID == "" {
		return true
	}
	respID := evt.Notification.GetPeerDataRequestSessionID()
	return respID == "" || respID == sendID
}

func (s *Whatsmiau) SyncChatMessages(ctx context.Context, req *SyncChatMessagesRequest) ([]WookMessageData, error) {
	syncMu, _ := s.syncLocks.LoadOrStore(req.InstanceID, &sync.Mutex{})
	syncMu.Lock()
	defer syncMu.Unlock()

	// Load client and check connection.
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}
	if !client.IsConnected() {
		return nil, errors.New("client not connected")
	}

	count := req.Count
	if count <= 0 {
		count = syncDefaultCount
	}

	chatJID := req.Chat
	if chatJID.Server == types.DefaultUserServer && client.Store != nil && client.Store.LIDs != nil {
		if lid, err := client.Store.LIDs.GetLIDForPN(ctx, chatJID); err == nil && !lid.IsEmpty() {
			chatJID = lid
		}
	}

	var allMessages []WookMessageData
	anchorTS := time.Now().Unix()

	for {
		onDemand := &waE2E.PeerDataOperationRequestMessage_HistorySyncOnDemandRequest{
			ChatJID:              proto.String(chatJID.String()),
			OnDemandMsgCount:     proto.Int32(int32(count)),
			OldestMsgTimestampMS: proto.Int64(anchorTS),
		}
		if req.ID != "" {
			onDemand.OldestMsgID = proto.String(req.ID)
			if req.FromMe != nil {
				onDemand.OldestMsgFromMe = proto.Bool(*req.FromMe)
			}
		}

		msg := &waE2E.Message{
			ProtocolMessage: &waE2E.ProtocolMessage{
				Type: waE2E.ProtocolMessage_PEER_DATA_OPERATION_REQUEST_MESSAGE.Enum(),
				PeerDataOperationRequestMessage: &waE2E.PeerDataOperationRequestMessage{
					PeerDataOperationRequestType: waE2E.PeerDataOperationRequestType_HISTORY_SYNC_ON_DEMAND.Enum(),
					HistorySyncOnDemandRequest:   onDemand,
				},
			},
		}

		resp, err := client.SendPeerMessage(ctx, msg)
		if err != nil {
			s.pendingSyncs.Delete(req.InstanceID)

			if len(allMessages) > 0 && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				return allMessages, nil
			}
			return nil, err
		}

		waiter := &pendingSyncWaiter{chat: chatJID, sendID: resp.ID, ch: make(chan *events.HistorySync, 1)}
		s.pendingSyncs.Store(req.InstanceID, waiter)

		// Wait for the phone to respond.
		var evt *events.HistorySync
		select {
		case evt = <-waiter.ch:
			s.pendingSyncs.Delete(req.InstanceID)
		case <-time.After(syncTimeout):
			s.pendingSyncs.Delete(req.InstanceID)

			if len(allMessages) > 0 {
				return allMessages, nil
			}
			return nil, ErrSyncTimeout
		case <-ctx.Done():
			s.pendingSyncs.Delete(req.InstanceID)
			if len(allMessages) > 0 {
				return allMessages, nil
			}
			return nil, ctx.Err()
		}

		var sinceTS int64
		if req.Since != nil {
			sinceTS = req.Since.Unix()
		}

		var oldestTS int64 = anchorTS
		oldestID := ""
		var oldestFromMe *bool
		pageHasMessages := false

	pageLoop:
		for _, conv := range evt.Data.GetConversations() {
			for _, hsm := range conv.GetMessages() {
				if hsm.Message == nil || hsm.Message.Message == nil {
					continue
				}

				ts := hsm.Message.GetMessageTimestamp()

				if sinceTS > 0 && int64(ts) < sinceTS {
					continue
				}

				md := s.buildMessageDataFromHistory(hsm.Message, conv.GetName(), conv.GetDisplayName())
				if md == nil {
					continue
				}
				md.InstanceId = req.InstanceID
				allMessages = append(allMessages, *md)
				pageHasMessages = true

				if ts > 0 && uint64(oldestTS) > ts {
					oldestTS = int64(ts)
					oldestID = hsm.Message.GetKey().GetID()
					fromMe := hsm.Message.GetKey().GetFromMe()
					oldestFromMe = &fromMe
				}

				if len(allMessages) >= maxSyncMessages {
					break pageLoop
				}
			}
		}

		// No more messages to fetch, or the per-request message cap was hit.
		if !pageHasMessages || len(allMessages) >= maxSyncMessages {
			return allMessages, nil
		}

		// If no Since filter, return after the first page.
		if req.Since == nil {
			return allMessages, nil
		}

		if oldestTS <= sinceTS {
			return allMessages, nil
		}

		// Prepare for next page: anchor becomes the oldest received message,
		// keeping its ID + fromMe so the phone recognizes the reference point.
		anchorTS = oldestTS
		if oldestID != "" {
			req.ID = oldestID
			req.FromMe = oldestFromMe
		}
	}
}