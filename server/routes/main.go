package routes

import (
	"github.com/labstack/echo/v4"
)

func Load(app *echo.Echo) {
	V1(app.Group("/v1"))
}

func V1(group *echo.Group) {

	Instance(group.Group("/instance"))
}
