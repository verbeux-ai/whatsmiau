package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
)

func Root(group *echo.Group) {
	group.GET("", controllers.Root)
}
