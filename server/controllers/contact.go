package controllers

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.mau.fi/whatsmeow"
	"go.uber.org/zap"
)

type contactLister interface {
	ListContacts(ctx context.Context, instanceID string, page, limit int) (*whatsmiau.ContactListResult, error)
}

type Contact struct {
	repo     interfaces.InstanceRepository
	service  contactLister
	validate *validator.Validate
}

func NewContacts(repository interfaces.InstanceRepository, whatsmiau *whatsmiau.Whatsmiau) *Contact {
	return &Contact{
		repo:     repository,
		service:  whatsmiau,
		validate: validator.New(),
	}
}

// List godoc
// @Summary      List synchronized contacts from an instance
// @Description  Returns the locally synchronized WhatsApp contacts for the specified connected instance
// @Tags         Contact
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                    true  "Instance ID"
// @Param        page      query     int                       true  "Page number (>= 1)"
// @Param        limit     query     int                       true  "Page size (1-100)"
// @Success      200       {object}  dto.ListContactsResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      404       {object}  utils.HTTPErrorResponse
// @Failure      409       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/contacts [get]
func (s *Contact) List(ctx echo.Context) error {
	var request dto.ListContactsRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request query")
	}
	if err := s.validate.Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request query")
	}

	c := ctx.Request().Context()
	result, err := s.repo.List(c, request.InstanceID)
	if err != nil {
		zap.L().Error("failed to list instances", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to get instance")
	}
	if len(result) == 0 {
		return utils.HTTPFail(ctx, http.StatusNotFound, nil, "instance not found")
	}

	contacts, err := s.service.ListContacts(c, request.InstanceID, *request.Page, *request.Limit)
	if err != nil {
		switch {
		case errors.Is(err, whatsmiau.ErrInvalidPagination):
			return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request query")
		case errors.Is(err, whatsmeow.ErrClientIsNil):
			return utils.HTTPFail(ctx, http.StatusConflict, err, "instance is not connected")
		default:
			zap.L().Error("failed to list contacts from local store",
				zap.String("instance", request.InstanceID),
				zap.Int("page", *request.Page),
				zap.Int("limit", *request.Limit),
				zap.Error(err),
			)
			return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to read contacts from local store")
		}
	}

	response := dto.ListContactsResponse{
		Data: make([]dto.ContactResponse, 0, len(contacts.Data)),
		Pagination: dto.ContactsPaginationResponse{
			Page:       contacts.Page,
			Limit:      contacts.Limit,
			Total:      contacts.Total,
			TotalPages: contacts.TotalPages,
		},
	}
	for _, contact := range contacts.Data {
		response.Data = append(response.Data, dto.ContactResponse{
			ID:            contact.ID,
			RemoteJID:     contact.RemoteJID,
			PushName:      contact.PushName,
			ProfilePicURL: contact.ProfilePicURL,
			CreatedAt:     contact.CreatedAt,
			UpdatedAt:     contact.UpdatedAt,
			InstanceID:    contact.InstanceID,
			IsGroup:       contact.IsGroup,
			IsSaved:       contact.IsSaved,
			Type:          contact.Type,
		})
	}

	return ctx.JSON(http.StatusOK, response)
}
