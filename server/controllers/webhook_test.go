package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/models"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
)

// inMemoryInstanceRepo stores instances in a map so handler tests can run
// without Redis. Its Update mirrors the additive header merge of
// repositories/instances, keeping the effective-state expectation honest.
type inMemoryInstanceRepo struct {
	byID map[string]models.Instance
}

var _ interfaces.InstanceRepository = (*inMemoryInstanceRepo)(nil)

func newInMemoryInstanceRepo(records ...models.Instance) *inMemoryInstanceRepo {
	repo := &inMemoryInstanceRepo{byID: make(map[string]models.Instance, len(records))}
	for _, record := range records {
		repo.byID[record.ID] = record
	}
	return repo
}

func (r *inMemoryInstanceRepo) Create(_ context.Context, instance *models.Instance) error {
	r.byID[instance.ID] = *instance
	return nil
}

func (r *inMemoryInstanceRepo) List(_ context.Context, id string) ([]models.Instance, error) {
	instance, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	return []models.Instance{instance}, nil
}

func (r *inMemoryInstanceRepo) Update(_ context.Context, id string, toUpdate *models.Instance) (*models.Instance, error) {
	stored, ok := r.byID[id]
	if !ok {
		return nil, instances.ErrorNotFound
	}
	if toUpdate.Webhook.Enabled != nil {
		stored.Webhook.Enabled = toUpdate.Webhook.Enabled
	}
	if toUpdate.Webhook.Url != "" {
		stored.Webhook.Url = toUpdate.Webhook.Url
	}
	if toUpdate.Webhook.Base64 != nil {
		stored.Webhook.Base64 = toUpdate.Webhook.Base64
	}
	if toUpdate.Webhook.Headers != nil {
		stored.Webhook.Headers = toUpdate.Webhook.Headers
	}
	if toUpdate.Webhook.Events != nil {
		stored.Webhook.Events = toUpdate.Webhook.Events
	}
	r.byID[id] = stored
	return &stored, nil
}

func (r *inMemoryInstanceRepo) Delete(_ context.Context, id string) error {
	delete(r.byID, id)
	return nil
}

func serveWebhookSet(t *testing.T, repo interfaces.InstanceRepository, body string) *httptest.ResponseRecorder {
	t.Helper()
	controller := &Webhook{repo: repo, validate: validator.New()}
	app := echo.New()
	app.POST("/v1/webhook/set/:instance", controller.Set)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhook/set/inst", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	app.ServeHTTP(rec, req)
	return rec
}

func TestWebhookSetRejectsHeadersOverPlainHTTP(t *testing.T) {
	repo := newInMemoryInstanceRepo(models.Instance{ID: "inst"})
	rec := serveWebhookSet(t, repo, `{"webhook":{"url":"http://example.com/hook","headers":{"Authorization":"Bearer x"}}}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestWebhookSetRejectsHeadersLeftOverFromStoredHTTPWebhook(t *testing.T) {
	repo := newInMemoryInstanceRepo(models.Instance{
		ID:      "inst",
		Webhook: models.InstanceWebhook{Url: "http://example.com/hook"},
	})
	// This request only adds headers; the http url comes from storage, so the
	// effective state must still be rejected.
	rec := serveWebhookSet(t, repo, `{"webhook":{"headers":{"Authorization":"Bearer x"}}}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestWebhookSetAcceptsHeadersOverHTTPS(t *testing.T) {
	repo := newInMemoryInstanceRepo(models.Instance{ID: "inst"})
	rec := serveWebhookSet(t, repo, `{"webhook":{"url":"https://example.com/hook","headers":{"Authorization":"Bearer x"}}}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := repo.byID["inst"].Webhook.Headers["Authorization"]; got != "Bearer x" {
		t.Fatalf("headers not persisted: %q", got)
	}
}

func TestWebhookSetNotFound(t *testing.T) {
	repo := newInMemoryInstanceRepo()
	rec := serveWebhookSet(t, repo, `{"webhook":{"url":"https://example.com/hook"}}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestWebhookSetReplacesHeadersInsteadOfMerging(t *testing.T) {
	repo := newInMemoryInstanceRepo(models.Instance{
		ID:      "inst",
		Webhook: models.InstanceWebhook{Url: "https://example.com/hook", Headers: map[string]string{"X-Keep": "1", "X-Drop": "2"}},
	})
	rec := serveWebhookSet(t, repo, `{"webhook":{"headers":{"X-Keep":"3"}}}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	got := repo.byID["inst"].Webhook.Headers
	if len(got) != 1 || got["X-Keep"] != "3" {
		t.Fatalf("supplying headers must replace the stored set: %#v", got)
	}
}

func TestWebhookSetClearsHeadersWithEmptyObject(t *testing.T) {
	repo := newInMemoryInstanceRepo(models.Instance{
		ID:      "inst",
		Webhook: models.InstanceWebhook{Url: "https://example.com/hook", Headers: map[string]string{"Authorization": "Bearer secret"}},
	})
	rec := serveWebhookSet(t, repo, `{"webhook":{"headers":{}}}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := repo.byID["inst"].Webhook.Headers; len(got) != 0 {
		t.Fatalf("an empty object must clear the stored headers: %#v", got)
	}
}

func TestWebhookSetKeepsHeadersWhenOmitted(t *testing.T) {
	repo := newInMemoryInstanceRepo(models.Instance{
		ID:      "inst",
		Webhook: models.InstanceWebhook{Url: "https://example.com/hook", Headers: map[string]string{"Authorization": "Bearer secret"}},
	})
	rec := serveWebhookSet(t, repo, `{"webhook":{"url":"https://other.example/hook"}}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := repo.byID["inst"].Webhook.Headers["Authorization"]; got != "Bearer secret" {
		t.Fatalf("omitting headers must keep the stored ones: %#v", repo.byID["inst"].Webhook.Headers)
	}
}

func TestWebhookSetIgnoresInstanceIDInBody(t *testing.T) {
	// The route names the target instance. A body key that the JSON binding
	// matches case-insensitively must not be able to redirect the write.
	repo := newInMemoryInstanceRepo(
		models.Instance{ID: "inst"},
		models.Instance{ID: "other"},
	)
	rec := serveWebhookSet(t, repo, `{"instanceId":"other","webhook":{"url":"https://example.com/hook"}}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := repo.byID["inst"].Webhook.Url; got != "https://example.com/hook" {
		t.Fatalf("the route instance must be the one updated, got url %q", got)
	}
	if got := repo.byID["other"].Webhook.Url; got != "" {
		t.Fatalf("the body must not redirect the write to another instance, got url %q", got)
	}
}
