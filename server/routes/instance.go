package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
	"github.com/verbeux-ai/whatsmiau/services"
)

func Instance(group *echo.Group) {
	redisInstance := instances.NewRedis(services.Redis())

	controller := controllers.NewInstances(redisInstance, whatsmiau.Get())
	group.POST("", controller.Create)
	group.GET("", controller.List)
	group.POST(":id/connect", controller.Connect)
	group.POST(":id/logout", controller.Logout)
	group.DELETE(":id", controller.Delete)
	group.GET(":id/status", controller.Status)

	// Evolution API Compatibility (partially REST)
	group.POST("/create", controller.Create)
	group.GET("/fetchInstances", controller.List)
	group.GET("/connect/:id", controller.Connect)
	group.GET("/connectionState/:id", controller.Status)
	group.DELETE("/logout/:id", controller.Logout)
	group.DELETE("/delete/:id", controller.Delete)
}
