package controllers

import (
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

type Group struct {
	repo      interfaces.InstanceRepository
	whatsmiau *whatsmiau.Whatsmiau
}

func NewGroups(repository interfaces.InstanceRepository, whatsmiau *whatsmiau.Whatsmiau) *Group {
	return &Group{
		repo:      repository,
		whatsmiau: whatsmiau,
	}
}

func mapGroupError(err error) (int, string) {
	switch {
	case errors.Is(err, whatsmeow.ErrClientIsNil):
		return http.StatusNotFound, "instance is not connected"
	case errors.Is(err, whatsmeow.ErrGroupNotFound):
		return http.StatusNotFound, "group not found"
	case errors.Is(err, whatsmeow.ErrNotInGroup):
		return http.StatusForbidden, "not a member of this group"
	case errors.Is(err, whatsmeow.ErrInviteLinkRevoked):
		return http.StatusGone, "invite link revoked"
	case errors.Is(err, whatsmeow.ErrInviteLinkInvalid):
		return http.StatusBadRequest, "invite link invalid"
	case errors.Is(err, whatsmeow.ErrGroupInviteLinkUnauthorized):
		return http.StatusForbidden, "unauthorized to access invite link"
	case errors.Is(err, whatsmeow.ErrIQRateOverLimit):
		return http.StatusTooManyRequests, "rate limit exceeded, try again later"
	default:
		return http.StatusInternalServerError, "operation failed"
	}
}

// CreateGroup godoc
// @Summary      Create a WhatsApp group
// @Tags         Group
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                 true  "Instance ID"
// @Param        body      body      dto.CreateGroupRequest true  "Group payload"
// @Success      201       {object}  whatsmiau.GroupInfoResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/group/create [post]
// @Router       /group/create/{instance} [post]
func (s *Group) CreateGroup(ctx echo.Context) error {
	var request dto.CreateGroupRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	resp, err := s.whatsmiau.CreateGroup(ctx.Request().Context(), &whatsmiau.CreateGroupRequest{
		InstanceID:          request.InstanceID,
		Subject:             request.Subject,
		Description:         request.Description,
		Participants:        request.Participants,
		PromoteParticipants: request.PromoteParticipants,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.CreateGroup failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusCreated, resp)
}

// UpdateGroupSubject godoc
// @Summary      Update group subject (name)
// @Tags         Group
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                  true  "Instance ID"
// @Param        body      body      dto.GroupSubjectRequest true  "Subject payload"
// @Success      201       {object}  map[string]interface{}
// @Router       /instance/{instance}/group/updateGroupSubject [post]
// @Router       /group/updateGroupSubject/{instance} [post]
func (s *Group) UpdateGroupSubject(ctx echo.Context) error {
	var request dto.GroupSubjectRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	groupJid, err := parseGroupJID(request.GroupJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid groupJid")
	}

	if err := s.whatsmiau.SetGroupSubject(ctx.Request().Context(), &whatsmiau.SetGroupSubjectRequest{
		InstanceID: request.InstanceID,
		GroupJID:   groupJid,
		Subject:    request.Subject,
	}); err != nil {
		zap.L().Error("Whatsmiau.SetGroupSubject failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusCreated, map[string]interface{}{})
}

// UpdateGroupPicture godoc
// @Summary      Update group picture
// @Tags         Group
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                  true  "Instance ID"
// @Param        body      body      dto.GroupPictureRequest true  "Picture payload (base64 or URL)"
// @Success      201       {object}  whatsmiau.SetGroupPictureResponse
// @Router       /instance/{instance}/group/updateGroupPicture [post]
// @Router       /group/updateGroupPicture/{instance} [post]
func (s *Group) UpdateGroupPicture(ctx echo.Context) error {
	var request dto.GroupPictureRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	groupJid, err := parseGroupJID(request.GroupJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid groupJid")
	}

	resp, err := s.whatsmiau.SetGroupPicture(ctx.Request().Context(), &whatsmiau.SetGroupPictureRequest{
		InstanceID: request.InstanceID,
		GroupJID:   groupJid,
		Image:      request.Image,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.SetGroupPicture failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusCreated, resp)
}

// UpdateGroupDescription godoc
// @Summary      Update group description
// @Tags         Group
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                      true  "Instance ID"
// @Param        body      body      dto.GroupDescriptionRequest true  "Description payload"
// @Success      201       {object}  map[string]interface{}
// @Router       /instance/{instance}/group/updateGroupDescription [post]
// @Router       /group/updateGroupDescription/{instance} [post]
func (s *Group) UpdateGroupDescription(ctx echo.Context) error {
	var request dto.GroupDescriptionRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	groupJid, err := parseGroupJID(request.GroupJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid groupJid")
	}

	if err := s.whatsmiau.SetGroupDescription(ctx.Request().Context(), &whatsmiau.SetGroupDescriptionRequest{
		InstanceID:  request.InstanceID,
		GroupJID:    groupJid,
		Description: request.Description,
	}); err != nil {
		zap.L().Error("Whatsmiau.SetGroupDescription failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusCreated, map[string]interface{}{})
}

// FindGroupInfos godoc
// @Summary      Get group info by JID
// @Tags         Group
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path   string true  "Instance ID"
// @Param        groupJid  query  string true  "Group JID"
// @Success      200  {object}  whatsmiau.GroupInfoResponse
// @Router       /instance/{instance}/group/findGroupInfos [get]
// @Router       /group/findGroupInfos/{instance} [get]
func (s *Group) FindGroupInfos(ctx echo.Context) error {
	var request dto.GroupJidQuery
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}
	groupJid, err := parseGroupJID(request.GroupJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid groupJid")
	}

	resp, err := s.whatsmiau.FindGroup(ctx.Request().Context(), &whatsmiau.FindGroupRequest{
		InstanceID: request.InstanceID,
		GroupJID:   groupJid,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.FindGroup failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// FetchAllGroups godoc
// @Summary      List all joined groups
// @Tags         Group
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance         path   string true  "Instance ID"
// @Param        getParticipants  query  string false "Include participants (true/false)"
// @Success      200  {array}  whatsmiau.GroupInfoResponse
// @Router       /instance/{instance}/group/fetchAllGroups [get]
// @Router       /group/fetchAllGroups/{instance} [get]
func (s *Group) FetchAllGroups(ctx echo.Context) error {
	var request dto.FetchAllGroupsQuery
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}

	resp, err := s.whatsmiau.FetchAllGroups(ctx.Request().Context(), &whatsmiau.FetchAllGroupsRequest{
		InstanceID:      request.InstanceID,
		GetParticipants: request.GetParticipants == "true",
	})
	if err != nil {
		zap.L().Error("Whatsmiau.FetchAllGroups failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// FindParticipants godoc
// @Summary      List participants of a group
// @Tags         Group
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path   string true  "Instance ID"
// @Param        groupJid  query  string true  "Group JID"
// @Success      200  {object}  whatsmiau.FindParticipantsResponse
// @Router       /instance/{instance}/group/participants [get]
// @Router       /group/participants/{instance} [get]
func (s *Group) FindParticipants(ctx echo.Context) error {
	var request dto.GroupJidQuery
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}
	groupJid, err := parseGroupJID(request.GroupJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid groupJid")
	}

	resp, err := s.whatsmiau.FindParticipants(ctx.Request().Context(), &whatsmiau.FindParticipantsRequest{
		InstanceID: request.InstanceID,
		GroupJID:   groupJid,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.FindParticipants failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// InviteCode godoc
// @Summary      Get the group invite code (link)
// @Tags         Group
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path   string true  "Instance ID"
// @Param        groupJid  query  string true  "Group JID"
// @Success      200  {object}  whatsmiau.InviteCodeResponse
// @Router       /instance/{instance}/group/inviteCode [get]
// @Router       /group/inviteCode/{instance} [get]
func (s *Group) InviteCode(ctx echo.Context) error {
	var request dto.GroupJidQuery
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}
	groupJid, err := parseGroupJID(request.GroupJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid groupJid")
	}

	resp, err := s.whatsmiau.GetGroupInviteCode(ctx.Request().Context(), &whatsmiau.InviteCodeRequest{
		InstanceID: request.InstanceID,
		GroupJID:   groupJid,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.GetGroupInviteCode failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// InviteInfo godoc
// @Summary      Get info about an invite code (no join)
// @Tags         Group
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance    path   string true  "Instance ID"
// @Param        inviteCode  query  string true  "Invite code or full link"
// @Success      200  {object}  whatsmiau.GroupInfoResponse
// @Router       /instance/{instance}/group/inviteInfo [get]
// @Router       /group/inviteInfo/{instance} [get]
func (s *Group) InviteInfo(ctx echo.Context) error {
	var request dto.GroupInviteCodeQuery
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}

	resp, err := s.whatsmiau.GetGroupInviteInfo(ctx.Request().Context(), &whatsmiau.InviteInfoRequest{
		InstanceID: request.InstanceID,
		Code:       request.InviteCode,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.GetGroupInviteInfo failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// AcceptInviteCode godoc
// @Summary      Join a group using an invite code
// @Tags         Group
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance    path   string true  "Instance ID"
// @Param        inviteCode  query  string true  "Invite code or full link"
// @Success      200  {object}  whatsmiau.AcceptGroupInviteResponse
// @Router       /instance/{instance}/group/acceptInviteCode [get]
// @Router       /group/acceptInviteCode/{instance} [get]
func (s *Group) AcceptInviteCode(ctx echo.Context) error {
	var request dto.GroupInviteCodeQuery
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}

	resp, err := s.whatsmiau.AcceptGroupInvite(ctx.Request().Context(), &whatsmiau.AcceptGroupInviteRequest{
		InstanceID: request.InstanceID,
		Code:       request.InviteCode,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.AcceptGroupInvite failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// SendInvite godoc
// @Summary      Send group invite link via text to phone numbers
// @Tags         Group
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                     true  "Instance ID"
// @Param        body      body      dto.GroupSendInviteRequest true  "Invite payload"
// @Success      200       {object}  whatsmiau.SendGroupInviteResponse
// @Router       /instance/{instance}/group/sendInvite [post]
// @Router       /group/sendInvite/{instance} [post]
func (s *Group) SendInvite(ctx echo.Context) error {
	var request dto.GroupSendInviteRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	groupJid, err := parseGroupJID(request.GroupJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid groupJid")
	}

	resp, err := s.whatsmiau.SendGroupInvite(ctx.Request().Context(), &whatsmiau.SendGroupInviteRequest{
		InstanceID:  request.InstanceID,
		GroupJID:    groupJid,
		Description: request.Description,
		Numbers:     request.Numbers,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.SendGroupInvite failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// RevokeInviteCode godoc
// @Summary      Revoke current invite code and generate a new one
// @Tags         Group
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path  string true  "Instance ID"
// @Param        body      body  dto.GroupRevokeInviteRequest true  "Group payload"
// @Success      201       {object}  whatsmiau.InviteCodeResponse
// @Router       /instance/{instance}/group/revokeInviteCode [post]
// @Router       /group/revokeInviteCode/{instance} [post]
func (s *Group) RevokeInviteCode(ctx echo.Context) error {
	var request dto.GroupRevokeInviteRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}
	groupJid, err := parseGroupJID(request.GroupJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid groupJid")
	}

	resp, err := s.whatsmiau.RevokeGroupInviteCode(ctx.Request().Context(), &whatsmiau.InviteCodeRequest{
		InstanceID: request.InstanceID,
		GroupJID:   groupJid,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.RevokeGroupInviteCode failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusCreated, resp)
}

// UpdateParticipant godoc
// @Summary      Update group participants (add/remove/promote/demote)
// @Tags         Group
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path  string true  "Instance ID"
// @Param        body      body  dto.GroupUpdateParticipantRequest true  "Update payload"
// @Success      201       {object}  whatsmiau.UpdateParticipantResponse
// @Router       /instance/{instance}/group/updateParticipant [post]
// @Router       /group/updateParticipant/{instance} [post]
func (s *Group) UpdateParticipant(ctx echo.Context) error {
	var request dto.GroupUpdateParticipantRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	groupJid, err := parseGroupJID(request.GroupJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid groupJid")
	}

	resp, err := s.whatsmiau.UpdateGroupParticipant(ctx.Request().Context(), &whatsmiau.UpdateParticipantRequest{
		InstanceID:   request.InstanceID,
		GroupJID:     groupJid,
		Action:       request.Action,
		Participants: request.Participants,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.UpdateGroupParticipant failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusCreated, resp)
}

// UpdateSetting godoc
// @Summary      Update group setting (announce/locked)
// @Tags         Group
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path  string true  "Instance ID"
// @Param        body      body  dto.GroupUpdateSettingRequest true  "Setting payload"
// @Success      201       {object}  map[string]interface{}
// @Router       /instance/{instance}/group/updateSetting [post]
// @Router       /group/updateSetting/{instance} [post]
func (s *Group) UpdateSetting(ctx echo.Context) error {
	var request dto.GroupUpdateSettingRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	groupJid, err := parseGroupJID(request.GroupJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid groupJid")
	}

	if err := s.whatsmiau.UpdateGroupSetting(ctx.Request().Context(), &whatsmiau.UpdateGroupSettingRequest{
		InstanceID: request.InstanceID,
		GroupJID:   groupJid,
		Action:     request.Action,
	}); err != nil {
		zap.L().Error("Whatsmiau.UpdateGroupSetting failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusCreated, map[string]interface{}{})
}

// ToggleEphemeral godoc
// @Summary      Toggle disappearing messages for the group
// @Tags         Group
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path  string true  "Instance ID"
// @Param        body      body  dto.GroupToggleEphemeralRequest true  "Ephemeral payload"
// @Success      201       {object}  map[string]interface{}
// @Router       /instance/{instance}/group/toggleEphemeral [post]
// @Router       /group/toggleEphemeral/{instance} [post]
func (s *Group) ToggleEphemeral(ctx echo.Context) error {
	var request dto.GroupToggleEphemeralRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	groupJid, err := parseGroupJID(request.GroupJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid groupJid")
	}

	if err := s.whatsmiau.ToggleGroupEphemeral(ctx.Request().Context(), &whatsmiau.ToggleEphemeralRequest{
		InstanceID: request.InstanceID,
		GroupJID:   groupJid,
		Expiration: request.Expiration,
	}); err != nil {
		zap.L().Error("Whatsmiau.ToggleGroupEphemeral failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusCreated, map[string]interface{}{})
}

// LeaveGroup godoc
// @Summary      Leave a group
// @Tags         Group
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path   string true  "Instance ID"
// @Param        groupJid  query  string true  "Group JID"
// @Success      200       {object}  map[string]interface{}
// @Router       /instance/{instance}/group/leaveGroup [delete]
// @Router       /group/leaveGroup/{instance} [delete]
func (s *Group) LeaveGroup(ctx echo.Context) error {
	var request dto.GroupLeaveRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request")
	}
	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request")
	}
	groupJid, err := parseGroupJID(request.GroupJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid groupJid")
	}

	if err := s.whatsmiau.LeaveGroup(ctx.Request().Context(), &whatsmiau.LeaveGroupRequest{
		InstanceID: request.InstanceID,
		GroupJID:   groupJid,
	}); err != nil {
		zap.L().Error("Whatsmiau.LeaveGroup failed", zap.Error(err))
		code, msg := mapGroupError(err)
		return utils.HTTPFail(ctx, code, err, msg)
	}
	return ctx.JSON(http.StatusOK, map[string]interface{}{})
}
