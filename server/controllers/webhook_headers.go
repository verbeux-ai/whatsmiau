package controllers

import (
	"errors"
	"fmt"
	"golang.org/x/net/http/httpguts"
	"net/url"
	"strings"
)

const (
	maxWebhookHeaders           = 20
	maxWebhookHeaderNameLength  = 100
	maxWebhookHeaderValueLength = 2048
)

func validateWebhookHeaderTransport(rawURL string, headers map[string]string) error {
	if rawURL == "" || len(headers) == 0 {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return errors.New("webhook URLs with custom headers must use https")
	}
	return nil
}

func validateWebhookHeaders(headers map[string]string) error {
	if len(headers) > maxWebhookHeaders {
		return fmt.Errorf("too many webhook headers (max %d)", maxWebhookHeaders)
	}
	seen := make(map[string]struct{}, len(headers))
	for name, value := range headers {
		if !httpguts.ValidHeaderFieldName(name) {
			return fmt.Errorf("invalid webhook header name %q", name)
		}
		if !httpguts.ValidHeaderFieldValue(value) {
			return fmt.Errorf("invalid value for webhook header %q", name)
		}
		if len(name) > maxWebhookHeaderNameLength {
			return fmt.Errorf("webhook header name %q is too long (max %d)", name, maxWebhookHeaderNameLength)
		}
		if len(value) > maxWebhookHeaderValueLength {
			return fmt.Errorf("value for webhook header %q is too long (max %d)", name, maxWebhookHeaderValueLength)
		}

		canonical := strings.ToLower(name)
		if _, ok := seen[canonical]; ok {
			return fmt.Errorf("duplicate webhook header %q (names are case-insensitive)", canonical)
		}
		seen[canonical] = struct{}{}
	}
	return nil
}
