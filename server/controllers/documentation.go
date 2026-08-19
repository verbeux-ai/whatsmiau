package controllers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
	"github.com/verbeux-ai/whatsmiau/docs"
)

// Documentation exposes the generated contract and the Swagger UI. The UI
// itself does not authenticate requests; the Swagger Authorize dialog supplies
// the standard apikey header when an operation is executed.
type Documentation struct {
	ui echo.HandlerFunc
}

func NewDocumentation() *Documentation {
	return &Documentation{
		ui: echoSwagger.EchoWrapHandler(echoSwagger.URL("/docs/swagger.json")),
	}
}

func noStore(ctx echo.Context) {
	ctx.Response().Header().Set(echo.HeaderCacheControl, "no-store")
}

// Bootstrap redirects to the generated Swagger UI. API keys are entered in
// Swagger's built-in Authorize dialog, not in a custom login page.
// @Summary Open the Swagger UI
// @Description Redirects to the generated Swagger UI. Use its Authorize dialog to configure the standard apikey header before trying authenticated operations.
// @Tags Documentation
// @Produce text/html
// @Success 302 {string} string "Redirect to Swagger UI"
// @Router /docs [get]
func (s *Documentation) Bootstrap(ctx echo.Context) error {
	noStore(ctx)
	return ctx.Redirect(http.StatusFound, "/docs/index.html")
}

// SwaggerJSON exposes the machine-readable contract to authenticated API
// clients through /v1/swagger.json.
// @Summary Download the Swagger JSON contract
// @Tags Documentation
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} object "Swagger 2.0 document"
// @Failure 401 {object} utils.AuthenticationErrorResponse
// @Router /v1/swagger.json [get]
func (s *Documentation) SwaggerJSON(ctx echo.Context) error {
	noStore(ctx)
	return ctx.JSONBlob(http.StatusOK, docs.SwaggerJSON)
}

// SwaggerYAML exposes the machine-readable YAML contract to authenticated API
// clients through /v1/swagger.yaml.
// @Summary Download the Swagger YAML contract
// @Tags Documentation
// @Produce application/yaml
// @Security ApiKeyAuth
// @Success 200 {string} string "Swagger 2.0 YAML document"
// @Failure 401 {object} utils.AuthenticationErrorResponse
// @Router /v1/swagger.yaml [get]
func (s *Documentation) SwaggerYAML(ctx echo.Context) error {
	noStore(ctx)
	return ctx.Blob(http.StatusOK, "application/yaml; charset=utf-8", docs.SwaggerYAML)
}

// BrowserSwaggerJSON returns the same contract used by the Swagger UI.
// @Summary Download Swagger JSON for the UI
// @Description The document is public so the Swagger UI can load before the user enters a key. API operations remain protected by ApiKeyAuth.
// @Tags Documentation
// @Produce json
// @Success 200 {object} object "Swagger 2.0 document"
// @Router /docs/swagger.json [get]
func (s *Documentation) BrowserSwaggerJSON(ctx echo.Context) error { return s.SwaggerJSON(ctx) }

// BrowserSwaggerYAML returns the YAML contract used by API tooling.
// @Summary Download Swagger YAML for the UI
// @Description The document is public so the Swagger UI can load before the user enters a key. API operations remain protected by ApiKeyAuth.
// @Tags Documentation
// @Produce application/yaml
// @Success 200 {string} string "Swagger 2.0 YAML document"
// @Router /docs/swagger.yaml [get]
func (s *Documentation) BrowserSwaggerYAML(ctx echo.Context) error { return s.SwaggerYAML(ctx) }

// UI serves the generated Swagger UI assets.
// @Summary Serve a Swagger UI asset
// @Description Serves Swagger UI assets. The API key is configured through Swagger's built-in Authorize dialog and is sent only to authenticated API operations.
// @Tags Documentation
// @Produce text/html
// @Param asset path string true "Swagger UI asset name"
// @Success 200 {string} string "Swagger UI HTML, JavaScript, CSS, or image asset"
// @Router /docs/{asset} [get]
func (s *Documentation) UI(ctx echo.Context) error {
	return s.ui(ctx)
}
