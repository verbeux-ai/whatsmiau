package controllers

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	msgrepo "github.com/verbeux-ai/whatsmiau/repositories/messages"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/services"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
)

type Chat struct {
	repo      interfaces.InstanceRepository
	whatsmiau *whatsmiau.Whatsmiau
}

func NewChats(repository interfaces.InstanceRepository, whatsmiau *whatsmiau.Whatsmiau) *Chat {
	return &Chat{
		repo:      repository,
		whatsmiau: whatsmiau,
	}
}

func (s *Chat) ReadMessages(ctx echo.Context) error {
	var request dto.ReadMessagesRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	result := make(map[string][]string)
	for _, msg := range request.ReadMessages {
		result[msg.RemoteJid] = append(result[msg.RemoteJid], msg.ID)
	}

	for remoteJid, msgs := range result {
		number, err := numberToJid(remoteJid)
		if err != nil {
			zap.L().Error("error converting number to jid", zap.Error(err))
			continue
		}

		if err := s.whatsmiau.ReadMessage(&whatsmiau.ReadMessageRequest{
			MessageIDs: msgs,
			InstanceID: request.InstanceID,
			RemoteJID:  number,
			Sender:     nil,
		}); err != nil {
			zap.L().Error("Whatsmiau.ReadMessages failed", zap.Error(err))
		}
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{})
}

func (s *Chat) SendChatPresence(ctx echo.Context) error {
	var request dto.SendChatPresenceRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	number, err := numberToJid(request.Number)
	if err != nil {
		zap.L().Error("error converting number to jid", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid number format")
	}

	var presence types.ChatPresence
	switch request.Presence {
	case dto.PresenceComposing:
		presence = types.ChatPresenceComposing
	case dto.PresenceAvailable:
		presence = types.ChatPresencePaused
	}

	presenceType := types.ChatPresenceMediaText
	if request.Type == dto.PresenceTypeAudio {
		presenceType = types.ChatPresenceMediaAudio
	}

	if request.Delay > 0 {
		go func() {
			time.Sleep(time.Duration(request.Delay) * time.Millisecond)
			if err := s.whatsmiau.ChatPresence(&whatsmiau.ChatPresenceRequest{
				InstanceID: request.InstanceID,
				RemoteJID:  number,
				Presence:   types.ChatPresencePaused,
				Media:      types.ChatPresenceMediaText,
			}); err != nil {
				zap.L().Error("Whatsmiau.ReadMessages failed", zap.Error(err))
			}
		}()
	}

	if err := s.whatsmiau.ChatPresence(&whatsmiau.ChatPresenceRequest{
		InstanceID: request.InstanceID,
		RemoteJID:  number,
		Presence:   presence,
		Media:      presenceType,
	}); err != nil {
		zap.L().Error("Whatsmiau.ReadMessages failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "Whatsmiau.ChatPresence failed")
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{})
}

func (s *Chat) GetContacts(ctx echo.Context) error {
	instanceID := ctx.Param("instance")
	if instanceID == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "instance ID is required in the URL path")
	}

	contacts, err := s.whatsmiau.GetAllContacts(ctx.Request().Context(), instanceID)
	if err != nil {
		zap.L().Error("Whatsmiau.GetAllContacts failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to get contacts")
	}

	return ctx.JSON(http.StatusOK, contacts)
}

func (s *Chat) GetMessages(ctx echo.Context) error {
	instanceID := ctx.Param("instance")
	if instanceID == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "instance ID is required in the URL path")
	}
	remoteJid := ctx.Param("remoteJid")
	if remoteJid == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "remoteJid is required in the URL path")
	}

	beforeStr := ctx.QueryParam("before")
	limitStr := ctx.QueryParam("limit")

	limit := 50
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			limit = n
		}
	}
	before, err := msgrepo.ParseBeforeParam(beforeStr)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid before")
	}

	store := services.MessageStore()
	msgs, err := store.List(ctx.Request().Context(), instanceID, remoteJid, before, limit)
	if err != nil {
		zap.L().Error("MessageStore.List failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to list messages")
	}

	return ctx.JSON(http.StatusOK, msgs)
}

func (s *Chat) GetGroups(ctx echo.Context) error {
	instanceID := ctx.Param("instance")
	if instanceID == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "instance ID is required in the URL path")
	}

	groups, err := s.whatsmiau.GetGroups(ctx.Request().Context(), instanceID)
	if err != nil {
		zap.L().Error("Whatsmiau.GetGroups failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to get groups")
	}

	return ctx.JSON(http.StatusOK, groups)
}

func (s *Chat) GetGroupInfo(ctx echo.Context) error {
	instanceID := ctx.Param("instance")
	if instanceID == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "instance ID is required in the URL path")
	}
	groupJid := ctx.Param("groupJid")
	if groupJid == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "groupJid is required in the URL path")
	}
	if decoded, err := url.PathUnescape(groupJid); err == nil && decoded != "" {
		groupJid = decoded
	}

	jid, err := numberToJid(groupJid)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid group jid format")
	}
	includeParticipants := false
	if p := ctx.QueryParam("participants"); p == "1" || p == "true" || p == "yes" {
		includeParticipants = true
	}

	info, err := s.whatsmiau.GetGroupInfo(ctx.Request().Context(), instanceID, *jid, includeParticipants)
	if err != nil {
		zap.L().Error("Whatsmiau.GetGroupInfo failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to get group info")
	}
	if info == nil {
		return utils.HTTPFail(ctx, http.StatusNotFound, nil, "group not found")
	}

	return ctx.JSON(http.StatusOK, info)
}

func (s *Chat) NumberExists(ctx echo.Context) error {
	instanceID := ctx.Param("instance")
	if instanceID == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "instance ID is required in the URL path")
	}

	var request dto.NumberExistsRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	response, err := s.whatsmiau.NumberExists(ctx.Request().Context(), &whatsmiau.NumberExistsRequest{
		InstanceID: instanceID,
		Numbers:    request.Numbers,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.NumberExists failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to check numbers")
	}

	return ctx.JSON(http.StatusOK, response)
}
