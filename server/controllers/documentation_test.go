package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/server/middleware"
)

func TestDocumentationRedirectsToSwaggerUI(t *testing.T) {
	controller := NewDocumentation()
	app := echo.New()
	app.GET("/docs", controller.Bootstrap)
	app.GET("/docs/swagger.json", controller.BrowserSwaggerJSON)
	app.GET("/docs/swagger.yaml", controller.BrowserSwaggerYAML)
	app.GET("/docs/*", controller.UI)

	request := httptest.NewRequest(http.MethodGet, "/docs", nil)
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("documentation redirect status: got %d, want %d", recorder.Code, http.StatusFound)
	}
	if got := recorder.Header().Get(echo.HeaderLocation); got != "/docs/index.html" {
		t.Fatalf("documentation redirect location: got %q", got)
	}
	if got := recorder.Header().Get(echo.HeaderCacheControl); got != "no-store" {
		t.Fatalf("documentation redirect cache control: got %q", got)
	}
}

func TestDocumentationUsesSwaggerAuthorizeAndDoesNotAuthenticateAPIs(t *testing.T) {
	previousEnv := env.Env
	env.Env.ApiKey = "docs-test-key"
	defer func() { env.Env = previousEnv }()

	controller := NewDocumentation()
	app := echo.New()
	app.GET("/docs", controller.Bootstrap)
	app.GET("/docs/swagger.json", controller.BrowserSwaggerJSON)
	app.GET("/docs/swagger.yaml", controller.BrowserSwaggerYAML)
	app.GET("/docs/*", controller.UI)
	app.GET("/v1/protected", func(ctx echo.Context) error { return ctx.NoContent(http.StatusNoContent) }, middleware.Simplify(middleware.Auth))

	uiRequest := httptest.NewRequest(http.MethodGet, "/docs/index.html", nil)
	uiRecorder := httptest.NewRecorder()
	app.ServeHTTP(uiRecorder, uiRequest)
	if uiRecorder.Code != http.StatusOK || !strings.Contains(uiRecorder.Body.String(), "swagger-ui") {
		t.Fatalf("Swagger UI response: status=%d body=%q", uiRecorder.Code, uiRecorder.Body.String())
	}
	if strings.Contains(uiRecorder.Body.String(), env.Env.ApiKey) {
		t.Fatal("Swagger UI must not embed the API key")
	}

	specRequest := httptest.NewRequest(http.MethodGet, "/docs/swagger.json", nil)
	specRecorder := httptest.NewRecorder()
	app.ServeHTTP(specRecorder, specRequest)
	if specRecorder.Code != http.StatusOK || !strings.Contains(specRecorder.Body.String(), `"swagger": "2.0"`) {
		t.Fatalf("public documentation JSON response: status=%d body=%q", specRecorder.Code, specRecorder.Body.String())
	}

	apiRequest := httptest.NewRequest(http.MethodGet, "/v1/protected", nil)
	apiRecorder := httptest.NewRecorder()
	app.ServeHTTP(apiRecorder, apiRequest)
	if apiRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing API key status: got %d, want %d", apiRecorder.Code, http.StatusUnauthorized)
	}

	authorizedAPIRequest := httptest.NewRequest(http.MethodGet, "/v1/protected", nil)
	authorizedAPIRequest.Header.Set("apikey", env.Env.ApiKey)
	authorizedAPIRecorder := httptest.NewRecorder()
	app.ServeHTTP(authorizedAPIRecorder, authorizedAPIRequest)
	if authorizedAPIRecorder.Code != http.StatusNoContent {
		t.Fatalf("authorized API status: got %d, want %d", authorizedAPIRecorder.Code, http.StatusNoContent)
	}
}
