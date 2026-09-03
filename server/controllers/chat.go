package controllers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// fetchPictureSingleFlight is package-level because both Chat and ChatEVO
// instantiate their own Chat controller; the deduplication must be shared.
var fetchPictureSingleFlight singleflight.Group

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
// @Router       /v1/instance/{instance}/chat/read-messages [post]
// @Router       /v1/chat/markMessageAsRead/{instance} [post]
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
// @Router       /v1/instance/{instance}/chat/presence [post]
// @Router       /v1/chat/sendPresence/{instance} [post]
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
// @Router       /v1/chat/whatsappNumbers/{instance} [post]
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
// @Description  Revokes a message in the chat so it is deleted for all participants.
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
// @Router       /v1/instance/{instance}/chat/deleteMessageForEveryone [delete]
// @Router       /v1/chat/deleteMessageForEveryone/{instance} [delete]
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

// UpdateMessage godoc
// @Summary      Edit a message
// @Description  Edits a previously sent text message, mirroring Evolution API's POST /chat/updateMessage
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                    true  "Instance ID"
// @Param        body      body      dto.UpdateMessageRequest   true  "Message edit parameters"
// @Success      200       {object}  dto.UpdateMessageResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /v1/chat/updateMessage/{instance} [post]
func (s *Chat) UpdateMessage(ctx echo.Context) error {
	var request dto.UpdateMessageRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	jid, err := numberToJid(request.Number)
	if err != nil {
		zap.L().Error("error converting number to jid", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid number format")
	}

	c := ctx.Request().Context()
	res, err := s.whatsmiau.UpdateMessage(c, &whatsmiau.UpdateMessage{
		Text:       request.Text,
		InstanceID: request.InstanceID,
		RemoteJID:  jid,
		Key: &whatsmiau.EditMessageKey{
			ID:          request.Key.Id,
			RemoteJID:   request.Key.RemoteJid,
			FromMe:      request.Key.FromMe != nil && *request.Key.FromMe,
			Participant: request.Key.Participant,
		},
	})
	if err != nil {
		zap.L().Error("Whatsmiau.UpdateMessage failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to edit message")
	}

	return ctx.JSON(http.StatusOK, dto.UpdateMessageResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Number,
			FromMe:    true,
			Id:        res.ID,
		},
		Status:           "sent",
		Message:          dto.SendTextResponseMessage{Conversation: request.Text},
		MessageType:      "conversation",
		MessageTimestamp: int(res.CreatedAt.Unix()),
		InstanceId:       request.InstanceID,
	})
}

// FetchProfilePicture godoc
// @Summary      Fetch profile picture URL
// @Description  Returns the full-size profile picture URL for a number or group JID, mirroring Evolution API's POST /chat/fetchProfilePictureUrl. profilePictureUrl is null when the target has no picture or hid it.
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                            true  "Instance ID"
// @Param        body      body      dto.FetchProfilePictureRequest     true  "Number or JID"
// @Success      200       {object}  dto.FetchProfilePictureResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /v1/chat/fetchProfilePictureUrl/{instance} [post]
func (s *Chat) FetchProfilePicture(ctx echo.Context) error {
	var request dto.FetchProfilePictureRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	number := request.Number
	if !strings.Contains(number, "@") && len(number) >= 18 {
		// A bare id that long cannot be a phone number (E.164 maxes at 15
		// digits); it is a group id, mirroring Evolution's createJid.
		number += "@" + types.GroupServer
	}

	jid, err := numberToJid(number)
	if err != nil {
		zap.L().Error("error converting number to jid", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid number format")
	}

	// Reject JIDs whose server is not a known WhatsApp one before any
	// network round-trip happens (ParseJID accepts arbitrary servers).
	switch jid.Server {
	case types.DefaultUserServer, types.GroupServer, types.HiddenUserServer, types.BroadcastServer:
	default:
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "invalid jid server")
	}

	// Singleflight with a detached context: the shared whatsmeow call must not
	// be canceled because one waiting request was aborted. In-process only —
	// no cache, mirroring Evolution's per-request fetch.
	sfCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	url, err, _ := fetchPictureSingleFlight.Do(request.InstanceID+":"+jid.String(), func() (interface{}, error) {
		return s.whatsmiau.FetchProfilePictureURL(sfCtx, request.InstanceID, *jid)
	})
	if err != nil {
		zap.L().Error("Whatsmiau.FetchProfilePictureURL failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to fetch profile picture")
	}

	return ctx.JSON(http.StatusOK, buildProfilePictureResponse(jid, url.(string)))
}

func buildProfilePictureResponse(jid *types.JID, url string) dto.FetchProfilePictureResponse {
	resp := dto.FetchProfilePictureResponse{Wuid: jid.String()}
	if url != "" {
		resp.ProfilePictureUrl = &url
	}
	return resp
}

// SyncChatMessages godoc
// @Summary      Sync chat messages on demand
// @Description  Requests message history for a chat from the user's primary device (on-demand history sync). The primary device (phone) must have a server-connected WhatsApp session — history is served from its local message store. The `id` anchor is required: history is returned backwards from it, `count` messages per page (default 50). Optionally paginates back to a target date with `since`. A single request returns at most 500 messages. Recommended not to be used for bulk history extraction.
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                        true  "Instance ID"
// @Param        body      body      dto.SyncChatMessagesRequest   true  "Sync parameters"
// @Success      200       {array}   whatsmiau.WookMessageData     "Messages synced from the chat"
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      409       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Failure      504       {object}  utils.HTTPErrorResponse
// @Router       /v1/instance/{instance}/chat/syncMessages [post]
// @Router       /v1/chat/syncMessages/{instance} [post]
func (s *Chat) SyncChatMessages(ctx echo.Context) error {
	var request dto.SyncChatMessagesRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	chat, err := numberToJid(request.Number)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid number")
	}

	var since *time.Time
	if request.Since != "" {
		parsed, parseErr := parseSyncSince(request.Since)
		if parseErr != nil {
			return utils.HTTPFail(ctx, http.StatusBadRequest, parseErr, "invalid since (use YYYY-MM-DD or RFC3339)")
		}
		since = &parsed
	}

	messages, err := s.whatsmiau.SyncChatMessages(ctx.Request().Context(), &whatsmiau.SyncChatMessagesRequest{
		InstanceID: request.InstanceID,
		Chat:       *chat,
		Count:      request.Count,
		Since:      since,
		ID:         request.ID,
		FromMe:     request.FromMe,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.SyncChatMessages failed", zap.Error(err))
		switch {
		case errors.Is(err, whatsmiau.ErrSyncTimeout):
			return utils.HTTPFail(ctx, http.StatusGatewayTimeout, err, "phone did not respond in time")
		case errors.Is(err, whatsmeow.ErrClientIsNil):
			return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "instance not found")
		case strings.Contains(err.Error(), "client not connected"):
			return utils.HTTPFail(ctx, http.StatusConflict, err, "client not connected")
		default:
			return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to sync chat messages")
		}
	}

	if messages == nil {
		messages = []whatsmiau.WookMessageData{}
	}
	return ctx.JSON(http.StatusOK, messages)
}

func parseSyncSince(input string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", input); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, input)
}
