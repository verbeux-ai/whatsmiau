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
