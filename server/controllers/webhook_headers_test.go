package controllers

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidateWebhookHeaderTransport(t *testing.T) {
	headers := map[string]string{"Authorization": "Bearer x"}

	if err := validateWebhookHeaderTransport("https://example.com/hook", headers); err != nil {
		t.Fatalf("https destination must be accepted: %v", err)
	}
	if err := validateWebhookHeaderTransport("http://example.com/hook", headers); err == nil {
		t.Fatal("expected http with custom headers to be rejected")
	}
	if err := validateWebhookHeaderTransport("http://example.com/hook", nil); err != nil {
		t.Fatalf("header-less http webhooks must keep working: %v", err)
	}
	if err := validateWebhookHeaderTransport("", headers); err != nil {
		t.Fatalf("an empty url must not trip the rule: %v", err)
	}
}

func TestValidateWebhookHeaders(t *testing.T) {
	if err := validateWebhookHeaders(map[string]string{"X-Token": "abc"}); err != nil {
		t.Fatalf("valid headers rejected: %v", err)
	}
	if err := validateWebhookHeaders(map[string]string{"X Bad": "abc"}); err == nil {
		t.Fatal("expected an invalid header name to be rejected")
	}
	if err := validateWebhookHeaders(map[string]string{"X-Token": "bad\nvalue"}); err == nil {
		t.Fatal("expected a control character in the value to be rejected")
	}
	if err := validateWebhookHeaders(map[string]string{"Authorization": "a", "authorization": "b"}); err == nil {
		t.Fatal("expected case-insensitive duplicate names to be rejected")
	}
	if err := validateWebhookHeaders(map[string]string{"X-Token": strings.Repeat("v", maxWebhookHeaderValueLength+1)}); err == nil {
		t.Fatal("expected an oversized value to be rejected")
	}
	if err := validateWebhookHeaders(map[string]string{strings.Repeat("X", maxWebhookHeaderNameLength+1): "v"}); err == nil {
		t.Fatal("expected an oversized name to be rejected")
	}

	tooMany := make(map[string]string, maxWebhookHeaders+1)
	for i := 0; i <= maxWebhookHeaders; i++ {
		tooMany[fmt.Sprintf("X-Header-%d", i)] = "v"
	}
	if err := validateWebhookHeaders(tooMany); err == nil {
		t.Fatal("expected too many headers to be rejected")
	}
}

func TestMergeWebhookHeaders(t *testing.T) {
	stored := map[string]string{"Authorization": "Bearer old", "X-Keep": "1"}

	merged := mergeWebhookHeaders(stored, map[string]string{"Authorization": "Bearer new"})
	if merged["Authorization"] != "Bearer new" || merged["X-Keep"] != "1" {
		t.Fatalf("incoming headers must override stored ones while keeping the rest: %#v", merged)
	}
	if mergeWebhookHeaders(stored, nil)["X-Keep"] != "1" {
		t.Fatal("a request without headers must keep the stored ones")
	}
}
