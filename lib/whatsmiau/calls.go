package whatsmiau

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/purpshell/meowcaller"
	"github.com/verbeux-ai/whatsmiau/env"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

const callPCMFrameSamples = meowcaller.FrameSamples

// CallOffer identifies an outgoing direct-audio call.
type CallOffer struct{ ID, Recipient string }

// CallSession is a safe call lifecycle view. It intentionally excludes keys,
// relay data and encoded media.
type CallSession struct {
	ID, Peer, Direction, Media, State, Reason string
	CanAnswer, CanReject, CanHangup           bool
	CreatedAt, UpdatedAt                      time.Time
}

// CallAudioStream is a 16 kHz mono float32 PCM bridge for a single call.
type CallAudioStream struct {
	Receive <-chan []float32
	Push    func([]float32) error
	Close   func()
}

type callBridge struct {
	instanceID string
	call       *meowcaller.Call
	source     *livePCMSource
	audio      *pcmBroadcaster
	mu         sync.RWMutex
	session    CallSession
}

func (s *Whatsmiau) callsEnabled() bool {
	return env.Env.CallsEnabled && s.callClients != nil && s.callBridges != nil
}
func callBridgeKey(instanceID, callID string) string { return instanceID + "\x00" + callID }

func (s *Whatsmiau) registerCallClient(instanceID string, client *whatsmeow.Client) {
	if !s.callsEnabled() {
		return
	}
	callClient := meowcaller.NewClient(client)
	s.attachIncomingCallHandler(instanceID, callClient)
	s.callClients.Store(instanceID, callClient)
}

func (s *Whatsmiau) attachIncomingCallHandler(instanceID string, callClient *meowcaller.Client) {
	callClient.OnIncomingCall(func(call *meowcaller.Call) {
		s.callBridges.Store(callBridgeKey(instanceID, call.ID()), newCallBridge(instanceID, call, "incoming"))
	})
}

func (s *Whatsmiau) removeCallClient(instanceID string) {
	if s.callClients != nil {
		s.callClients.Delete(instanceID)
	}
	if s.callBridges == nil {
		return
	}
	s.callBridges.Range(func(key string, bridge *callBridge) bool {
		if bridge.instanceID == instanceID {
			_ = bridge.call.Hangup()
			s.callBridges.Delete(key)
		}
		return true
	})
}

func newCallBridge(instanceID string, call *meowcaller.Call, direction string) *callBridge {
	now := time.Now().UTC()
	b := &callBridge{instanceID: instanceID, call: call, source: newLivePCMSource(), audio: newPCMBroadcaster(), session: CallSession{ID: call.ID(), Peer: call.Peer().String(), Direction: direction, Media: "audio", State: callPhaseName(call.State()), CreatedAt: now, UpdatedAt: now}}
	call.Play(b.source)
	call.Receive(meowcaller.SinkFunc(func(frame []float32) { b.audio.publish(frame) }))
	call.OnStateChange(func(phase meowcaller.CallPhase) {
		b.update(func(view *CallSession) {
			view.State = callPhaseName(phase)
			view.CanAnswer = direction == "incoming" && phase == meowcaller.CallPhaseRinging
			view.CanReject = view.CanAnswer
			view.CanHangup = phase != meowcaller.CallPhaseEnded
		})
	})
	call.OnPeerAccept(func() { b.update(func(view *CallSession) { view.State = "connecting" }) })
	call.OnReady(func() { b.update(func(view *CallSession) { view.State = "active" }) })
	call.OnEnd(func(reason string) {
		b.update(func(view *CallSession) {
			view.State, view.Reason, view.CanAnswer, view.CanReject, view.CanHangup = "ended", reason, false, false, false
		})
		_ = b.source.Close()
		b.audio.close()
	})
	b.update(func(view *CallSession) {
		view.CanAnswer = direction == "incoming" && call.State() == meowcaller.CallPhaseRinging
		view.CanReject = view.CanAnswer
		view.CanHangup = call.State() != meowcaller.CallPhaseEnded
	})
	return b
}

func (b *callBridge) update(fn func(*CallSession)) {
	b.mu.Lock()
	fn(&b.session)
	b.session.UpdatedAt = time.Now().UTC()
	b.mu.Unlock()
}
func (b *callBridge) snapshot() CallSession { b.mu.RLock(); defer b.mu.RUnlock(); return b.session }
func callPhaseName(phase meowcaller.CallPhase) string {
	switch phase {
	case meowcaller.CallPhaseCalling:
		return "calling"
	case meowcaller.CallPhaseRinging:
		return "ringing"
	case meowcaller.CallPhaseConnecting:
		return "connecting"
	case meowcaller.CallPhaseActive:
		return "active"
	case meowcaller.CallPhaseEnded:
		return "ended"
	default:
		return "idle"
	}
}

