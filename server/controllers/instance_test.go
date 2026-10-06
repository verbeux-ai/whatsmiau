package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/interfaces"
)

func serveInstanceCreate(t *testing.T, repo interfaces.InstanceRepository, body string) *httptest.ResponseRecorder {
	t.Helper()
	// The create handler only touches the repository unless a migration is
	// present, so a nil core is enough to exercise webhook validation.
	controller := NewInstances(repo, nil)
	app := echo.New()
	app.POST("/v1/instance/create", controller.Create)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/instance/create", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	app.ServeHTTP(rec, req)
	return rec
}

func TestCreateInstanceRejectsHeadersOverPlainHTTP(t *testing.T) {
	repo := newInMemoryInstanceRepo()
	rec := serveInstanceCreate(t, repo,
		`{"instanceName":"inst","webhook":{"url":"http://internal-host/hook","headers":{"Authorization":"Bearer secret"}}}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestCreateInstanceRejectsInvalidHeaderName(t *testing.T) {
	repo := newInMemoryInstanceRepo()
	rec := serveInstanceCreate(t, repo,
		`{"instanceName":"inst","webhook":{"url":"https://example.com/hook","headers":{"Bad Name":"x"}}}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestCreateInstanceAcceptsValidHTTPSWebhook(t *testing.T) {
	repo := newInMemoryInstanceRepo()
	rec := serveInstanceCreate(t, repo,
		`{"instanceName":"inst","webhook":{"url":"https://example.com/hook","headers":{"Authorization":"Bearer secret"}}}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if got := repo.byID["inst"].Webhook.Headers["Authorization"]; got != "Bearer secret" {
		t.Fatalf("headers not persisted: %q", got)
	}
}

func TestCreateInstanceWithoutWebhookStillWorks(t *testing.T) {
	repo := newInMemoryInstanceRepo()
	rec := serveInstanceCreate(t, repo, `{"instanceName":"inst"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestCreateInstanceWithPlainHTTPWebhookWithoutHeadersStillWorks(t *testing.T) {
	repo := newInMemoryInstanceRepo()
	rec := serveInstanceCreate(t, repo, `{"instanceName":"inst","webhook":{"url":"http://example.com/hook"}}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
}
