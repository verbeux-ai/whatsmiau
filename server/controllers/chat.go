package controllers

import (
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
)

type Chat struct {
	repo      interfaces.InstanceRepository
	whatsmiau *lib.Whatsmiau
}

func NewChats(repository interfaces.InstanceRepository, whatsmiau *lib.Whatsmiau) *Chat {
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

		if err := s.whatsmiau.ReadMessage(&lib.ReadMessageRequest{
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
			if err := s.whatsmiau.ChatPresence(&lib.ChatPresenceRequest{
				InstanceID: request.InstanceID,
				RemoteJID:  number,
				Presence:   types.ChatPresencePaused,
				Media:      types.ChatPresenceMediaText,
			}); err != nil {
				zap.L().Error("Whatsmiau.ReadMessages failed", zap.Error(err))
			}
		}()
	}

	if err := s.whatsmiau.ChatPresence(&lib.ChatPresenceRequest{
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
