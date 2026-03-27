package controllers

import (
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/dto"
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

// ReadMessages godoc
// @Summary      Mark messages as read
// @Description  Marks one or more messages as read in a WhatsApp conversation
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                   true  "Instance ID"
// @Param        body      body      dto.ReadMessagesRequest   true  "Messages to mark as read"
// @Success      200       {object}  map[string]interface{}   "Empty object on success"
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/chat/read-messages [post]
// @Router       /chat/markMessageAsRead/{instance} [post]
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

// SendChatPresence godoc
// @Summary      Send chat presence (typing indicator)
// @Description  Sends a presence status (composing/available) to a WhatsApp contact, with optional auto-stop delay
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                       true  "Instance ID"
// @Param        body      body      dto.SendChatPresenceRequest  true  "Presence parameters"
// @Success      200       {object}  map[string]interface{}       "Empty object on success"
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/chat/presence [post]
// @Router       /chat/sendPresence/{instance} [post]
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

// NumberExists godoc
// @Summary      Check if numbers exist on WhatsApp
// @Description  Checks whether the given phone numbers are registered on WhatsApp
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                    true  "Instance ID"
// @Param        body      body      dto.NumberExistsRequest    true  "Numbers to check"
// @Success      200       {array}   object                    "List of number existence results"
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /chat/whatsappNumbers/{instance} [post]
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

// DeleteMessageForEveryone godoc
// @Summary      Delete message for everyone
// @Description  Revokes a message in the chat so it is deleted for all participants (BuildRevoke + SendMessage)
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                             true  "Instance ID"
// @Param        body      body      dto.DeleteMessageForEveryoneRequest true  "Message to revoke"
// @Success      200       {object}  map[string]interface{}             "Empty object on success"
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/chat/deleteMessageForEveryone [delete]
// @Router       /chat/deleteMessageForEveryone/{instance} [delete]
func (s *Chat) DeleteMessageForEveryone(ctx echo.Context) error {
	var request dto.DeleteMessageForEveryoneRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	remoteJid, err := numberToJid(request.RemoteJid)
	if err != nil {
		zap.L().Error("error converting remoteJid to jid", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid remoteJid format")
	}

	if !request.FromMe && remoteJid.Server == types.GroupServer && request.Participant == "" {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "participant is required when deleting another user's message in a group")
	}

	var participantJid *types.JID
	if request.Participant != "" {
		p, err := numberToJid(request.Participant)
		if err != nil {
			zap.L().Error("error converting participant to jid", zap.Error(err))
			return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid participant format")
		}
		participantJid = p
	}

	c := ctx.Request().Context()
	if err := s.whatsmiau.DeleteMessageForEveryone(c, &whatsmiau.DeleteMessageForEveryoneRequest{
		InstanceID:       request.InstanceID,
		RemoteJID:        remoteJid,
		MessageID:        request.ID,
		FromMe:           request.FromMe,
		ParticipantJID:   participantJid,
	}); err != nil {
		zap.L().Error("Whatsmiau.DeleteMessageForEveryone failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to delete message for everyone")
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{})
}
