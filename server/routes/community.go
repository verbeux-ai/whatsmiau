package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
	"github.com/verbeux-ai/whatsmiau/services"
)

func Community(group *echo.Group) {
	redisInstance := instances.NewRedis(services.Redis())
	controller := controllers.NewCommunities(redisInstance, whatsmiau.Get())

	group.POST("/create", controller.CreateCommunity)
	group.POST("/createSubGroup", controller.CreateSubGroup)
	group.POST("/linkGroup", controller.LinkGroup)
	group.POST("/unlinkGroup", controller.UnlinkGroup)
	group.GET("/subGroups", controller.SubGroups)
	group.GET("/linkedGroupsParticipants", controller.LinkedGroupsParticipants)
	group.POST("/setJoinApprovalMode", controller.SetJoinApprovalMode)
	group.POST("/setMemberAddMode", controller.SetMemberAddMode)
	group.GET("/requestParticipants", controller.RequestParticipants)
	group.POST("/requestParticipants/update", controller.UpdateRequestParticipants)
}
