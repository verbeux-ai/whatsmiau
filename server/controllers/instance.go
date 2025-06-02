package controllers

import (
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/interface"
	"github.com/verbeux-ai/whatsmiau/lib"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.uber.org/zap"
	"net/http"
)

type Instance struct {
	repo      _interface.InstanceRepository
	whatsmiau *lib.Whatsmiau
}

func NewInstances(repository _interface.InstanceRepository, whatsmiau *lib.Whatsmiau) *Instance {
	return &Instance{
		repo:      repository,
		whatsmiau: whatsmiau,
	}
}

func (s *Instance) Create(ctx echo.Context) error {
	var request dto.CreateInstanceRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	request.ID = request.InstanceName
	request.Instance.ID = request.ID

	c := ctx.Request().Context()
	if err := s.repo.Create(c, &request.Instance); err != nil {
		zap.L().Error("failed to create instance", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to create instance")
	}

	return ctx.JSON(http.StatusCreated, dto.CreateInstanceResponse{
		Instance: request.Instance,
	})
}

func (s *Instance) List(ctx echo.Context) error {
	c := ctx.Request().Context()
	result, err := s.repo.List(c)
	if err != nil {
		zap.L().Error("failed to list instances", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to list instances")
	}

	return ctx.JSON(http.StatusOK, dto.ListInstancesResponse{
		Instances: result,
	})
}
