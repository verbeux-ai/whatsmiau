package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/server/middleware"
)

func Load(app *echo.Echo) {
	app.Use(middleware.Simplify(middleware.Auth))

	V1(app.Group("/v1"))
}

func V1(group *echo.Group) {
	Instance(group.Group("/instance"))
	Message(group.Group("/instance/:instance/message"))
	Chat(group.Group("/instance/:instance/chat"))

	ChatEVO(group.Group("/chat"))
	MessageEVO(group.Group("/message"))

}
