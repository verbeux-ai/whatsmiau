package controllers

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/models"
)

// cachedInstance returns a core with just the instance cache wired, so handlers
// that invalidate the cache can run without standing up Redis or WhatsApp.
func cachedInstance(t *testing.T) *whatsmiau.Whatsmiau {
	t.Helper()
	core := &whatsmiau.Whatsmiau{}
	setUnexportedField(t, core, "instanceCache", reflect.ValueOf(xsync.NewMap[string, models.Instance]()))
	return core
}

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

func serveInstanceUpdate(t *testing.T, repo interfaces.InstanceRepository, body string) *httptest.ResponseRecorder {
	t.Helper()
	controller := NewInstances(repo, cachedInstance(t))
	app := echo.New()
	app.PUT("/v1/instance/update/:id", controller.Update)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/instance/update/inst", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	app.ServeHTTP(rec, req)
	return rec
}

func serveInstanceList(t *testing.T, repo interfaces.InstanceRepository) *httptest.ResponseRecorder {
	t.Helper()
	controller := NewInstances(repo, nil)
	app := echo.New()
	app.GET("/v1/instance/fetchInstances", controller.List)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/instance/fetchInstances?instanceName=inst", nil)
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

func TestUpdateInstanceRejectsURLDowngradeWithStoredHeaders(t *testing.T) {
	repo := newInMemoryInstanceRepo(models.Instance{
		ID:      "inst",
		Webhook: models.InstanceWebhook{Url: "https://example.com/hook", Headers: map[string]string{"Authorization": "Bearer secret"}},
	})
	// The request does not carry headers; they come from storage, so the
	// resulting pair must still be rejected.
	rec := serveInstanceUpdate(t, repo, `{"webhook":{"url":"http://example.com/hook"}}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestUpdateInstanceAllowsHTTPSWithStoredHeaders(t *testing.T) {
	repo := newInMemoryInstanceRepo(models.Instance{
		ID:      "inst",
		Webhook: models.InstanceWebhook{Url: "https://example.com/hook", Headers: map[string]string{"Authorization": "Bearer secret"}},
	})
	rec := serveInstanceUpdate(t, repo, `{"webhook":{"url":"https://other.example/hook"}}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestUpdateInstanceAllowsPlainHTTPWithoutStoredHeaders(t *testing.T) {
	repo := newInMemoryInstanceRepo(models.Instance{ID: "inst"})
	rec := serveInstanceUpdate(t, repo, `{"webhook":{"url":"http://example.com/hook"}}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestListInstancesHidesWebhookHeaders(t *testing.T) {
	repo := newInMemoryInstanceRepo(models.Instance{
		ID:      "inst",
		Webhook: models.InstanceWebhook{Url: "https://example.com/hook", Headers: map[string]string{"Authorization": "Bearer secret"}},
	})
	rec := serveInstanceList(t, repo)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Bearer secret") {
		t.Fatalf("listing must not leak webhook headers: %s", rec.Body.String())
	}
}
