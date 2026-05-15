package controllers

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.uber.org/zap"
)

type Community struct {
	repo      interfaces.InstanceRepository
	whatsmiau *whatsmiau.Whatsmiau
}

func NewCommunities(repository interfaces.InstanceRepository, whatsmiau *whatsmiau.Whatsmiau) *Community {
	return &Community{
		repo:      repository,
		whatsmiau: whatsmiau,
	}
}

// CreateCommunity godoc
// @Summary      Create a WhatsApp community (parent group)
// @Tags         Community
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path  string                   true  "Instance ID"
// @Param        body      body  dto.CreateCommunityRequest true  "Community payload"
// @Success      201       {object}  whatsmiau.GroupInfoResponse
// @Router       /instance/{instance}/community/create [post]
func (s *Community) CreateCommunity(ctx echo.Context) error {
	var request dto.CreateCommunityRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	resp, err := s.whatsmiau.CreateCommunity(ctx.Request().Context(), &whatsmiau.CreateCommunityRequest{
		InstanceID:  request.InstanceID,
		Subject:     request.Subject,
		Description: request.Description,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.CreateCommunity failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusCreated, resp)
}

// CreateSubGroup godoc
// @Summary      Create a new group already linked to a community
// @Tags         Community
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path  string                            true  "Instance ID"
// @Param        body      body  dto.CreateCommunitySubGroupRequest true  "Sub group payload"
// @Success      201       {object}  whatsmiau.GroupInfoResponse
// @Router       /instance/{instance}/community/createSubGroup [post]
func (s *Community) CreateSubGroup(ctx echo.Context) error {
	var request dto.CreateCommunitySubGroupRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	parentJid, err := parseGroupJID(request.ParentJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid parentJid")
	}

	resp, err := s.whatsmiau.CreateCommunitySubGroup(ctx.Request().Context(), &whatsmiau.CreateSubGroupRequest{
		InstanceID:   request.InstanceID,
		Subject:      request.Subject,
		ParentJID:    parentJid,
		Participants: request.Participants,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.CreateCommunitySubGroup failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusCreated, resp)
}

// LinkGroup godoc
// @Summary      Link an existing group to a community
// @Tags         Community
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path  string                       true  "Instance ID"
// @Param        body      body  dto.CommunityLinkGroupRequest true  "Link payload"
// @Success      200       {object}  map[string]interface{}
// @Router       /instance/{instance}/community/linkGroup [post]
func (s *Community) LinkGroup(ctx echo.Context) error {
	var request dto.CommunityLinkGroupRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	parentJid, err := parseGroupJID(request.ParentJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid parentJid")
	}
	childJid, err := parseGroupJID(request.ChildJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid childJid")
	}

	if err := s.whatsmiau.LinkCommunityGroup(ctx.Request().Context(), &whatsmiau.LinkGroupRequest{
		InstanceID: request.InstanceID,
		ParentJID:  parentJid,
		ChildJID:   childJid,
	}); err != nil {
		zap.L().Error("Whatsmiau.LinkCommunityGroup failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusOK, map[string]interface{}{})
}

// UnlinkGroup godoc
// @Summary      Remove a group from a community
// @Tags         Community
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path  string                       true  "Instance ID"
// @Param        body      body  dto.CommunityLinkGroupRequest true  "Unlink payload"
// @Success      200       {object}  map[string]interface{}
// @Router       /instance/{instance}/community/unlinkGroup [post]
func (s *Community) UnlinkGroup(ctx echo.Context) error {
	var request dto.CommunityLinkGroupRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	parentJid, err := parseGroupJID(request.ParentJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid parentJid")
	}
	childJid, err := parseGroupJID(request.ChildJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid childJid")
	}

	if err := s.whatsmiau.UnlinkCommunityGroup(ctx.Request().Context(), &whatsmiau.LinkGroupRequest{
		InstanceID: request.InstanceID,
		ParentJID:  parentJid,
		ChildJID:   childJid,
	}); err != nil {
		zap.L().Error("Whatsmiau.UnlinkCommunityGroup failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusOK, map[string]interface{}{})
}

// SubGroups godoc
// @Summary      List subgroups of a community
// @Tags         Community
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance      path  string true  "Instance ID"
// @Param        communityJid  query string true  "Community JID"
// @Success      200  {array}  whatsmiau.SubGroupResponse
// @Router       /instance/{instance}/community/subGroups [get]
func (s *Community) SubGroups(ctx echo.Context) error {
	var request dto.CommunityJidQuery
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}
	communityJid, err := parseGroupJID(request.CommunityJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid communityJid")
	}

	resp, err := s.whatsmiau.GetCommunitySubGroups(ctx.Request().Context(), &whatsmiau.SubGroupsRequest{
		InstanceID:   request.InstanceID,
		CommunityJID: communityJid,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.GetCommunitySubGroups failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// LinkedGroupsParticipants godoc
// @Summary      List participants across all linked groups of a community
// @Tags         Community
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance      path  string true  "Instance ID"
// @Param        communityJid  query string true  "Community JID"
// @Success      200  {array}  whatsmiau.LinkedParticipantResponse
// @Router       /instance/{instance}/community/linkedGroupsParticipants [get]
func (s *Community) LinkedGroupsParticipants(ctx echo.Context) error {
	var request dto.CommunityJidQuery
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}
	communityJid, err := parseGroupJID(request.CommunityJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid communityJid")
	}

	resp, err := s.whatsmiau.GetCommunityLinkedGroupsParticipants(ctx.Request().Context(), &whatsmiau.LinkedGroupsParticipantsRequest{
		InstanceID:   request.InstanceID,
		CommunityJID: communityJid,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.GetCommunityLinkedGroupsParticipants failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// SetJoinApprovalMode godoc
// @Summary      Toggle join-approval mode for a community
// @Tags         Community
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path  string                                 true  "Instance ID"
// @Param        body      body  dto.CommunitySetJoinApprovalModeRequest true  "Mode payload"
// @Success      201       {object}  map[string]interface{}
// @Router       /instance/{instance}/community/setJoinApprovalMode [post]
func (s *Community) SetJoinApprovalMode(ctx echo.Context) error {
	var request dto.CommunitySetJoinApprovalModeRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	communityJid, err := parseGroupJID(request.CommunityJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid communityJid")
	}

	if err := s.whatsmiau.SetCommunityJoinApprovalMode(ctx.Request().Context(), &whatsmiau.SetJoinApprovalModeRequest{
		InstanceID:   request.InstanceID,
		CommunityJID: communityJid,
		Mode:         request.Mode,
	}); err != nil {
		zap.L().Error("Whatsmiau.SetCommunityJoinApprovalMode failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusCreated, map[string]interface{}{})
}

// SetMemberAddMode godoc
// @Summary      Toggle who can add members (admins only or all)
// @Tags         Community
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path  string                              true  "Instance ID"
// @Param        body      body  dto.CommunitySetMemberAddModeRequest true  "Mode payload"
// @Success      201       {object}  map[string]interface{}
// @Router       /instance/{instance}/community/setMemberAddMode [post]
func (s *Community) SetMemberAddMode(ctx echo.Context) error {
	var request dto.CommunitySetMemberAddModeRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	communityJid, err := parseGroupJID(request.CommunityJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid communityJid")
	}

	if err := s.whatsmiau.SetCommunityMemberAddMode(ctx.Request().Context(), &whatsmiau.SetMemberAddModeRequest{
		InstanceID:   request.InstanceID,
		CommunityJID: communityJid,
		Mode:         request.Mode,
	}); err != nil {
		zap.L().Error("Whatsmiau.SetCommunityMemberAddMode failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusCreated, map[string]interface{}{})
}

// RequestParticipants godoc
// @Summary      List pending join requests
// @Tags         Community
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance      path  string true  "Instance ID"
// @Param        communityJid  query string true  "Community JID"
// @Success      200  {array}  whatsmiau.RequestParticipantResponse
// @Router       /instance/{instance}/community/requestParticipants [get]
func (s *Community) RequestParticipants(ctx echo.Context) error {
	var request dto.CommunityJidQuery
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}
	communityJid, err := parseGroupJID(request.CommunityJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid communityJid")
	}

	resp, err := s.whatsmiau.GetCommunityRequestParticipants(ctx.Request().Context(), &whatsmiau.RequestParticipantsListRequest{
		InstanceID:   request.InstanceID,
		CommunityJID: communityJid,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.GetCommunityRequestParticipants failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// UpdateRequestParticipants godoc
// @Summary      Approve or reject pending join requests
// @Tags         Community
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path  string                                          true  "Instance ID"
// @Param        body      body  dto.CommunityUpdateRequestParticipantsRequest true  "Update payload"
// @Success      201       {object}  whatsmiau.UpdateParticipantResponse
// @Router       /instance/{instance}/community/requestParticipants/update [post]
func (s *Community) UpdateRequestParticipants(ctx echo.Context) error {
	var request dto.CommunityUpdateRequestParticipantsRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	communityJid, err := parseGroupJID(request.CommunityJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid communityJid")
	}

	resp, err := s.whatsmiau.UpdateCommunityRequestParticipants(ctx.Request().Context(), &whatsmiau.UpdateRequestParticipantsRequest{
		InstanceID:   request.InstanceID,
		CommunityJID: communityJid,
		Action:       request.Action,
		Participants: request.Participants,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.UpdateCommunityRequestParticipants failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusCreated, resp)
}
