package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
	"github.com/verbeux-ai/whatsmiau/services"
)

func Message(group *echo.Group) {
	redisInstance := instances.NewRedis(services.Redis())
	controller := controllers.NewMessages(redisInstance, lib.Get())

	group.POST("text", controller.SendText)
}

func MessageEVO(group *echo.Group) {
	redisInstance := instances.NewRedis(services.Redis())
	controller := controllers.NewMessages(redisInstance, lib.Get())

	// Evolution API Compatibility (partially REST)
	group.POST("/sendText/:instance", controller.SendText)
}
