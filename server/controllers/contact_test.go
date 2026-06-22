package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/models"
	"go.mau.fi/whatsmeow"
)

type fakeInstanceRepository struct {
	listResult []models.Instance
	listErr    error
}

type fakeContactService struct {
	result *whatsmiau.ContactListResult
	err    error
}

func (f *fakeContactService) ListContacts(ctx context.Context, instanceID string, page, limit int) (*whatsmiau.ContactListResult, error) {
	return f.result, f.err
}

func (f *fakeInstanceRepository) Create(ctx context.Context, instance *models.Instance) error {
	return nil
}

func (f *fakeInstanceRepository) List(ctx context.Context, id string) ([]models.Instance, error) {
	return f.listResult, f.listErr
}

func (f *fakeInstanceRepository) Update(ctx context.Context, id string, instance *models.Instance) (*models.Instance, error) {
	return nil, nil
}

func (f *fakeInstanceRepository) Delete(ctx context.Context, id string) error {
	return nil
}

func decodeErrorResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return body
}

func TestContactListValidationRequiresPageAndLimit(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/chat/findContacts/test", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPath("/chat/findContacts/:nameInstance")
	ctx.SetParamNames("nameInstance")
	ctx.SetParamValues("test")

	controller := NewContacts(&fakeInstanceRepository{}, nil)
	if err := controller.List(ctx); err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	body := decodeErrorResponse(t, rec)
	if body["message"] != "invalid request query" {
		t.Fatalf("expected invalid request query message, got %#v", body["message"])
	}
}

func TestContactListReturnsNotFoundForUnknownInstance(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/chat/findContacts/test?page=1&limit=10", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPath("/chat/findContacts/:nameInstance")
	ctx.SetParamNames("nameInstance")
	ctx.SetParamValues("test")

	controller := NewContacts(&fakeInstanceRepository{listResult: []models.Instance{}}, nil)
	if err := controller.List(ctx); err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	body := decodeErrorResponse(t, rec)
	if body["message"] != "instance not found" {
		t.Fatalf("expected instance not found message, got %#v", body["message"])
	}
}

func TestContactListReturnsPaginatedData(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/chat/findContacts/test?page=2&limit=1", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPath("/chat/findContacts/:nameInstance")
	ctx.SetParamNames("nameInstance")
	ctx.SetParamValues("test")

	controller := NewContacts(&fakeInstanceRepository{listResult: []models.Instance{{ID: "test"}}}, nil)
	controller.service = &fakeContactService{
		result: &whatsmiau.ContactListResult{
			Data: []whatsmiau.ContactListItem{{
				ID:            "5511999999999@s.whatsapp.net",
				RemoteJID:     "5511999999999@s.whatsapp.net",
				PushName:      "Joao",
				ProfilePicURL: "",
				CreatedAt:     "",
				UpdatedAt:     "",
				InstanceID:    "test",
				IsGroup:       false,
				IsSaved:       true,
				Type:          "contact",
			}},
			Page:       2,
			Limit:      1,
			Total:      3,
			TotalPages: 3,
		},
	}

	if err := controller.List(ctx); err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := body["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("expected one contact in data, got %#v", body["data"])
	}

	contact, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("expected contact object, got %#v", data[0])
	}
	if contact["id"] != "5511999999999@s.whatsapp.net" {
		t.Fatalf("expected id in response, got %#v", contact["id"])
	}
	if contact["remoteJid"] != "5511999999999@s.whatsapp.net" {
		t.Fatalf("expected remoteJid in response, got %#v", contact["remoteJid"])
	}
	if contact["pushName"] != "Joao" {
		t.Fatalf("expected pushName in response, got %#v", contact["pushName"])
	}
	if contact["profilePicUrl"] != "" {
		t.Fatalf("expected empty profilePicUrl in response, got %#v", contact["profilePicUrl"])
	}
	if contact["instanceId"] != "test" {
		t.Fatalf("expected instanceId in response, got %#v", contact["instanceId"])
	}
	if contact["isGroup"] != false {
		t.Fatalf("expected isGroup false in response, got %#v", contact["isGroup"])
	}
	if contact["isSaved"] != true {
		t.Fatalf("expected isSaved true in response, got %#v", contact["isSaved"])
	}
	if contact["type"] != "contact" {
		t.Fatalf("expected type contact in response, got %#v", contact["type"])
	}

	pagination, ok := body["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("expected pagination object, got %#v", body["pagination"])
	}
	if pagination["page"] != float64(2) || pagination["limit"] != float64(1) || pagination["total"] != float64(3) || pagination["totalPages"] != float64(3) {
		t.Fatalf("unexpected pagination %#v", pagination)
	}
}

func TestContactListReturnsConflictForDisconnectedInstance(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/chat/findContacts/test?page=1&limit=10", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPath("/chat/findContacts/:nameInstance")
	ctx.SetParamNames("nameInstance")
	ctx.SetParamValues("test")

	controller := NewContacts(&fakeInstanceRepository{listResult: []models.Instance{{ID: "test"}}}, nil)
	controller.service = &fakeContactService{err: whatsmeow.ErrClientIsNil}

	if err := controller.List(ctx); err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
	body := decodeErrorResponse(t, rec)
	if body["message"] != "instance is not connected" {
		t.Fatalf("expected instance is not connected message, got %#v", body["message"])
	}
}

func TestContactListReturnsInternalServerErrorOnStoreFailure(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/chat/findContacts/test?page=1&limit=10", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPath("/chat/findContacts/:nameInstance")
	ctx.SetParamNames("nameInstance")
	ctx.SetParamValues("test")

	controller := NewContacts(&fakeInstanceRepository{listResult: []models.Instance{{ID: "test"}}}, nil)
	controller.service = &fakeContactService{err: errors.New("boom")}

	if err := controller.List(ctx); err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	body := decodeErrorResponse(t, rec)
	if body["message"] != "failed to read contacts from local store" {
		t.Fatalf("expected store failure message, got %#v", body["message"])
	}
}
