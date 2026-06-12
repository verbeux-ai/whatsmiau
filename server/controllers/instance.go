package controllers

import (
	"encoding/base64"
	"errors"
	"math/rand/v2"
	"net/http"

	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/models"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
	"go.mau.fi/whatsmeow/types"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/skip2/go-qrcode"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.uber.org/zap"
)

type Instance struct {
	repo      interfaces.InstanceRepository
	whatsmiau *whatsmiau.Whatsmiau
}

func NewInstances(repository interfaces.InstanceRepository, whatsmiau *whatsmiau.Whatsmiau) *Instance {
	return &Instance{
		repo:      repository,
		whatsmiau: whatsmiau,
	}
}

// Create godoc
// @Summary      Create a new WhatsApp instance
// @Description  Creates a new WhatsApp instance with the given name and optional configuration
// @Tags         Instance
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body      dto.CreateInstanceRequest  true  "Instance creation parameters"
// @Success      201   {object}  dto.CreateInstanceResponse
// @Failure      400   {object}  utils.HTTPErrorResponse
// @Failure      422   {object}  utils.HTTPErrorResponse
// @Failure      500   {object}  utils.HTTPErrorResponse
// @Router       /instance [post]
// @Router       /instance/create [post]
func (s *Instance) Create(ctx echo.Context) error {
	var request dto.CreateInstanceRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	request.ID = request.InstanceName
	if request.Instance == nil {
		request.Instance = &models.Instance{
			ID: request.InstanceName,
		}
	} else {
		request.Instance.ID = request.InstanceName
	}
	request.RemoteJID = ""

	if len(request.ProxyHost) <= 0 && len(env.Env.ProxyAddresses) > 0 {
		rd := rand.IntN(len(env.Env.ProxyAddresses))
		proxyUrl := env.Env.ProxyAddresses[rd]

		proxy, err := parseProxyURL(proxyUrl)
		if err != nil {
			return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "invalid proxy url on env")
		}
		request.InstanceProxy = *proxy
	}

	c := ctx.Request().Context()
	if err := s.repo.Create(c, request.Instance); err != nil {
		zap.L().Error("failed to create instance", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to create instance")
	}

	// If migration data is present, import the Baileys session
	if request.Migration != nil {
		result, err := s.whatsmiau.Migrate(c, request.InstanceName, request.Migration.Creds, request.Migration.PreKeys)
		if err != nil {
			zap.L().Error("failed to migrate instance", zap.Error(err))
			return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to migrate instance")
		}

		return ctx.JSON(http.StatusCreated, dto.CreateInstanceResponse{
			Instance: request.Instance,
			Migration: &dto.MigrationResult{
				JID:       result.JID,
				LID:       result.LID,
				PreKeys:   result.PreKeys,
				Connected: result.Connected,
			},
		})
	}

	return ctx.JSON(http.StatusCreated, dto.CreateInstanceResponse{
		Instance: request.Instance,
	})
}

