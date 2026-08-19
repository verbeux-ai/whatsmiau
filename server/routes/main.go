package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/server/middleware"
)

func Load(app *echo.Echo) {
	v1 := app.Group("/v1", middleware.Simplify(middleware.Auth))
	Documentation(app, v1)
	V1(v1)
}

func V1(group *echo.Group) {
	Root(group)
	Instance(group.Group("/instance"))
	Calls(group.Group("/instance/:instance/calls"))
	Message(group.Group("/instance/:instance/message"))
	Chat(group.Group("/instance/:instance/chat"))
	Group(group.Group("/instance/:instance/group"))
	Community(group.Group("/instance/:instance/community"))
	Contact(group.Group("/instance/:instance/contact"))

	ChatEVO(group.Group("/chat"))
	MessageEVO(group.Group("/message"))
	GroupEVO(group.Group("/group"))
	Webhook(group.Group("/webhook"))
	ContactEVO(group.Group("/contact/fetchAll"))
}
