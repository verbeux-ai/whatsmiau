package routes

import (
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/server/middleware"
	"go.uber.org/zap"
)

func Load(app *echo.Echo) {
	app.GET("/swagger/*", echoSwagger.WrapHandler)

	if len(env.Env.ManagerURL) > 0 {
		if len(env.Env.ApiKey) == 0 {
			zap.L().Warn("manager enabled without API_KEY — login accepts any password")
		}
		Manager(app.Group("/manager"))
		zap.L().Info("manager enabled", zap.String("url", env.Env.ManagerURL))
	}

	v1 := app.Group("/v1", middleware.Simplify(middleware.Auth))
	V1(v1)
}

func V1(group *echo.Group) {
	Root(group)
	Instance(group.Group("/instance"))
	Message(group.Group("/instance/:instance/message"))
	Chat(group.Group("/instance/:instance/chat"))
	Group(group.Group("/instance/:instance/group"))
	Community(group.Group("/instance/:instance/community"))

	ChatEVO(group.Group("/chat"))
	MessageEVO(group.Group("/message"))
	GroupEVO(group.Group("/group"))
	Webhook(group.Group("/webhook"))
}
