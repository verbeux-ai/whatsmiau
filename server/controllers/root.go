package controllers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.mau.fi/whatsmeow"
)

func Root(ctx echo.Context) error {
	res, err := whatsmeow.GetLatestVersion(ctx.Request().Context(), &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "failed to fetch latest version")
	}

	jsonData := map[string]any{
		"status":             200,
		"message":            "Welcome to the Whatsmiau API, a Evolution API alternative, it is working!",
		"version":            "0.3.2",
		"clientName":         "whatsmiau",
		"documentation":      "https://doc.evolution-api.com",
		"whatsappWebVersion": res.String(),
	}

	return ctx.JSON(http.StatusOK, jsonData)
}
