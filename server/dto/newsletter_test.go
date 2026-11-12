package dto

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestNewsletterRequestsRequireTheirKeys(t *testing.T) {
	validate := validator.New()
	invalid := map[string]any{
		"list without instance":   &NewsletterInstanceRequest{},
		"info without jid":        &NewsletterJidQuery{InstanceID: "inst"},
		"invite without code":     &NewsletterInviteQuery{InstanceID: "inst"},
		"create without name":     &CreateNewsletterRequest{InstanceID: "inst", Description: "x"},
		"create without instance": &CreateNewsletterRequest{Name: "Daily Deals"},
		"action without jid":      &NewsletterJidRequest{InstanceID: "inst"},
	}
	for name, request := range invalid {
		if err := validate.Struct(request); err == nil {
			t.Errorf("%s: validation passed, want error", name)
		}
	}

	valid := map[string]any{
		"list":   &NewsletterInstanceRequest{InstanceID: "inst"},
		"info":   &NewsletterJidQuery{InstanceID: "inst", NewsletterJid: "120363144038483540@newsletter"},
		"invite": &NewsletterInviteQuery{InstanceID: "inst", InviteCode: "0029Va4K0PZ5a245NkngBA2M"},
		"create": &CreateNewsletterRequest{InstanceID: "inst", Name: "Daily Deals"},
		"action": &NewsletterJidRequest{InstanceID: "inst", NewsletterJid: "120363144038483540@newsletter"},
	}
	for name, request := range valid {
		if err := validate.Struct(request); err != nil {
			t.Errorf("%s: unexpected validation error: %v", name, err)
		}
	}
}
