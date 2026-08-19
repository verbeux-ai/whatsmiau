package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
)

// Documentation registers the Swagger UI and machine-readable Swagger
// documents. The UI owns API-key entry through its Authorize dialog; the
// application endpoints under /v1 remain protected by the normal middleware.
func Documentation(app *echo.Echo, v1 *echo.Group) {
	controller := controllers.NewDocumentation()

	app.GET("/docs", controller.Bootstrap)
	app.GET("/docs/swagger.json", controller.BrowserSwaggerJSON)
	app.GET("/docs/swagger.yaml", controller.BrowserSwaggerYAML)
	app.GET("/docs/*", controller.UI)

	v1.GET("/swagger.json", controller.SwaggerJSON)
	v1.GET("/swagger.yaml", controller.SwaggerYAML)
}
