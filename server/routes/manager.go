package routes

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
	"github.com/verbeux-ai/whatsmiau/server/middleware"
	"github.com/verbeux-ai/whatsmiau/services"
)

func managerOrigin() string {
	parsed, err := url.Parse(env.Env.ManagerURL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return strings.TrimRight(parsed.Scheme+"://"+parsed.Host, "/")
}

func Manager(group *echo.Group) {
	repo := instances.NewRedis(services.Redis())
	w := whatsmiau.Get()
	tmpl := controllers.ParseManagerTemplates()

	controller := controllers.NewManager(repo, w, tmpl)

	group.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: []string{managerOrigin()},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
	}))

	group.Static("/static", "server/static")

	group.GET("", func(ctx echo.Context) error {
		return ctx.Redirect(http.StatusFound, "/manager/instances")
	})

	group.GET("/login", controller.LoginPage)
	group.POST("/login", controller.Login)

	auth := group.Group("",
		middleware.Simplify(middleware.ManagerAuth),
	)

	auth.POST("/logout", controller.Logout)
	auth.GET("/instances", controller.ListInstances)
	auth.POST("/instances", controller.CreateInstance)
	auth.GET("/instances/:id", controller.GetInstance)
	auth.GET("/instances/:id/status", controller.StatusBadge)
	auth.POST("/instances/:id/connect", controller.ConnectInstance)
	auth.GET("/instances/:id/qr-poll", controller.PollQRCode)
	auth.POST("/instances/:id/logout", controller.LogoutInstance)
	auth.DELETE("/instances/:id", controller.DeleteInstance)
	auth.PUT("/instances/:id", controller.UpdateInstance)
}
