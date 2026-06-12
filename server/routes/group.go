package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
	"github.com/verbeux-ai/whatsmiau/services"
)

func Group(group *echo.Group) {
	redisInstance := instances.NewRedis(services.Redis())
	controller := controllers.NewGroups(redisInstance, whatsmiau.Get())

	group.POST("/create", controller.CreateGroup)
	group.POST("/updateGroupSubject", controller.UpdateGroupSubject)
	group.POST("/updateGroupPicture", controller.UpdateGroupPicture)
	group.POST("/updateGroupDescription", controller.UpdateGroupDescription)
	group.GET("/findGroupInfos", controller.FindGroupInfos)
	group.GET("/fetchAllGroups", controller.FetchAllGroups)
	group.GET("/participants", controller.FindParticipants)
	group.GET("/inviteCode", controller.InviteCode)
	group.GET("/inviteInfo", controller.InviteInfo)
	group.GET("/acceptInviteCode", controller.AcceptInviteCode)
	group.POST("/sendInvite", controller.SendInvite)
	group.POST("/revokeInviteCode", controller.RevokeInviteCode)
	group.POST("/updateParticipant", controller.UpdateParticipant)
	group.POST("/updateSetting", controller.UpdateSetting)
	group.POST("/toggleEphemeral", controller.ToggleEphemeral)
	group.DELETE("/leaveGroup", controller.LeaveGroup)
}

func GroupEVO(group *echo.Group) {
	redisInstance := instances.NewRedis(services.Redis())
	controller := controllers.NewGroups(redisInstance, whatsmiau.Get())

	// Evolution API Compatibility
	group.POST("/create/:instance", controller.CreateGroup)
	group.POST("/updateGroupSubject/:instance", controller.UpdateGroupSubject)
	group.POST("/updateGroupPicture/:instance", controller.UpdateGroupPicture)
	group.POST("/updateGroupDescription/:instance", controller.UpdateGroupDescription)
	group.GET("/findGroupInfos/:instance", controller.FindGroupInfos)
	group.GET("/fetchAllGroups/:instance", controller.FetchAllGroups)
	group.GET("/participants/:instance", controller.FindParticipants)
	group.GET("/inviteCode/:instance", controller.InviteCode)
	group.GET("/inviteInfo/:instance", controller.InviteInfo)
	group.GET("/acceptInviteCode/:instance", controller.AcceptInviteCode)
	group.POST("/sendInvite/:instance", controller.SendInvite)
	group.POST("/revokeInviteCode/:instance", controller.RevokeInviteCode)
	group.POST("/updateParticipant/:instance", controller.UpdateParticipant)
	group.POST("/updateSetting/:instance", controller.UpdateSetting)
	group.POST("/toggleEphemeral/:instance", controller.ToggleEphemeral)
	group.DELETE("/leaveGroup/:instance", controller.LeaveGroup)
}
