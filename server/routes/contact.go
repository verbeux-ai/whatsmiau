package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
)

func Contact(group *echo.Group) {
	controller := controllers.NewContacts(whatsmiau.Get())
	group.GET("", controller.List)
}

func ContactEVO(group *echo.Group) {
	controller := controllers.NewContacts(whatsmiau.Get())
	group.GET("/:instance", controller.List)
}
