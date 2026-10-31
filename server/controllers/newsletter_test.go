package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

func TestParseNewsletterJIDAcceptsChannelIDs(t *testing.T) {
	for _, input := range []string{"120363144038483540@newsletter", "120363144038483540", " 120363144038483540@newsletter "} {
		jid, err := parseNewsletterJID(input)
		if err != nil {
			t.Fatalf("parseNewsletterJID(%q): %v", input, err)
		}
		if jid.String() != "120363144038483540@newsletter" {
			t.Fatalf("parseNewsletterJID(%q) = %s", input, jid)
		}
	}
}

func TestParseNewsletterJIDRejectsOtherJIDs(t *testing.T) {
	for _, input := range []string{
		"",
		"120363144038483540@g.us",
		"5511999999999@s.whatsapp.net",
		"status@broadcast",
		"@newsletter",
		"abc@newsletter",
		"0029Va4K0PZ5a245NkngBA2M",
		"https://whatsapp.com/channel/0029Va4K0PZ5a245NkngBA2M",
	} {
		if jid, err := parseNewsletterJID(input); err == nil {
			t.Errorf("parseNewsletterJID(%q) = %s, want error", input, jid)
		}
	}
}

func TestMapNewsletterError(t *testing.T) {
	graphQL := func(code int) error {
		// Mirrors how whatsmeow wraps GraphQL failures in sendMexIQ.
		return fmt.Errorf("graphql error: %w", types.GraphQLErrors{{Extensions: types.GraphQLErrorExtensions{ErrorCode: code}}})
	}
	cases := []struct {
		name string
		err  error
		code int
	}{
		{"client missing", whatsmeow.ErrClientIsNil, http.StatusNotFound},
		{"socket down", whatsmeow.ErrNotConnected, http.StatusNotFound},
		{"logged out", whatsmeow.ErrNotLoggedIn, http.StatusNotFound},
		{"metadata missing", fmt.Errorf("%w: x", whatsmiau.ErrNewsletterNotFound), http.StatusNotFound},
		{"iq not found", whatsmeow.ErrIQNotFound, http.StatusNotFound},
		{"graphql not found", graphQL(404), http.StatusNotFound},
		{"graphql bad request", graphQL(400), http.StatusBadRequest},
		{"graphql forbidden", graphQL(403), http.StatusForbidden},
		{"graphql unauthorized", graphQL(401), http.StatusForbidden},
		{"graphql rate limit", graphQL(429), http.StatusTooManyRequests},
		{"graphql unknown", graphQL(500), http.StatusInternalServerError},
		{"bad picture", fmt.Errorf("%w: not an image", whatsmiau.ErrInvalidNewsletterPicture), http.StatusBadRequest},
		{"iq forbidden", whatsmeow.ErrIQForbidden, http.StatusForbidden},
		{"iq rate limit", whatsmeow.ErrIQRateOverLimit, http.StatusTooManyRequests},
		{"unknown", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		if code, _ := mapNewsletterError(tc.err); code != tc.code {
			t.Errorf("%s: status = %d, want %d", tc.name, code, tc.code)
		}
	}
}

// newNewsletterTestApp wires the controller to a Whatsmiau without any
// connected client, so every lookup behaves like an offline instance.
func newNewsletterTestApp(t *testing.T) *echo.Echo {
	t.Helper()
	w := &whatsmiau.Whatsmiau{}
	setUnexportedField(t, w, "clients", reflect.ValueOf(xsync.NewMap[string, *whatsmeow.Client]()))
	controller := NewNewsletters(w)

	app := echo.New()
	group := app.Group("/v1/instance/:instance/newsletter")
	group.GET("/fetchAll", controller.FetchAll)
	group.GET("/info", controller.Info)
	group.GET("/inviteInfo", controller.InviteInfo)
	group.POST("/create", controller.Create)
	group.POST("/follow", controller.Follow)
	group.POST("/unfollow", controller.Unfollow)
	group.POST("/mute", controller.Mute)
	group.POST("/unmute", controller.Unmute)
	return app
}

func TestNewsletterControllerStatuses(t *testing.T) {
	app := newNewsletterTestApp(t)
	const base = "/v1/instance/inst/newsletter"
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		code   int
	}{
		{"list offline instance", http.MethodGet, base + "/fetchAll", "", http.StatusNotFound},
		{"info missing jid", http.MethodGet, base + "/info", "", http.StatusBadRequest},
		{"info group jid", http.MethodGet, base + "/info?newsletterJid=123@g.us", "", http.StatusBadRequest},
		{"info offline instance", http.MethodGet, base + "/info?newsletterJid=120363144038483540@newsletter", "", http.StatusNotFound},
		{"invite missing code", http.MethodGet, base + "/inviteInfo", "", http.StatusBadRequest},
		{"invite group link", http.MethodGet, base + "/inviteInfo?inviteCode=https://chat.whatsapp.com/abc", "", http.StatusBadRequest},
		{"invite offline instance", http.MethodGet, base + "/inviteInfo?inviteCode=https://www.whatsapp.com/channel/0029Va4K0PZ5a245NkngBA2M", "", http.StatusNotFound},
		{"create malformed body", http.MethodPost, base + "/create", `{"name":`, http.StatusUnprocessableEntity},
		{"create missing name", http.MethodPost, base + "/create", `{"description":"x"}`, http.StatusBadRequest},
		{"create blank name", http.MethodPost, base + "/create", `{"name":"   "}`, http.StatusBadRequest},
		{"create offline instance", http.MethodPost, base + "/create", `{"name":"Daily Deals"}`, http.StatusNotFound},
		{"follow missing jid", http.MethodPost, base + "/follow", `{}`, http.StatusBadRequest},
		{"follow user jid", http.MethodPost, base + "/follow", `{"newsletterJid":"5511999999999@s.whatsapp.net"}`, http.StatusBadRequest},
		{"follow offline instance", http.MethodPost, base + "/follow", `{"newsletterJid":"120363144038483540@newsletter"}`, http.StatusNotFound},
		{"unfollow offline instance", http.MethodPost, base + "/unfollow", `{"newsletterJid":"120363144038483540"}`, http.StatusNotFound},
		{"mute invalid jid", http.MethodPost, base + "/mute", `{"newsletterJid":"abc@newsletter"}`, http.StatusBadRequest},
		{"mute offline instance", http.MethodPost, base + "/mute", `{"newsletterJid":"120363144038483540@newsletter"}`, http.StatusNotFound},
		{"unmute offline instance", http.MethodPost, base + "/unmute", `{"newsletterJid":"120363144038483540@newsletter"}`, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			}
			recorder := httptest.NewRecorder()
			app.ServeHTTP(recorder, request)
			if recorder.Code != tc.code {
				t.Fatalf("status = %d, want %d (body=%s)", recorder.Code, tc.code, recorder.Body.String())
			}
		})
	}
}
