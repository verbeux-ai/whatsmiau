package controllers

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/utils"
)

type Contact struct{ whatsmiau *whatsmiau.Whatsmiau }

func NewContacts(service *whatsmiau.Whatsmiau) *Contact { return &Contact{whatsmiau: service} }

func (s *Contact) List(ctx echo.Context) error {
	var request dto.ListContactsQuery
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}
	contacts, err := s.whatsmiau.ListContacts(ctx.Request().Context(), &whatsmiau.ListContactsRequest{InstanceID: request.InstanceID})
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusNotFound, err, "instance is not connected")
	}
	return ctx.JSON(http.StatusOK, contacts)
}
