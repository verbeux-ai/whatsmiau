package controllers

import (
	"net/http"
	"strings"
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

	sendText := &lib.SendText{
		Text:       request.Text,
		InstanceID: request.InstanceID,
		RemoteJID:  &jid,
	}

	if request.Quoted != nil && len(request.Quoted.Key.Id) > 0 && len(request.Quoted.Message.Conversation) > 0 {
		sendText.QuoteMessage = request.Quoted.Message.Conversation
		sendText.QuoteMessageID = request.Quoted.Key.Id
	}

	c := ctx.Request().Context()
	if err := s.whatsmiau.ChatPresence(&lib.ChatPresenceRequest{
		InstanceID: request.InstanceID,
		RemoteJID:  &jid,
		Presence:   types.ChatPresenceComposing,
	}); err != nil {
		zap.L().Error("Whatsmiau.ChatPresence", zap.Error(err))
	} else {
		time.Sleep(time.Millisecond * time.Duration(request.Delay)) // TODO: create a more robust solution
	}

	res, err := s.whatsmiau.SendText(c, sendText)
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

func (s *Message) SendAudio(ctx echo.Context) error {
	var request dto.SendAudioRequest
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

	sendText := &lib.SendAudio{
		AudioURL:   request.Audio,
		InstanceID: request.InstanceID,
		RemoteJID:  &jid,
	}

	if request.Quoted != nil && len(request.Quoted.Key.Id) > 0 && len(request.Quoted.Message.Conversation) > 0 {
		sendText.QuoteMessage = request.Quoted.Message.Conversation
		sendText.QuoteMessageID = request.Quoted.Key.Id
	}

	c := ctx.Request().Context()
	if err := s.whatsmiau.ChatPresence(&lib.ChatPresenceRequest{
		InstanceID: request.InstanceID,
		RemoteJID:  &jid,
		Presence:   types.ChatPresenceComposing,
		Media:      types.ChatPresenceMediaAudio,
	}); err != nil {
		zap.L().Error("Whatsmiau.ChatPresence", zap.Error(err))
	} else {
		time.Sleep(time.Millisecond * time.Duration(request.Delay)) // TODO: create a more robust solution
	}

	res, err := s.whatsmiau.SendAudio(c, sendText)
	if err != nil {
		zap.L().Error("Whatsmiau.SendAudio failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send audio")
	}

	return ctx.JSON(http.StatusOK, dto.SendAudioResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Number,
			FromMe:    true,
			Id:        res.ID,
		},

		Status:           "sent",
		MessageType:      "audioMessage",
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       request.InstanceID,
	})
}

// For evolution compatibility
func (s *Message) SendMedia(ctx echo.Context) error {
	var request dto.SendMediaRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}
	switch request.Mediatype {
	case "image":
		request.SendDocumentRequest.Mimetype = "image/png"
		return s.sendImage(ctx, request.SendDocumentRequest)
	}

	return s.sendDocument(ctx, request.SendDocumentRequest)
}

func (s *Message) SendDocument(ctx echo.Context) error {
	var request dto.SendDocumentRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	return s.sendDocument(ctx, request)
}

func (s *Message) sendDocument(ctx echo.Context, request dto.SendDocumentRequest) error {
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

	sendData := &lib.SendDocumentRequest{
		InstanceID: request.InstanceID,
		MediaURL:   request.Media,
		Caption:    request.Caption,
		FileName:   request.FileName,
		RemoteJID:  &jid,
		Mimetype:   request.Mimetype,
	}

	c := ctx.Request().Context()
	time.Sleep(time.Millisecond * time.Duration(request.Delay)) // TODO: create a more robust solution

	res, err := s.whatsmiau.SendDocument(c, sendData)
	if err != nil {
		zap.L().Error("Whatsmiau.SendDocument failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send document")
	}

	return ctx.JSON(http.StatusOK, dto.SendDocumentResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Number,
			FromMe:    true,
			Id:        res.ID,
		},
		Status:           "sent",
		MessageType:      "documentMessage",
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       request.InstanceID,
	})
}

func (s *Message) SendImage(ctx echo.Context) error {
	var request dto.SendDocumentRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	return s.sendImage(ctx, request)
}

func (s *Message) sendImage(ctx echo.Context, request dto.SendDocumentRequest) error {
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

	sendData := &lib.SendImageRequest{
		InstanceID: request.InstanceID,
		MediaURL:   request.Media,
		Caption:    request.Caption,
		RemoteJID:  &jid,
		Mimetype:   request.Mimetype,
	}

	c := ctx.Request().Context()
	time.Sleep(time.Millisecond * time.Duration(request.Delay)) // TODO: create a more robust solution

	res, err := s.whatsmiau.SendImage(c, sendData)
	if err != nil {
		zap.L().Error("Whatsmiau.SendDocument failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send document")
	}

	return ctx.JSON(http.StatusOK, dto.SendDocumentResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Number,
			FromMe:    true,
			Id:        res.ID,
		},
		Status:           "sent",
		MessageType:      "imageMessage",
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       request.InstanceID,
	})
}