// Update godoc
// @Summary      Update an existing instance
// @Description  Updates webhook and proxy settings for the given instance
// @Tags         Instance
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id    path      string                     true  "Instance ID"
// @Param        body  body      dto.UpdateInstanceRequest   true  "Update parameters"
// @Success      201   {object}  dto.UpdateInstanceResponse
// @Failure      400   {object}  utils.HTTPErrorResponse
// @Failure      404   {object}  utils.HTTPErrorResponse
// @Failure      422   {object}  utils.HTTPErrorResponse
// @Failure      500   {object}  utils.HTTPErrorResponse
// @Router       /instance/update/{id} [put]
func (s *Instance) Update(ctx echo.Context) error {
	var request dto.UpdateInstanceRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	c := ctx.Request().Context()
	instance, err := s.repo.Update(c, request.ID, &models.Instance{
		ID: request.ID,
		Webhook: models.InstanceWebhook{
			Enabled: request.Webhook.Enabled,
			Url:     request.Webhook.URL,
			Base64:  &[]bool{request.Webhook.Base64}[0],
			Events:  request.Webhook.Events,
		},
		InstanceProxy: request.InstanceProxy,
	})
	if err != nil {
		if errors.Is(err, instances.ErrorNotFound) {
			return utils.HTTPFail(ctx, http.StatusNotFound, err, "instance not found")
		}
		zap.L().Error("failed to create instance", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to update instance")
	}

	return ctx.JSON(http.StatusCreated, dto.UpdateInstanceResponse{
		Instance: instance,
	})
}

// List godoc
// @Summary      List instances
// @Description  Returns all instances, optionally filtered by name or ID
// @Tags         Instance
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instanceName  query     string  false  "Filter by instance name"
// @Param        id            query     string  false  "Filter by instance ID"
// @Success      200  {array}   dto.ListInstancesResponse
// @Failure      422  {object}  utils.HTTPErrorResponse
// @Failure      500  {object}  utils.HTTPErrorResponse
// @Router       /instance [get]
// @Router       /instance/fetchInstances [get]
func (s *Instance) List(ctx echo.Context) error {
	c := ctx.Request().Context()
	var request dto.ListInstancesRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if request.InstanceName == "" {
		request.InstanceName = request.ID
	}

	result, err := s.repo.List(c, request.InstanceName)
	if err != nil {
		zap.L().Error("failed to list instances", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to list instances")
	}

	var response []dto.ListInstancesResponse
	for _, instance := range result {
		jid, err := types.ParseJID(instance.RemoteJID)
		if err != nil {
			zap.L().Error("failed to parse jid", zap.Error(err))
		}

		response = append(response, dto.ListInstancesResponse{
			Instance:     &instance,
			OwnerJID:     jid.ToNonAD().String(),
			InstanceName: instance.ID,
		})
	}

	if len(response) == 0 {
		return ctx.JSON(http.StatusOK, []string{})
	}

	return ctx.JSON(http.StatusOK, response)
}

// Connect godoc
// @Summary      Connect an instance (get QR code)
// @Description  Initiates connection for an instance. Returns a base64-encoded QR code PNG if not yet connected, or a connected status message.
// @Tags         Instance
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path      string  true  "Instance ID"
// @Success      200  {object}  dto.ConnectInstanceResponse
// @Failure      404  {object}  utils.HTTPErrorResponse
// @Failure      422  {object}  utils.HTTPErrorResponse
// @Failure      500  {object}  utils.HTTPErrorResponse
// @Router       /instance/{id}/connect [post]
// @Router       /instance/connect/{id} [get]
func (s *Instance) Connect(ctx echo.Context) error {
	c := ctx.Request().Context()
	var request dto.ConnectInstanceRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	result, err := s.repo.List(c, request.ID)
	if err != nil {
		zap.L().Error("failed to list instances", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to list instances")
	}

	if len(result) == 0 {
		return utils.HTTPFail(ctx, http.StatusNotFound, err, "instance not found")
	}

	qrCode, pairingCode, err := s.whatsmiau.Connect(c, request.ID, request.Number)
	if err != nil {
		zap.L().Error("failed to connect instance", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to connect instance")
	}
	if qrCode != "" {
		png, err := qrcode.Encode(qrCode, qrcode.Medium, 512)
		if err != nil {
			zap.L().Error("failed to encode qrcode", zap.Error(err))
			return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to encode qrcode")
		}
		return ctx.JSON(http.StatusOK, dto.ConnectInstanceResponse{
			Message:     "If instance restart this instance could be lost if you cannot connect",
			Connected:   false,
			Base64:      "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
			PairingCode: pairingCode,
		})
	}

	return ctx.JSON(http.StatusOK, dto.ConnectInstanceResponse{
		Message:   "instance already connected",
		Connected: true,
	})
}

// ConnectQRBuffer godoc
// @Summary      Get QR code as PNG image
// @Description  Returns the QR code as a raw PNG image buffer. Returns 204 No Content if already connected.
// @Tags         Instance
// @Produce      png
// @Security     ApiKeyAuth
// @Param        id  path      string  true  "Instance ID"
// @Success      200  {file}    binary  "QR code PNG image"
// @Success      204  "Instance already connected"
// @Failure      404  {object}  utils.HTTPErrorResponse
// @Failure      422  {object}  utils.HTTPErrorResponse
// @Failure      500  {object}  utils.HTTPErrorResponse
// @Router       /instance/connect/{id}/image [get]
func (s *Instance) ConnectQRBuffer(ctx echo.Context) error {
	c := ctx.Request().Context()
	var request dto.ConnectInstanceRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	result, err := s.repo.List(c, request.ID)
	if err != nil {
		zap.L().Error("failed to list instances", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to list instances")
	}

	if len(result) == 0 {
		return utils.HTTPFail(ctx, http.StatusNotFound, err, "instance not found")
	}

	qrCode, _, err := s.whatsmiau.Connect(c, request.ID, "")
	if err != nil {
		zap.L().Error("failed to connect instance", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to connect instance")
	}
	if qrCode != "" {
		png, err := qrcode.Encode(qrCode, qrcode.Medium, 256)
		if err != nil {
			zap.L().Error("failed to encode qrcode", zap.Error(err))
			return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to encode qrcode")
		}
		return ctx.Blob(http.StatusOK, "image/png", png)
	}

	return ctx.NoContent(http.StatusOK)
}

// Status godoc
// @Summary      Get instance connection state
// @Description  Returns the current connection state of the specified instance
// @Tags         Instance
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path      string  true  "Instance ID"
// @Success      200  {object}  dto.StatusInstanceResponse
// @Failure      404  {object}  utils.HTTPErrorResponse
// @Failure      422  {object}  utils.HTTPErrorResponse
// @Failure      500  {object}  utils.HTTPErrorResponse
// @Router       /instance/{id}/status [get]
// @Router       /instance/connectionState/{id} [get]
func (s *Instance) Status(ctx echo.Context) error {
	c := ctx.Request().Context()
	var request dto.ConnectInstanceRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	result, err := s.repo.List(c, request.ID)
	if err != nil {
		zap.L().Error("failed to list instances", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to list instances")
	}

	if len(result) == 0 {
		return utils.HTTPFail(ctx, http.StatusNotFound, err, "instance not found")
	}

	status, err := s.whatsmiau.Status(request.ID)
	if err != nil {
		zap.L().Error("failed to get status instance", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to get status instance")
	}

	return ctx.JSON(http.StatusOK, dto.StatusInstanceResponse{
		ID:     request.ID,
		Status: string(status),
		Instance: &dto.StatusInstanceResponseEvolutionCompatibility{
			InstanceName: request.ID,
			State:        string(status),
		},
	})
}

// Logout godoc
// @Summary      Logout an instance
// @Description  Disconnects the WhatsApp session for the given instance without deleting it
// @Tags         Instance
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path      string  true  "Instance ID"
// @Success      200  {object}  dto.DeleteInstanceResponse
// @Failure      404  {object}  utils.HTTPErrorResponse
// @Failure      422  {object}  utils.HTTPErrorResponse
// @Failure      500  {object}  utils.HTTPErrorResponse
// @Router       /instance/{id}/logout [post]
// @Router       /instance/logout/{id} [delete]
func (s *Instance) Logout(ctx echo.Context) error {
	c := ctx.Request().Context()
	var request dto.DeleteInstanceRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	result, err := s.repo.List(c, request.ID)
	if err != nil {
		zap.L().Error("failed to list instances", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to list instances")
	}

	if len(result) == 0 {
		return utils.HTTPFail(ctx, http.StatusNotFound, err, "instance not found")
	}

	if err := s.whatsmiau.Logout(c, request.ID); err != nil {
		zap.L().Error("failed to logout instance", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to logout instance")
	}

	return ctx.JSON(http.StatusOK, dto.DeleteInstanceResponse{
		Message: "instance logout successfully",
	})
}

// Delete godoc
// @Summary      Delete an instance
// @Description  Disconnects and permanently removes the specified instance
// @Tags         Instance
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path      string  true  "Instance ID"
// @Success      200  {object}  dto.DeleteInstanceResponse
// @Failure      422  {object}  utils.HTTPErrorResponse
// @Failure      500  {object}  utils.HTTPErrorResponse
// @Router       /instance/{id} [delete]
// @Router       /instance/delete/{id} [delete]
func (s *Instance) Delete(ctx echo.Context) error {
	c := ctx.Request().Context()
	var request dto.DeleteInstanceRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	result, err := s.repo.List(c, request.ID)
	if err != nil {
		zap.L().Error("failed to list instances", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to list instances")
	}

	if len(result) == 0 {
		return ctx.JSON(http.StatusOK, dto.DeleteInstanceResponse{
			Message: "instance doesn't exists",
		})
	}

	if err := s.whatsmiau.Logout(ctx.Request().Context(), request.ID); err != nil {
		zap.L().Error("failed to disconnect instance", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to logout instance")
	}

	if err := s.repo.Delete(c, request.ID); err != nil {
		zap.L().Error("failed to delete instance", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to delete instance")
	}

	return ctx.JSON(http.StatusOK, dto.DeleteInstanceResponse{
		Message: "instance deleted",
	})
}

// Restart godoc
// @Summary      Restart an instance connection
// @Description  Disconnects and reconnects the WhatsApp websocket without logging out
// @Tags         Instance
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Instance ID"
// @Success      200  {object}  dto.RestartInstanceResponse
// @Failure      400  {object}  utils.HTTPErrorResponse
// @Failure      404  {object}  utils.HTTPErrorResponse
// @Failure      422  {object}  utils.HTTPErrorResponse
// @Failure      500  {object}  utils.HTTPErrorResponse
// @Router       /instance/{id}/restart [post]
// @Router       /instance/restart/{id} [post]
func (s *Instance) Restart(ctx echo.Context) error {
	c := ctx.Request().Context()
	var request dto.RestartInstanceRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	result, err := s.repo.List(c, request.ID)
	if err != nil {
		zap.L().Error("failed to list instances", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to list instances")
	}
	if len(result) == 0 {
		return utils.HTTPFail(ctx, http.StatusNotFound, nil, "instance not found")
	}

	if err := s.whatsmiau.Restart(c, request.ID); err != nil {
		zap.L().Error("failed to restart instance", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to restart instance")
	}

	return ctx.JSON(http.StatusOK, dto.RestartInstanceResponse{
		ID:     request.ID,
		Status: string(whatsmiau.Connecting),
		Instance: &dto.RestartInstanceEvoCompatibility{
			InstanceName: request.ID,
			Status:       string(whatsmiau.Connecting),
		},
	})
}
