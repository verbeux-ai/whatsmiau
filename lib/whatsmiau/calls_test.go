package whatsmiau

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"unsafe"

	"github.com/purpshell/meowcaller"
	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/models"
	"go.mau.fi/whatsmeow"
)

func setCallPhase(t *testing.T, call *meowcaller.Call, phase meowcaller.CallPhase) {
	t.Helper()
	v := reflect.ValueOf(call).Elem().FieldByName("phase")
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Set(reflect.ValueOf(phase))
}

func setCallFlag(t *testing.T, call *meowcaller.Call, name string, value bool) {
	t.Helper()
	v := reflect.ValueOf(call).Elem().FieldByName(name)
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().SetBool(value)
}

func TestTrackCallBridgeDropsAlreadyEndedCall(t *testing.T) {
	s := &Whatsmiau{callBridges: xsync.NewMap[string, *callBridge]()}
	call := &meowcaller.Call{}
	setCallPhase(t, call, meowcaller.CallPhaseEnded)

	s.trackCallBridge("inst", call, "incoming")

	if _, ok := s.callBridges.Load(callBridgeKey("inst", call.ID())); ok {
		t.Fatal("ended call bridge retained in map")
	}
}

func TestTrackCallBridgeKeepsLiveCall(t *testing.T) {
	s := &Whatsmiau{callBridges: xsync.NewMap[string, *callBridge]()}
	call := &meowcaller.Call{}
	setCallPhase(t, call, meowcaller.CallPhaseRinging)

	s.trackCallBridge("inst", call, "incoming")

	if _, ok := s.callBridges.Load(callBridgeKey("inst", call.ID())); !ok {
		t.Fatal("live call bridge missing from map")
	}
}

func TestTrackCallBridgeDropsOutgoingCallAcceptedThenEnded(t *testing.T) {
	s := &Whatsmiau{callBridges: xsync.NewMap[string, *callBridge]()}
	call := &meowcaller.Call{}
	setCallPhase(t, call, meowcaller.CallPhaseEnded)
	setCallFlag(t, call, "peerAccepted", true)

	s.trackCallBridge("inst", call, "outgoing")

	if _, ok := s.callBridges.Load(callBridgeKey("inst", call.ID())); ok {
		t.Fatal("ended outgoing call bridge retained in map")
	}
}

// fireCallCallback invokes a callback the bridge registered on the call, simulating the
// library firing it from its media or stanza goroutine.
func fireCallCallback(t *testing.T, call *meowcaller.Call, field string) {
	t.Helper()
	v := reflect.ValueOf(call).Elem().FieldByName(field)
	fn := reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Interface().(func())
	fn()
}

// A working call is promoted to "active" by the first inbound RTP, which can happen before
// the peer's <accept> arrives. The accept must not regress the state: on an already active
// call it used to overwrite "active" with "connecting" and strand the session there.
func TestPeerAcceptAfterMediaKeepsActiveState(t *testing.T) {
	s := &Whatsmiau{callBridges: xsync.NewMap[string, *callBridge]()}
	call := &meowcaller.Call{}
	setCallPhase(t, call, meowcaller.CallPhaseConnecting)

	s.trackCallBridge("inst", call, "outgoing")
	b, ok := s.callBridges.Load(callBridgeKey("inst", call.ID()))
	if !ok {
		t.Fatal("call bridge missing from map")
	}

	// Inbound media first: the library advances the phase and fires OnReady.
	setCallPhase(t, call, meowcaller.CallPhaseActive)
	fireCallCallback(t, call, "onReady")
	if got := b.snapshot().State; got != "active" {
		t.Fatalf("after OnReady: state = %q, want %q", got, "active")
	}

	// The peer's <accept> lands afterwards.
	fireCallCallback(t, call, "onPeerAccept")
	if got := b.snapshot().State; got != "active" {
		t.Fatalf("peer accept after media: state = %q, want %q", got, "active")
	}
}

// The bridge teardown is what releases an attached WebSocket: it closes the
// source (so the call stops pushing) and the broadcaster (so every subscriber
// stops waiting). A path that skips it leaves the socket hanging.
func TestCallBridgeTeardownClosesSubscribers(t *testing.T) {
	b := &callBridge{source: newLivePCMSource(), audio: newPCMBroadcaster()}
	frames, unsubscribe := b.audio.subscribe()
	defer unsubscribe()

	b.teardown()

	select {
	case _, ok := <-frames:
		if ok {
			t.Fatal("subscriber channel still open after teardown")
		}
	default:
		t.Fatal("teardown did not close the subscriber channel")
	}
	if err := b.source.Push(make([]float32, callPCMFrameSamples)); err == nil {
		t.Fatal("source accepted a frame after teardown")
	}
}

