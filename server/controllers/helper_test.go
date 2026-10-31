package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"go.mau.fi/whatsmeow/types"
)

func TestNumberToJidAcceptsInternationalNumbers(t *testing.T) {
	validNumbers := []string{
		"5561999211277", // Brazil (13 digits)
		"13233923870",   // United States, +1 (11 digits)
		"34662418782",   // Spain, +34 (11 digits)
	}

	for _, number := range validNumbers {
		if _, err := numberToJid(number); err != nil {
			t.Errorf("numberToJid(%q) returned unexpected error: %v", number, err)
		}
	}
}

func TestNumberToJidRejectsTooShortNumbers(t *testing.T) {
	tooShort := []string{"123", "5561"}

	for _, number := range tooShort {
		if _, err := numberToJid(number); err == nil {
			t.Errorf("numberToJid(%q) expected an error, got nil", number)
		}
	}
}

func TestNumberToJidKeepsChannelJID(t *testing.T) {
	jid, err := numberToJid("120363144038483540@newsletter")
	if err != nil {
		t.Fatalf("numberToJid returned unexpected error: %v", err)
	}
	if jid.Server != types.NewsletterServer || jid.User != "120363144038483540" {
		t.Fatalf("expected the channel JID unchanged, got %s", jid)
	}
}

func TestSendFailStatusMapsChannelRejectionsToBadRequest(t *testing.T) {
	rejected := fmt.Errorf("list message: %w", whatsmiau.ErrNewsletterUnsupported)
	if got := sendFailStatus(rejected); got != http.StatusBadRequest {
		t.Fatalf("channel rejection status: got %d, want %d", got, http.StatusBadRequest)
	}
	if got := sendFailStatus(errors.New("server returned error 479")); got != http.StatusInternalServerError {
		t.Fatalf("send failure status: got %d, want %d", got, http.StatusInternalServerError)
	}
}

func TestMessageResponseKeyOmitsServerIDOutsideChannels(t *testing.T) {
	raw, err := json.Marshal(dto.MessageResponseKey{RemoteJid: "5511999999999", FromMe: true, Id: "ABC"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "serverId") {
		t.Fatalf("serverId must be omitted when unset, got %s", raw)
	}

	raw, err = json.Marshal(dto.MessageResponseKey{RemoteJid: "120363144038483540@newsletter", FromMe: true, Id: "ABC", ServerId: 105})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"serverId":105`) {
		t.Fatalf("expected serverId in a channel response, got %s", raw)
	}
}
