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

//go:generate go run github.com/swaggo/swag/cmd/swag init -g main.go -o docs
//go:generate go run ./cmd/swagger-contract

// @title           WhatsMiau API
// @version         0.3.2
// @description     Authenticated WhatsApp API compatible with Evolution API. The specification covers the complete application route surface, including canonical routes, compatibility aliases, calls and documentation delivery.

// @contact.name   Verbeux AI
// @contact.url    https://github.com/verbeux-ai/whatsmiau

// @license.name  MIT

// @host      localhost:8080
// @BasePath  /
// @schemes   http https

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name apikey
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
