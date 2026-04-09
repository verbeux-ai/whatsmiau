package routes

import (
	"encoding/json"
	"fmt"
	"net/http"

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
	group.POST("/:id/connect", controller.Connect)
	group.POST("/:id/logout", controller.Logout)
	group.DELETE("/:id", controller.Delete)
	group.GET("/:id/status", controller.Status)

	group.GET("/events/stream", func(c echo.Context) error {
		c.Response().Header().Set("Content-Type", "text/event-stream")
		c.Response().Header().Set("Cache-Control", "no-cache")
		c.Response().Header().Set("Connection", "keep-alive")
		c.Response().WriteHeader(http.StatusOK)
		c.Response().Flush()

		miau := whatsmiau.Get()
		ch := miau.SSE.Register()
		defer miau.SSE.Unregister(ch)

		for {
			select {
			case event := <-ch:
				data, _ := json.Marshal(event)
				fmt.Fprintf(c.Response(), "event: status\ndata: %s\n\n", data)
				c.Response().Flush()
			case <-c.Request().Context().Done():
				return nil
			}
		}
	})

	// Evolution API Compatibility (partially REST)
	group.POST("/create", controller.Create)
	group.GET("/fetchInstances", controller.List)
	group.GET("/connect/:id", controller.Connect)
	group.GET("/connect/:id/image", controller.ConnectQRBuffer)
	group.GET("/connectionState/:id", controller.Status)
	group.DELETE("/logout/:id", controller.Logout)
	group.DELETE("/delete/:id", controller.Delete)
	group.PUT("/update/:id", controller.Update)

}
