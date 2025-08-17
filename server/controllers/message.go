package controllers

import (
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
)

type Message struct {
	repo      interfaces.InstanceRepository
	whatsmiau *lib.Whatsmiau
}

func NewMessages(repository interfaces.InstanceRepository, whatsmiau *lib.Whatsmiau) *Message {
	return &Message{
		repo:      repository,
		whatsmiau: whatsmiau,
	}
}

func (s *Message) SendText(ctx echo.Context) error {
	var request dto.SendTextRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	splitNumber := strings.Split(request.Number, "@")
	if len(splitNumber) != 2 {
		request.Number += "@s.whatsapp.net"
	}

	if len(splitNumber[0]) < 12 {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "invalid jid, put country prefix")
	}

	if len(splitNumber[0]) == 13 {
		first4 := request.Number[:4]
		last8 := request.Number[5:]
		request.Number = first4 + last8
	}

	jid, err := types.ParseJID(request.Number)
	if err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid jid (number)")
	}

	c := ctx.Request().Context()
	res, err := s.whatsmiau.SendText(c, &lib.SendText{
		Text:       request.Text,
		InstanceID: request.InstanceID,
		RemoteJID:  jid,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.SendText failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send text")
	}

	return ctx.JSON(http.StatusOK, dto.SendTextResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Number,
			FromMe:    true,
			Id:        res.ID,
		},
		Status: "sent",
		Message: dto.SendTextResponseMessage{
			Conversation: request.Text,
		},
		MessageType:      "conversation",
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       request.InstanceID,
	})
}
