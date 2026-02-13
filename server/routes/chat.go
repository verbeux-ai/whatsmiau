package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
	"github.com/verbeux-ai/whatsmiau/services"
)

func Chat(group *echo.Group) {
	redisInstance := instances.NewRedis(services.Redis())
	controller := controllers.NewChats(redisInstance, whatsmiau.Get())

	group.POST("/presence", controller.SendChatPresence)
	group.POST("/read-messages", controller.ReadMessages)
	group.POST("/mark-audio-played", controller.MarkAudioPlayed)
	group.GET("/contacts", controller.GetContacts)
	group.GET("/contacts/:remoteJid/profile-pic", controller.GetContactProfilePic)
	group.GET("/groups", controller.GetGroups)
	group.GET("/groups/:groupJid", controller.GetGroupInfo)
	group.GET("/messages/:remoteJid", controller.GetMessages)
}

func ChatEVO(group *echo.Group) {
	redisInstance := instances.NewRedis(services.Redis())
	controller := controllers.NewChats(redisInstance, whatsmiau.Get())

	// Evolution API Compatibility (partially REST)
	group.POST("/markMessageAsRead/:instance", controller.ReadMessages)
	group.POST("/sendPresence/:instance", controller.SendChatPresence)
	group.POST("/whatsappNumbers/:instance", controller.NumberExists)
}
