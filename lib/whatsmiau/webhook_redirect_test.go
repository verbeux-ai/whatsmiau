package whatsmiau

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func parseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

// redirectScenario builds the (next, via) pair net/http hands to CheckRedirect:
// via[0] is the original delivery, next is where the redirect points.
func redirectScenario(t *testing.T, from, to string, credentials bool, hops int) (*http.Request, []*http.Request) {
	t.Helper()
	original, err := http.NewRequest(http.MethodPost, from, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if credentials {
		original = original.WithContext(context.WithValue(original.Context(), webhookCredentialsContextKey{}, true))
	}
	via := make([]*http.Request, hops)
	for i := range via {
		via[i] = original
	}
	return &http.Request{URL: parseURL(t, to)}, via
}

func TestRejectWebhookRedirect(t *testing.T) {
	t.Run("credentials stay inside the origin", func(t *testing.T) {
		next, via := redirectScenario(t, "https://example.com/hook", "https://example.com/hook/2", true, 1)
		if err := rejectWebhookRedirect(next, via); err != nil {
			t.Fatalf("same-origin redirect must be allowed: %v", err)
		}
	})

	t.Run("credentials blocked on another host", func(t *testing.T) {
		next, via := redirectScenario(t, "https://example.com/hook", "https://evil.example/hook", true, 1)
		if err := rejectWebhookRedirect(next, via); err != errWebhookRedirect {
			t.Fatalf("cross-host redirect must be refused, got %v", err)
		}
	})

	t.Run("credentials blocked on another port", func(t *testing.T) {
		next, via := redirectScenario(t, "https://example.com/hook", "https://example.com:8443/hook", true, 1)
		if err := rejectWebhookRedirect(next, via); err != errWebhookRedirect {
			t.Fatalf("cross-port redirect must be refused, got %v", err)
		}
	})

	t.Run("credentials blocked on https downgrade", func(t *testing.T) {
		next, via := redirectScenario(t, "https://example.com/hook", "http://example.com/hook", true, 1)
		if err := rejectWebhookRedirect(next, via); err != errWebhookRedirect {
			t.Fatalf("https-to-http redirect must be refused, got %v", err)
		}
	})

	t.Run("default ports are equivalent", func(t *testing.T) {
		next, via := redirectScenario(t, "https://example.com/hook", "https://example.com:443/hook", true, 1)
		if err := rejectWebhookRedirect(next, via); err != nil {
			t.Fatalf("explicit default port must match the implicit one: %v", err)
		}
	})

	t.Run("header-less webhooks keep following redirects", func(t *testing.T) {
		next, via := redirectScenario(t, "https://example.com/hook", "https://cdn.example/hook", false, 1)
		if err := rejectWebhookRedirect(next, via); err != nil {
			t.Fatalf("legacy redirect behavior must be preserved without headers: %v", err)
		}
	})

	t.Run("redirect cap", func(t *testing.T) {
		next, via := redirectScenario(t, "https://example.com/hook", "https://example.com/hook", false, 10)
		if err := rejectWebhookRedirect(next, via); err != errTooManyWebhookRedirects {
			t.Fatalf("expected the redirect cap, got %v", err)
		}
	})

	t.Run("malformed state is refused", func(t *testing.T) {
		if err := rejectWebhookRedirect(nil, nil); err != errWebhookRedirect {
			t.Fatalf("nil request must be refused, got %v", err)
		}
	})
}

func TestIsHTTPSWebhookURL(t *testing.T) {
	if !isHTTPSWebhookURL("https://example.com/hook") {
		t.Fatal("https url must be accepted")
	}
	if isHTTPSWebhookURL("http://example.com/hook") {
		t.Fatal("plain http url must be rejected")
	}
	if isHTTPSWebhookURL("://not a url") {
		t.Fatal("malformed url must be rejected")
	}
}

func webhookTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func webhookTLSTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	return server
}

// tlsWebhookClient trusts the httptest certificate and enforces the delivery
// redirect guard. Every httptest TLS server shares the same certificate, so one
// client trusts them all.
func tlsWebhookClient(server *httptest.Server) *http.Client {
	client := server.Client()
	client.CheckRedirect = rejectWebhookRedirect
	return client
}

func TestDoEmitRefusesCredentialsOverPlainHTTP(t *testing.T) {
	var hit bool
	server := webhookTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	})

	s := &Whatsmiau{webhookClient: &http.Client{CheckRedirect: rejectWebhookRedirect}}
	success, shouldRetry := s.doEmit([]byte(`{}`), server.URL, map[string]string{"X-Api-Key": "secret"})
	if success || shouldRetry {
		t.Fatalf("credentials over plain http must be refused without retry, got success=%v shouldRetry=%v", success, shouldRetry)
	}
	if hit {
		t.Fatal("a credentialed delivery must never reach a plain http endpoint")
	}
}

func TestDoEmitRefusesCredentialedCrossOriginRedirect(t *testing.T) {
	var targetHit bool
	target := webhookTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		targetHit = true
		w.WriteHeader(http.StatusOK)
	})

	redirector := webhookTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	})

	s := &Whatsmiau{webhookClient: tlsWebhookClient(redirector)}
	success, shouldRetry := s.doEmit([]byte(`{}`), redirector.URL, map[string]string{"X-Api-Key": "secret"})
	if success || shouldRetry {
		t.Fatalf("credentialed cross-origin redirect must fail without retry, got success=%v shouldRetry=%v", success, shouldRetry)
	}
	if targetHit {
		t.Fatal("credential must not be delivered to the redirect target")
	}
}

func TestDoEmitFollowsCrossOriginRedirectWithoutHeaders(t *testing.T) {
	var targetHit bool
	target := webhookTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		targetHit = true
		w.WriteHeader(http.StatusOK)
	})

	redirector := webhookTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	})

	s := &Whatsmiau{webhookClient: &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: rejectWebhookRedirect,
	}}

	success, shouldRetry := s.doEmit([]byte(`{}`), redirector.URL, nil)
	if !success || shouldRetry {
		t.Fatalf("header-less delivery must keep following redirects, got success=%v shouldRetry=%v", success, shouldRetry)
	}
	if !targetHit {
		t.Fatal("header-less delivery should have reached the redirect target")
	}
}

func TestDoEmitFollowsSameOriginRedirectWithHeaders(t *testing.T) {
	var received http.Header
	server := webhookTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/target" {
			received = r.Header.Clone()
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/target", http.StatusFound)
	})

	s := &Whatsmiau{webhookClient: tlsWebhookClient(server)}
	success, shouldRetry := s.doEmit([]byte(`{}`), server.URL+"/hook", map[string]string{"X-Api-Key": "secret"})
	if !success || shouldRetry {
		t.Fatalf("same-origin redirect must be allowed, got success=%v shouldRetry=%v", success, shouldRetry)
	}
	if got := received.Get("X-Api-Key"); got != "secret" {
		t.Fatalf("credential must survive a same-origin redirect: %q", got)
	}
}