// stubInstanceRepo lets the call tests control instance existence without Redis.
type stubInstanceRepo struct {
	instances []models.Instance
	err       error
}

func (stubInstanceRepo) Create(context.Context, *models.Instance) error { return nil }
func (r stubInstanceRepo) List(context.Context, string) ([]models.Instance, error) {
	return r.instances, r.err
}
func (stubInstanceRepo) Update(context.Context, string, *models.Instance) (*models.Instance, error) {
	return nil, nil
}
func (stubInstanceRepo) Delete(context.Context, string) error { return nil }

func enableCalls(t *testing.T) {
	t.Helper()
	previous := env.Env.CallsEnabled
	env.Env.CallsEnabled = true
	t.Cleanup(func() { env.Env.CallsEnabled = previous })
}

func newCallTestInstance(repo stubInstanceRepo) *Whatsmiau {
	return &Whatsmiau{
		clients:     xsync.NewMap[string, *whatsmeow.Client](),
		callClients: xsync.NewMap[string, *meowcaller.Client](),
		callBridges: xsync.NewMap[string, *callBridge](),
		repo:        repo,
	}
}

func TestListCallSessionsDistinguishesNotFoundFromEmpty(t *testing.T) {
	enableCalls(t)

	if _, err := newCallTestInstance(stubInstanceRepo{}).ListCallSessions(context.Background(), "missing"); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatalf("missing instance error = %v, want ErrInstanceNotFound", err)
	}

	sessions, err := newCallTestInstance(stubInstanceRepo{instances: []models.Instance{{ID: "inst"}}}).
		ListCallSessions(context.Background(), "inst")
	if err != nil {
		t.Fatalf("offline instance error = %v, want nil", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("offline instance sessions = %d, want 0", len(sessions))
	}
}

func TestCallOperationsReportSupportDisabled(t *testing.T) {
	previous := env.Env.CallsEnabled
	env.Env.CallsEnabled = false
	t.Cleanup(func() { env.Env.CallsEnabled = previous })

	s := newCallTestInstance(stubInstanceRepo{})
	if _, err := s.ListCallSessions(context.Background(), "inst"); !errors.Is(err, ErrCallSupportDisabled) {
		t.Fatalf("list error = %v, want ErrCallSupportDisabled", err)
	}
	if _, err := s.OfferAudioCall(context.Background(), "inst", nil); !errors.Is(err, ErrCallSupportDisabled) {
		t.Fatalf("offer error = %v, want ErrCallSupportDisabled", err)
	}
	if _, err := s.loadCallBridge("inst", "call"); !errors.Is(err, ErrCallSupportDisabled) {
		t.Fatalf("load bridge error = %v, want ErrCallSupportDisabled", err)
	}
}

func TestOfferAudioCallReportsNotFoundAndNotConnected(t *testing.T) {
	enableCalls(t)

	if _, err := newCallTestInstance(stubInstanceRepo{}).OfferAudioCall(context.Background(), "missing", nil); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatalf("missing instance error = %v, want ErrInstanceNotFound", err)
	}

	if _, err := newCallTestInstance(stubInstanceRepo{instances: []models.Instance{{ID: "inst"}}}).
		OfferAudioCall(context.Background(), "inst", nil); !errors.Is(err, ErrInstanceNotConnected) {
		t.Fatalf("offline instance error = %v, want ErrInstanceNotConnected", err)
	}
}

func TestLoadCallBridgeReportsSessionNotFound(t *testing.T) {
	enableCalls(t)
	if _, err := newCallTestInstance(stubInstanceRepo{}).loadCallBridge("inst", "call"); !errors.Is(err, ErrCallSessionNotFound) {
		t.Fatalf("bridge error = %v, want ErrCallSessionNotFound", err)
	}
}

func TestInstanceLookupFailureClassifiesAndKeepsCause(t *testing.T) {
	enableCalls(t)
	cause := errors.New("redis unavailable (simulated)")
	repo := stubInstanceRepo{err: cause}

	_, offerErr := newCallTestInstance(repo).OfferAudioCall(context.Background(), "inst", nil)
	if !errors.Is(offerErr, ErrInstanceLookupFailed) {
		t.Fatalf("offer error = %v, want ErrInstanceLookupFailed", offerErr)
	}
	if !errors.Is(offerErr, cause) {
		t.Fatalf("offer error = %v, want to preserve the repository cause", offerErr)
	}

	_, listErr := newCallTestInstance(repo).ListCallSessions(context.Background(), "inst")
	if !errors.Is(listErr, ErrInstanceLookupFailed) || !errors.Is(listErr, cause) {
		t.Fatalf("list error = %v, want a classified lookup failure wrapping the cause", listErr)
	}
}
