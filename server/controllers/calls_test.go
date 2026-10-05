package controllers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/purpshell/meowcaller"
	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/models"
)

// failingInstanceRepo returns a storage error for every lookup, simulating an
// unavailable repository without standing up Redis.
type failingInstanceRepo struct{ err error }

func (failingInstanceRepo) Create(context.Context, *models.Instance) error { return nil }
func (r failingInstanceRepo) List(context.Context, string) ([]models.Instance, error) {
	return nil, r.err
}
func (failingInstanceRepo) Update(context.Context, string, *models.Instance) (*models.Instance, error) {
	return nil, nil
}
func (failingInstanceRepo) Delete(context.Context, string) error { return nil }

// setUnexportedField writes an unexported field across packages, mirroring the
// reflection helper the lib/whatsmiau tests use to reach library internals.
func setUnexportedField(t *testing.T, target any, name string, value reflect.Value) {
	t.Helper()
	field := reflect.ValueOf(target).Elem().FieldByName(name)
	if !field.IsValid() {
		t.Fatalf("field %q not found", name)
	}
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(value)
}

func newFailureCallInstance(t *testing.T, repo interfaces.InstanceRepository) *whatsmiau.Whatsmiau {
	t.Helper()
	w := &whatsmiau.Whatsmiau{}
	setUnexportedField(t, w, "repo", reflect.ValueOf(repo))
	setUnexportedField(t, w, "callClients", reflect.ValueOf(xsync.NewMap[string, *meowcaller.Client]()))
	bridgesType := reflect.ValueOf(w).Elem().FieldByName("callBridges").Type()
	setUnexportedField(t, w, "callBridges", reflect.New(bridgesType.Elem()))
	return w
}

func TestCallStorageFailureIsInternalError(t *testing.T) {
	previous := env.Env.CallsEnabled
	env.Env.CallsEnabled = true
	t.Cleanup(func() { env.Env.CallsEnabled = previous })

	controller := &Calls{
		whatsmiau: newFailureCallInstance(t, failingInstanceRepo{err: errors.New("redis unavailable (simulated)")}),
		validate:  validator.New(),
	}

	app := echo.New()
	app.POST("/v1/instance/:instance/calls", controller.Offer)
	app.GET("/v1/instance/:instance/calls", controller.List)

	post := httptest.NewRecorder()
	postRequest := httptest.NewRequest(http.MethodPost, "/v1/instance/inst/calls", strings.NewReader(`{"number":"5511999999999"}`))
	postRequest.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	app.ServeHTTP(post, postRequest)
	if post.Code != http.StatusInternalServerError {
		t.Fatalf("POST storage failure status = %d, want %d (body=%s)", post.Code, http.StatusInternalServerError, post.Body.String())
	}

	get := httptest.NewRecorder()
	app.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/v1/instance/inst/calls", nil))
	if get.Code != http.StatusInternalServerError {
		t.Fatalf("GET storage failure status = %d, want %d", get.Code, http.StatusInternalServerError)
	}
}
