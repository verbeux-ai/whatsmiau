package whatsmiau

import (
	"errors"
	"fmt"
	"testing"

	"github.com/puzpuzpuz/xsync/v4"
	"go.mau.fi/whatsmeow"
)

func TestPublicPairingError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"too short", whatsmeow.ErrPhoneNumberTooShort, "invalid_number"},
		{"not international", whatsmeow.ErrPhoneNumberIsNotInternational, "invalid_number"},
		{"wrapped", fmt.Errorf("pair: %w", whatsmeow.ErrPhoneNumberTooShort), "invalid_number"},
		{"whatsapp refused", errors.New("server returned error 400"), "unavailable"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := publicPairingError(test.err); got != test.want {
				t.Fatalf("publicPairingError = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAttemptAliveFollowsCachedQR(t *testing.T) {
	const id = "instance"
	service := &Whatsmiau{qrCache: xsync.NewMap[string, string]()}

	if service.attemptAlive(id) {
		t.Fatal("no QR cached: want the attempt reported as dead")
	}

	service.qrCache.Store(id, "qr")
	if !service.attemptAlive(id) {
		t.Fatal("QR cached: want the attempt reported as alive")
	}

	service.qrCache.Delete(id)
	if service.attemptAlive(id) {
		t.Fatal("QR cleared: want the attempt reported as dead")
	}
}

func TestPairingFailureStaysSilentWhenCodeExists(t *testing.T) {
	const id = "instance"
	service := &Whatsmiau{
		pairingCache:      xsync.NewMap[string, string](),
		pairingErrorCache: xsync.NewMap[string, string](),
	}

	if got := service.pairingFailure(id); got != "" {
		t.Fatalf("no attempt: got %q, want empty", got)
	}

	service.pairingErrorCache.Store(id, "invalid_number")
	if got := service.pairingFailure(id); got != "invalid_number" {
		t.Fatalf("failed attempt: got %q, want invalid_number", got)
	}

	service.pairingCache.Store(id, "ABCD-1234")
	if got := service.pairingFailure(id); got != "" {
		t.Fatalf("code present: got %q, want empty", got)
	}
}
