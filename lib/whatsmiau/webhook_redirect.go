package whatsmiau

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

var errWebhookRedirect = errors.New("refusing webhook redirect outside its original origin")
var errTooManyWebhookRedirects = errors.New("stopped after 10 redirects")

type webhookCredentialsContextKey struct{}

func rejectWebhookRedirect(req *http.Request, via []*http.Request) error {
	if req == nil || req.URL == nil || len(via) == 0 || via[0] == nil || via[0].URL == nil {
		return errWebhookRedirect
	}
	if len(via) >= 10 {
		return errTooManyWebhookRedirects
	}
	if !carriesWebhookCredentials(via[0]) {
		return nil
	}
	if !sameWebhookOrigin(via[0].URL, req.URL) {
		return errWebhookRedirect
	}
	return nil
}

func carriesWebhookCredentials(req *http.Request) bool {
	if req == nil {
		return false
	}
	marked, _ := req.Context().Value(webhookCredentialsContextKey{}).(bool)
	return marked
}

func sameWebhookOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) &&
		strings.EqualFold(a.Hostname(), b.Hostname()) &&
		webhookPort(a) == webhookPort(b)
}

func webhookPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	if strings.EqualFold(u.Scheme, "http") {
		return "80"
	}
	return ""
}

func isHTTPSWebhookURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	return err == nil && strings.EqualFold(u.Scheme, "https")
}
