package whatsmiau

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestResolveExistingJIDPreservesLIDServer(t *testing.T) {
	fallback := types.NewJID("5511999999999", types.DefaultUserServer)
	lid := types.NewJID("123456789012345", types.HiddenUserServer)

	got := resolveExistingJID(fallback, []types.IsOnWhatsAppResponse{{
		Query:       fallback.User,
		JID:         lid,
		PhoneNumber: fallback,
		IsIn:        true,
	}})

	if got != lid {
		t.Fatalf("resolveExistingJID() = %s, want %s", got, lid)
	}
}

func TestResolveExistingJIDKeepsPhoneNumberServer(t *testing.T) {
	fallback := types.NewJID("5511999999999", types.DefaultUserServer)
	alternate := types.NewJID("551199999999", types.DefaultUserServer)

	got := resolveExistingJID(fallback, []types.IsOnWhatsAppResponse{{
		Query: alternate.User,
		JID:   alternate,
		IsIn:  true,
	}})

	if got != alternate {
		t.Fatalf("resolveExistingJID() = %s, want %s", got, alternate)
	}
}

func TestResolveExistingJIDRemovesDevicePart(t *testing.T) {
	fallback := types.NewJID("5511999999999", types.DefaultUserServer)
	deviceLID := types.NewADJID("123456789012345", types.LIDDomain, 2)
	want := deviceLID.ToNonAD()

	got := resolveExistingJID(fallback, []types.IsOnWhatsAppResponse{{
		JID:  deviceLID,
		IsIn: true,
	}})

	if got != want {
		t.Fatalf("resolveExistingJID() = %s, want %s", got, want)
	}
}

func TestResolveExistingJIDFallsBackWhenNoNumberExists(t *testing.T) {
	fallback := types.NewJID("5511999999999", types.DefaultUserServer)

	got := resolveExistingJID(fallback, []types.IsOnWhatsAppResponse{
		{
			JID:  types.NewJID("123456789012345", types.HiddenUserServer),
			IsIn: false,
		},
		{
			JID:  types.EmptyJID,
			IsIn: true,
		},
	})

	if got != fallback {
		t.Fatalf("resolveExistingJID() = %s, want fallback %s", got, fallback)
	}
}
