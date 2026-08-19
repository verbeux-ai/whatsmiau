package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
)

// Calls is mounted inside /v1, so every REST request and WebSocket handshake
// is authenticated by middleware.Auth using the standard apikey header.
func Calls(group *echo.Group) {
	controller := controllers.NewCalls(whatsmiau.Get())
	group.POST("", controller.Offer)
	group.GET("", controller.List)
	group.POST("/:callID/answer", controller.Answer)
	group.POST("/:callID/reject", controller.Reject)
	group.POST("/:callID/hangup", controller.Hangup)
	group.GET("/:callID/audio", controller.Audio)
}
