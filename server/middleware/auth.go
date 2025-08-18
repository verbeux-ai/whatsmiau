package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/env"
)

func Auth(ctx echo.Context, next echo.HandlerFunc) error {
	gotApikey := ctx.Get("apikey")
	if gotApikey != env.Env.ApiKey {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	return next(ctx)
}

type simplifiedMiddleware func(c echo.Context, next echo.HandlerFunc) error

func Simplify(handler simplifiedMiddleware) func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			return handler(ctx, next)
		}
	}
}
