package whatsmiau

import (
	"context"
	"testing"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/models"
	"go.mau.fi/whatsmeow"
)

func TestLogoutUnlockedClearsRuntimeStateWithoutClient(t *testing.T) {
	const id = "missing-client"
	service := &Whatsmiau{
		clients:            xsync.NewMap[string, *whatsmeow.Client](),
		qrCache:            xsync.NewMap[string, string](),
		pairingCache:       xsync.NewMap[string, string](),
		observerRunning:    xsync.NewMap[string, *whatsmeow.Client](),
		instanceCache:      xsync.NewMap[string, models.Instance](),
		connectPhoneNumber: xsync.NewMap[string, string](),
	}

	service.qrCache.Store(id, "qr")
	service.pairingCache.Store(id, "pairing")
	service.observerRunning.Store(id, nil)
	service.instanceCache.Store(id, models.Instance{ID: id})
	service.connectPhoneNumber.Store(id, "5511999999999")

	if err := service.logoutUnlocked(context.Background(), id); err != nil {
		t.Fatalf("logoutUnlocked: %v", err)
	}

	if _, ok := service.qrCache.Load(id); ok {
		t.Fatal("QR cache was not cleared")
	}
	if _, ok := service.pairingCache.Load(id); ok {
		t.Fatal("pairing cache was not cleared")
	}
	if _, ok := service.observerRunning.Load(id); ok {
		t.Fatal("observer marker was not cleared")
	}
	if _, ok := service.instanceCache.Load(id); ok {
		t.Fatal("instance cache was not cleared")
	}
	if _, ok := service.connectPhoneNumber.Load(id); ok {
		t.Fatal("connect phone number was not cleared")
	}
}
