package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/services"
	"go.uber.org/zap"
)

const (
	managerCookieName    = "manager_session"
	managerSessionPrefix = "manager_session:"
	managerSessionTTL    = 48 * time.Hour
)

var managerSessionMaxAge = int(managerSessionTTL.Seconds())

func ManagerAuth(ctx echo.Context, next echo.HandlerFunc) error {
	cookie, err := ctx.Cookie(managerCookieName)
	if err != nil || cookie.Value == "" {
		return ctx.Redirect(http.StatusFound, "/manager/login")
	}

	c, cancel := context.WithTimeout(ctx.Request().Context(), 5*time.Second)
	defer cancel()

	exists, err := services.Redis().Exists(c, managerSessionPrefix+cookie.Value).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		zap.L().Error("failed to check manager session", zap.Error(err))
	}
	if err != nil || exists == 0 {
		return ctx.Redirect(http.StatusFound, "/manager/login")
	}

	return next(ctx)
}

func isManagerSecure() bool {
	return strings.HasPrefix(env.Env.ManagerURL, "https://")
}

func CreateSession(ctx echo.Context) (*http.Cookie, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)

	c, cancel := context.WithTimeout(ctx.Request().Context(), 5*time.Second)
	defer cancel()

	if err := services.Redis().Set(c, managerSessionPrefix+token, "1", managerSessionTTL).Err(); err != nil {
		return nil, err
	}

	cookie := &http.Cookie{
		Name:     managerCookieName,
		Value:    token,
		Path:     "/manager",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   managerSessionMaxAge,
		Secure:   isManagerSecure(),
	}

	return cookie, nil
}

func SessionToken(ctx echo.Context) (string, error) {
	cookie, err := ctx.Cookie(managerCookieName)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func DeleteSession(ctx echo.Context, token string) (*http.Cookie, error) {
	c, cancel := context.WithTimeout(ctx.Request().Context(), 5*time.Second)
	defer cancel()

	services.Redis().Del(c, managerSessionPrefix+token)

	cookie := &http.Cookie{
		Name:     managerCookieName,
		Value:    "",
		Path:     "/manager",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Secure:   isManagerSecure(),
	}

	return cookie, nil
}
