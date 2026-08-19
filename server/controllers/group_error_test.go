package controllers

import (
	"net/http"
	"testing"

	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"go.mau.fi/whatsmeow"
)

func TestMapGroupErrorMapsWhatsAppBadRequest(t *testing.T) {
	code, message := mapGroupError(&whatsmeow.IQError{Code: 400, Text: "bad-request"})
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", code, http.StatusBadRequest)
	}
	if message != "WhatsApp rejected the group operation" {
		t.Fatalf("message = %q", message)
	}
}

func TestMapGroupErrorMapsGroupAddModeWithoutCommunity(t *testing.T) {
	code, message := mapGroupError(whatsmiau.ErrGroupAddModeRequiresCommunity)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", code, http.StatusBadRequest)
	}
	if message != "group add mode requires a community parent" {
		t.Fatalf("message = %q", message)
	}
}
