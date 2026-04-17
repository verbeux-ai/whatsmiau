package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
	"github.com/verbeux-ai/whatsmiau/services"
)

func Message(group *echo.Group) {
	redisInstance := instances.NewRedis(services.Redis())
	controller := controllers.NewMessages(redisInstance, whatsmiau.Get())

	group.POST("/text", controller.SendText)
	group.POST("/audio", controller.SendAudio)
	group.POST("/document", controller.SendDocument)
	group.POST("/image", controller.SendImage)
	group.POST("/list", controller.SendList)
	group.POST("/buttons", controller.SendButtons)
	group.POST("/location", controller.SendLocation)
}

func MessageEVO(group *echo.Group) {
	redisInstance := instances.NewRedis(services.Redis())
	controller := controllers.NewMessages(redisInstance, whatsmiau.Get())

	// Evolution API Compatibility (partially REST)
	group.POST("/sendText/:instance", controller.SendText)
	group.POST("/sendWhatsAppAudio/:instance", controller.SendAudio) // is always whatsapp 🤣
	group.POST("/sendMedia/:instance", controller.SendMedia)
	group.POST("/sendReaction/:instance", controller.SendReaction)
	group.POST("/sendList/:instance", controller.SendList)
	group.POST("/sendButtons/:instance", controller.SendButtons)
	group.POST("/sendLocation/:instance", controller.SendLocation)
}