// OfferAudioCall places an audio call. API consumers can attach media through
// OpenCallAudio after the call enters its media lifecycle.
func (s *Whatsmiau) OfferAudioCall(ctx context.Context, instanceID string, remoteJID *types.JID) (*CallOffer, error) {
	client, recipient, err := s.loadClientWithJID(ctx, instanceID, remoteJID)
	if err != nil {
		return nil, err
	}
	if !client.IsConnected() || !client.IsLoggedIn() {
		return nil, errors.New("instance is not connected")
	}
	callClient, ok := s.callClients.Load(instanceID)
	if !ok {
		return nil, errors.New("call support was not initialized; reconnect the instance")
	}
	call, err := callClient.Call(ctx, recipient.String())
	if err != nil {
		return nil, err
	}
	s.callBridges.Store(callBridgeKey(instanceID, call.ID()), newCallBridge(instanceID, call, "outgoing"))
	return &CallOffer{ID: call.ID(), Recipient: call.Peer().String()}, nil
}

func (s *Whatsmiau) ListCallSessions(instanceID string) ([]CallSession, error) {
	if _, ok := s.clients.Load(instanceID); !ok {
		return nil, whatsmeow.ErrClientIsNil
	}
	if !s.callsEnabled() {
		return nil, errors.New("call support is disabled")
	}
	result := make([]CallSession, 0)
	s.callBridges.Range(func(_ string, bridge *callBridge) bool {
		if bridge.instanceID == instanceID {
			result = append(result, bridge.snapshot())
		}
		return true
	})
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })
	return result, nil
}

func (s *Whatsmiau) loadCallBridge(instanceID, callID string) (*callBridge, error) {
	if !s.callsEnabled() {
		return nil, errors.New("call support is disabled")
	}
	bridge, ok := s.callBridges.Load(callBridgeKey(instanceID, callID))
	if !ok {
		return nil, errors.New("call session not found")
	}
	return bridge, nil
}
func (s *Whatsmiau) AnswerIncomingCall(instanceID, callID string) error {
	bridge, err := s.loadCallBridge(instanceID, callID)
	if err != nil {
		return err
	}
	if !bridge.snapshot().CanAnswer {
		return errors.New("call session cannot be answered")
	}
	return bridge.call.Answer()
}
func (s *Whatsmiau) RejectIncomingCall(_ context.Context, instanceID, callID string) error {
	bridge, err := s.loadCallBridge(instanceID, callID)
	if err != nil {
		return err
	}
	if !bridge.snapshot().CanReject {
		return errors.New("call session cannot be rejected")
	}
	return bridge.call.Reject()
}
func (s *Whatsmiau) HangupCall(instanceID, callID string) error {
	bridge, err := s.loadCallBridge(instanceID, callID)
	if err != nil {
		return err
	}
	if !bridge.snapshot().CanHangup {
		return errors.New("call session cannot be ended")
	}
	return bridge.call.Hangup()
}
func (s *Whatsmiau) OpenCallAudio(instanceID, callID string) (*CallAudioStream, error) {
	bridge, err := s.loadCallBridge(instanceID, callID)
	if err != nil {
		return nil, err
	}
	frames, closeSubscription := bridge.audio.subscribe()
	return &CallAudioStream{Receive: frames, Push: bridge.source.Push, Close: closeSubscription}, nil
}

type livePCMSource struct {
	mu     sync.Mutex
	frames chan []float32
	closed bool
}

func newLivePCMSource() *livePCMSource { return &livePCMSource{frames: make(chan []float32, 8)} }
func (s *livePCMSource) ReadFrame() ([]float32, error) {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return nil, io.EOF
	}
	select {
	case frame := <-s.frames:
		return frame, nil
	default:
		return make([]float32, callPCMFrameSamples), nil
	}
}
func (s *livePCMSource) Push(frame []float32) error {
	if len(frame) != callPCMFrameSamples {
		return fmt.Errorf("invalid pcm frame: got %d samples, want %d", len(frame), callPCMFrameSamples)
	}
	copyFrame := append([]float32(nil), frame...)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("call audio source is closed")
	}
	select {
	case s.frames <- copyFrame:
	default:
		select {
		case <-s.frames:
		default:
		}
		s.frames <- copyFrame
	}
	return nil
}
func (s *livePCMSource) Close() error { s.mu.Lock(); s.closed = true; s.mu.Unlock(); return nil }

type pcmBroadcaster struct {
	mu          sync.Mutex
	next        uint64
	subscribers map[uint64]chan []float32
	closed      bool
}

func newPCMBroadcaster() *pcmBroadcaster {
	return &pcmBroadcaster{subscribers: make(map[uint64]chan []float32)}
}
func (b *pcmBroadcaster) subscribe() (<-chan []float32, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	channel := make(chan []float32, 8)
	if b.closed {
		close(channel)
		return channel, func() {}
	}
	id := b.next
	b.next++
	b.subscribers[id] = channel
	return channel, func() {
		b.mu.Lock()
		if subscriber, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(subscriber)
		}
		b.mu.Unlock()
	}
}
func (b *pcmBroadcaster) publish(frame []float32) {
	if len(frame) != callPCMFrameSamples {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, subscriber := range b.subscribers {
		copyFrame := append([]float32(nil), frame...)
		select {
		case subscriber <- copyFrame:
		default:
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- copyFrame:
			default:
			}
		}
	}
}
func (b *pcmBroadcaster) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for id, subscriber := range b.subscribers {
		delete(b.subscribers, id)
		close(subscriber)
	}
}
