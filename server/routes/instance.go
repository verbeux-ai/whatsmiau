package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
	"github.com/verbeux-ai/whatsmiau/services"
)

func Instance(group *echo.Group) {
	redisInstance := instances.NewRedis(services.Redis())

	controller := controllers.NewInstances(redisInstance, lib.Get())
	group.POST("", controller.Create)
	group.GET("", controller.List)

	// Evolution API Compatibility (partially REST)
	group.POST("/create", controller.Create)
	group.GET("/fetchInstances", controller.List)
}
