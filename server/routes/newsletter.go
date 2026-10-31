package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
)

// Newsletter manages WhatsApp channels. The Evolution API has no channel
// routes, so there is no compatibility alias for this group.
func Newsletter(group *echo.Group) {
	registerNewsletter(group, controllers.NewNewsletters(whatsmiau.Get()))
}

func registerNewsletter(group *echo.Group, controller *controllers.Newsletter) {
	group.GET("/fetchAll", controller.FetchAll)
	group.GET("/info", controller.Info)
	group.GET("/inviteInfo", controller.InviteInfo)
	group.POST("/create", controller.Create)
	group.POST("/follow", controller.Follow)
	group.POST("/unfollow", controller.Unfollow)
	group.POST("/mute", controller.Mute)
	group.POST("/unmute", controller.Unmute)
}
