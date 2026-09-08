package controllers

import (
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/models"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.uber.org/zap"
)

type Webhook struct {
	repo      interfaces.InstanceRepository
	whatsmiau *whatsmiau.Whatsmiau
	validate  *validator.Validate
}

func NewWebhooks(repository interfaces.InstanceRepository, whatsmiau *whatsmiau.Whatsmiau) *Webhook {
	return &Webhook{
		repo:      repository,
		whatsmiau: whatsmiau,
		validate:  validator.New(),
	}
}

// Set configures outbound webhook delivery for an instance.
// @Summary Configure an instance webhook
// @Description Enables or updates the outbound webhook target. Call signaling is emitted only when the configured events include CALL; payloads never contain media, SDP, relay data, or call keys.
// @Tags Webhooks
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param instance path string true "Instance ID"
// @Param body body dto.SetWebhookRequest true "Webhook configuration"
// @Success 200 {object} dto.SetWebhookResponse
// @Failure 400 {object} utils.HTTPErrorResponse
// @Failure 401 {object} utils.AuthenticationErrorResponse
// @Failure 404 {object} utils.HTTPErrorResponse
// @Failure 422 {object} utils.HTTPErrorResponse
// @Failure 500 {object} utils.HTTPErrorResponse
// @Router /v1/webhook/set/{instance} [post]
func (s *Webhook) Set(ctx echo.Context) error {
	var request dto.SetWebhookRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := s.validate.Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	c := ctx.Request().Context()
	instance, err := s.repo.Update(c, request.InstanceID, &models.Instance{
		ID: request.InstanceID,
		Webhook: models.InstanceWebhook{
			Enabled: request.Webhook.Enabled,
			Url:     request.Webhook.URL,
			Base64:  request.Webhook.Base64,
			Headers: request.Webhook.Headers,
			Events:  request.Webhook.Events,
		},
	})
	if err != nil {
		if errors.Is(err, instances.ErrorNotFound) {
			return utils.HTTPFail(ctx, http.StatusNotFound, err, "instance not found")
		}
		zap.L().Error("failed to update webhook", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to update webhook")
	}

	return ctx.JSON(http.StatusOK, dto.SetWebhookResponse{
		Webhook: &instance.Webhook,
	})
}

// Find returns the current outbound webhook configuration for an instance.
// @Summary Get an instance webhook
// @Tags Webhooks
// @Produce json
// @Security ApiKeyAuth
// @Param instance path string true "Instance ID"
// @Success 200 {object} dto.FindWebhookResponse
// @Failure 400 {object} utils.HTTPErrorResponse
// @Failure 401 {object} utils.AuthenticationErrorResponse
// @Failure 404 {object} utils.HTTPErrorResponse
// @Failure 422 {object} utils.HTTPErrorResponse
// @Failure 500 {object} utils.HTTPErrorResponse
// @Router /v1/webhook/find/{instance} [get]
func (s *Webhook) Find(ctx echo.Context) error {
	var request dto.FindWebhookRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := s.validate.Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	c := ctx.Request().Context()
	result, err := s.repo.List(c, request.InstanceID)
	if err != nil {
		zap.L().Error("failed to list instances", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to get instance")
	}

	if len(result) == 0 {
		return utils.HTTPFail(ctx, http.StatusNotFound, instances.ErrorNotFound, "instance not found")
	}

	return ctx.JSON(http.StatusOK, dto.FindWebhookResponse{
		Webhook: &result[0].Webhook,
	})
}
