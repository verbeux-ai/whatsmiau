package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
	"github.com/verbeux-ai/whatsmiau/services"
)

func Chat(group *echo.Group) {
	redisInstance := instances.NewRedis(services.Redis())
	controller := controllers.NewChats(redisInstance, lib.Get())

	group.POST("presence", controller.SendChatPresence)
	group.POST("read-messages", controller.ReadMessages)
}

func ChatEVO(group *echo.Group) {
	redisInstance := instances.NewRedis(services.Redis())
	controller := controllers.NewChats(redisInstance, lib.Get())

	// Evolution API Compatibility (partially REST)
	group.POST("/markMessageAsRead/:instance", controller.ReadMessages)
	group.POST("/sendPresence/:instance", controller.SendChatPresence)
}
