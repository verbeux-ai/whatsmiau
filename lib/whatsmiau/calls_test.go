package whatsmiau

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/purpshell/meowcaller"
	"github.com/puzpuzpuz/xsync/v4"
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
