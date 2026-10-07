package whatsmiau

import (
	"net/http"
	"testing"

	"github.com/verbeux-ai/whatsmiau/models"
)

func TestDoEmitAppliesConfiguredHeaders(t *testing.T) {
	var received http.Header
	server := webhookTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		received = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	})

	s := &Whatsmiau{webhookClient: tlsWebhookClient(server)}
	success, shouldRetry := s.doEmit([]byte(`{"event":"messages.upsert"}`), server.URL, map[string]string{
		"Authorization": "Bearer customer-token",
		"X-Tenant":      "acme",
	})

	if !success || shouldRetry {
		t.Fatalf("expected a successful delivery, got success=%v shouldRetry=%v", success, shouldRetry)
	}
	if got := received.Get("Authorization"); got != "Bearer customer-token" {
		t.Fatalf("authorization header not delivered: %q", got)
	}
	if got := received.Get("X-Tenant"); got != "acme" {
		t.Fatalf("custom header not delivered: %q", got)
	}
	if got := received.Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type must stay json, got %q", got)
	}
}

func TestEmitQueuesURLAndHeaders(t *testing.T) {
	s := &Whatsmiau{emitter: make(chan emitter, 1)}
	webhook := models.InstanceWebhook{
		Url:     "https://webhook.example/hook",
		Headers: map[string]string{"Authorization": "Bearer token"},
	}

	s.emit("payload", webhook)

	select {
	case emitted := <-s.emitter:
		if emitted.url != webhook.Url {
			t.Fatalf("unexpected url %q", emitted.url)
		}
		if emitted.headers["Authorization"] != "Bearer token" {
			t.Fatalf("headers not queued: %#v", emitted.headers)
		}
	default:
		t.Fatal("expected an emitted event")
	}
}
