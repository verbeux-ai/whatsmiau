package whatsmiau

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/models"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

type recoveryRoundTripper func(*http.Request) (*http.Response, error)

func (f recoveryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type recoveryDeviceContainer struct {
	store.NoopStore
	deletes int
	err     error
}

func (c *recoveryDeviceContainer) DeleteDevice(context.Context, *store.Device) error {
	c.deletes++
	return c.err
}

func recoveryDevice(user string) (*store.Device, *recoveryDeviceContainer) {
	jid := types.NewJID(user, types.DefaultUserServer)
	container := &recoveryDeviceContainer{}
	device := &store.Device{ID: &jid, Container: container}
	device.SetAllStores(&store.NoopStore{Error: errors.New("original store")})
	return device, container
}

func TestSavedSessionRetriesTransientInitialDialFailure(t *testing.T) {
	device, _ := recoveryDevice("5511999999000")
	client := newWhatsmeowClient(device, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer client.Disconnect()
	client.BackgroundEventCtx = ctx
	attempts := make(chan struct{}, 10)
	client.SetWebsocketHTTPClient(&http.Client{Transport: recoveryRoundTripper(func(req *http.Request) (*http.Response, error) {
		select {
		case attempts <- struct{}{}:
		default:
		}
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Status:     "503 Service Unavailable",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("temporary proxy failure")),
			Request:    req,
		}, nil
	})})
	if err := client.Connect(); err != nil {
		t.Fatalf("saved session should retry in the background: %v", err)
	}
	for range 2 {
		select {
		case <-attempts:
		case <-time.After(2 * time.Second):
			t.Fatal("the initial transient failure did not trigger another dial")
		}
	}
	if device.ID == nil || device.Deleted {
		t.Fatal("a temporary dial failure removed the saved session")
	}
}

func TestConnectRequestPreservesSavedSessionDuringBackgroundRetry(t *testing.T) {
	device, container := recoveryDevice("5511999999005")
	client := newWhatsmeowClient(device, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer client.Disconnect()
	client.BackgroundEventCtx = ctx
	client.SetWebsocketHTTPClient(&http.Client{Transport: recoveryRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("temporary proxy failure")),
			Request:    req,
		}, nil
	})})
	service := &Whatsmiau{
		clients:        xsync.NewMap[string, *whatsmeow.Client](),
		lockConnection: xsync.NewMap[string, *sync.Mutex](),
		instanceCache:  xsync.NewMap[string, models.Instance](),
	}
	service.clients.Store("instance", client)
	service.instanceCache.Store("instance", models.Instance{ID: "instance"})
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to start the saved-session retry: %v", err)
	}
	got, err := service.generateClient(ctx, "instance")
	if got != nil || !errors.Is(err, ErrAwaitingConnection) {
		t.Fatalf("expected a pending saved connection, got client=%p err=%v", got, err)
	}
	current, ok := service.clients.Load("instance")
	if !ok || current != client || device.ID == nil || device.Deleted || container.deletes != 0 {
		t.Fatal("a pending authentication was converted into a new pairing attempt")
	}
}

func TestFreshSessionDoesNotRetryInitialDialInBackground(t *testing.T) {
	device := &store.Device{}
	client := newWhatsmeowClient(device, nil)
	defer client.Disconnect()
	client.SetPreLoginHTTPClient(&http.Client{Transport: recoveryRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("temporary proxy failure")),
			Request:    req,
		}, nil
	})})
	if err := client.Connect(); err == nil {
		t.Fatal("a fresh pairing attempt must return its initial connection error")
	}
}

func TestDeleteDeviceInvalidatesSessionStores(t *testing.T) {
	device, container := recoveryDevice("5511999999001")
	client := newWhatsmeowClient(device, nil)
	service := &Whatsmiau{}
	if err := service.deleteDeviceIfExists(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	if container.deletes != 1 || device.ID != nil || !device.Deleted {
		t.Fatalf("session was not fully deleted: deletes=%d id=%v deleted=%v", container.deletes, device.ID, device.Deleted)
	}
	if err := device.Identities.PutIdentity(context.Background(), "peer", [32]byte{}); !errors.Is(err, store.ErrDeviceDeleted) {
		t.Fatalf("a late identity write should stop before reaching the database: %v", err)
	}
	if err := device.Save(context.Background()); !errors.Is(err, store.ErrDeviceDeleted) {
		t.Fatalf("a late save should not resurrect the removed device: %v", err)
	}
	if err := service.deleteDeviceIfExists(context.Background(), client); err != nil || container.deletes != 1 {
		t.Fatalf("repeated cleanup should be harmless: deletes=%d err=%v", container.deletes, err)
	}
}

func TestDeleteDeviceFailureKeepsSessionAvailableForRetry(t *testing.T) {
	device, container := recoveryDevice("5511999999002")
	want := errors.New("database unavailable")
	container.err = want
	client := newWhatsmeowClient(device, nil)
	if err := (&Whatsmiau{}).deleteDeviceIfExists(context.Background(), client); !errors.Is(err, want) {
		t.Fatalf("got %v, want the storage error", err)
	}
	if device.ID == nil || device.Deleted {
		t.Fatal("a failed deletion discarded the session before persistence succeeded")
	}
}

func TestLateLogoutDoesNotDeleteReplacementClient(t *testing.T) {
	oldDevice, oldContainer := recoveryDevice("5511999999003")
	newDevice, newContainer := recoveryDevice("5511999999004")
	oldClient := newWhatsmeowClient(oldDevice, nil)
	newClient := newWhatsmeowClient(newDevice, nil)
	service := &Whatsmiau{
		clients:        xsync.NewMap[string, *whatsmeow.Client](),
		lockConnection: xsync.NewMap[string, *sync.Mutex](),
	}
	service.clients.Store("instance", newClient)
	service.handleClientLoggedOut("instance", oldClient)
	current, ok := service.clients.Load("instance")
	if !ok || current != newClient || newDevice.Deleted || newContainer.deletes != 0 || oldContainer.deletes != 0 {
		t.Fatal("a delayed logout modified the replacement session")
	}
}
