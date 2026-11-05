package controllers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/models"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

type pendingSessionTransport func(*http.Request) (*http.Response, error)

func (f pendingSessionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestConnectSavedSessionDuringProxyFailureReturnsPendingWithoutQR(t *testing.T) {
	for _, image := range []bool{false, true} {
		name := "json"
		if image {
			name = "qr-image"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			jid := types.NewJID("5511999999010", types.DefaultUserServer)
			device := &store.Device{ID: &jid}
			client := whatsmeow.NewClient(device, nil)
			defer client.Disconnect()
			client.BackgroundEventCtx = ctx
			client.SetWebsocketHTTPClient(&http.Client{Transport: pendingSessionTransport(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("proxy unavailable")),
					Request:    req,
				}, nil
			})})
			core := cachedInstance(t)
			clients := xsync.NewMap[string, *whatsmeow.Client]()
			clients.Store("inst", client)
			cache := xsync.NewMap[string, models.Instance]()
			cache.Store("inst", models.Instance{ID: "inst"})
			setUnexportedField(t, core, "clients", reflect.ValueOf(clients))
			setUnexportedField(t, core, "instanceCache", reflect.ValueOf(cache))
			setUnexportedField(t, core, "lockConnection", reflect.ValueOf(xsync.NewMap[string, *sync.Mutex]()))
			setUnexportedField(t, core, "connectPhoneNumber", reflect.ValueOf(xsync.NewMap[string, string]()))
			repo := newInMemoryInstanceRepo()
			if err := repo.Create(ctx, &models.Instance{ID: "inst"}); err != nil {
				t.Fatal(err)
			}
			controller := NewInstances(repo, core)
			app := echo.New()
			method, path := http.MethodPost, "/v1/instance/inst/connect"
			if image {
				method, path = http.MethodGet, "/v1/instance/connect/inst/image"
				app.GET("/v1/instance/connect/:id/image", controller.ConnectQRBuffer)
			} else {
				app.POST("/v1/instance/:id/connect", controller.Connect)
			}
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("pending saved session returned %d: %s", rec.Code, rec.Body.String())
			}
			if image {
				if rec.Body.Len() != 0 {
					t.Fatal("saved session reconnect should not produce another QR code")
				}
			} else {
				var body map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body["connected"] != false || body["base64"] != nil || body["message"] != "waiting for saved session connection" {
					t.Fatalf("unexpected pending response: %v", body)
				}
			}
			current, ok := clients.Load("inst")
			if !ok || current != client || device.ID == nil || device.Deleted {
				t.Fatal("pending connection discarded the original session")
			}
		})
	}
}
