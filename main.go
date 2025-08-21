package main

import (
	"log"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/verbeux-ai/whatsmiau/env"
	log_connect "github.com/verbeux-ai/whatsmiau/lib/log-connect"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/routes"
	"github.com/verbeux-ai/whatsmiau/services"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"golang.org/x/net/http2"
)

//	@title			Whatsmiau API
//	@version		1.0
//	@description	Backend whatsapp

//	@contact.name	Verbeux
//	@contact.url	https://verbeux.com.br/support
//	@contact.email	verbeux@verbeux.com.br

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

//	@host		localhost:8080
//	@BasePath	/
//	@schemes	http

// @securityDefinitions.apikey	ApiKeyAuth
// @in							header
// @name						apikey
// @description				Apikey gerada dentro da plataforma administrativa
func main() {
	if err := env.Load(); err != nil {
		panic(err)
	}

	if err := log_connect.StartLogger(); err != nil {
		log.Fatalln(err)
	}

	ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	whatsmiau.LoadMiau(ctx, services.SQLStore())

	app := echo.New()
	app.Pre(middleware.Recover())
	app.Pre(middleware.Logger())
	app.Pre(middleware.RemoveTrailingSlash())
	app.Pre(middleware.CORS())

	routes.Load(app)

	port := ":" + env.Env.Port
	zap.L().Info("starting server...", zap.String("port", port))

	s := &http2.Server{}
	if err := app.StartH2CServer(port, s); err != nil {
		zap.L().Fatal("failed to start server", zap.Error(err))
	}
}
