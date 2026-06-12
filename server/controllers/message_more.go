package controllers

import (
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
)

// SendVideo godoc
// @Summary      Send a video
// @Description  Sends a video file by URL to a WhatsApp number
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                  true  "Instance ID"
// @Param        body      body      dto.SendDocumentRequest  true  "Video parameters"
// @Success      200       {object}  dto.SendDocumentResponse
// @Router       /instance/{instance}/message/video [post]
func (s *Message) SendVideo(ctx echo.Context) error {
	var request dto.SendDocumentRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	return s.sendVideo(ctx, request, false)
}

func (s *Message) sendVideo(ctx echo.Context, request dto.SendDocumentRequest, gif bool) error {
	jid, err := numberToJid(request.Number)
	if err != nil {
		zap.L().Error("error converting number to jid", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid number format")
	}

	sendData := &whatsmiau.SendVideoRequest{
		InstanceID:  request.InstanceID,
		MediaURL:    request.Media,
		Caption:     request.Caption,
		RemoteJID:   jid,
		Mimetype:    request.Mimetype,
		GifPlayback: gif,
	}

	c := ctx.Request().Context()
	time.Sleep(time.Millisecond * time.Duration(request.Delay))

	res, err := s.whatsmiau.SendVideo(c, sendData)
	if err != nil {
		zap.L().Error("Whatsmiau.SendVideo failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send video")
	}

	return ctx.JSON(http.StatusOK, dto.SendDocumentResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Number,
			FromMe:    true,
			Id:        res.ID,
		},
		Status:           "sent",
		MessageType:      "videoMessage",
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       request.InstanceID,
	})
}

// SendPtv godoc
// @Summary      Send a round video note (PTV)
// @Description  Sends a round/circle video note to a WhatsApp number
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string             true  "Instance ID"
// @Param        body      body      dto.SendPtvRequest  true  "PTV parameters"
// @Success      200       {object}  dto.SendPtvResponse
// @Router       /message/sendPtv/{instance} [post]
func (s *Message) SendPtv(ctx echo.Context) error {
	var request dto.SendPtvRequest
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
	if err := s.whatsmiau.ChatPresence(&whatsmiau.ChatPresenceRequest{
		InstanceID: request.InstanceID,
		RemoteJID:  jid,
		Presence:   types.ChatPresenceComposing,
	}); err != nil {
		zap.L().Error("Whatsmiau.ChatPresence", zap.Error(err))
	} else {
		time.Sleep(time.Millisecond * time.Duration(request.Delay))
	}

	res, err := s.whatsmiau.SendPtv(c, &whatsmiau.SendPtvRequest{
		InstanceID: request.InstanceID,
		VideoURL:   request.Video,
		RemoteJID:  jid,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.SendPtv failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send ptv")
	}

	return ctx.JSON(http.StatusOK, dto.SendPtvResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Number,
			FromMe:    true,
			Id:        res.ID,
		},
		Status:           "sent",
		MessageType:      "ptvMessage",
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       request.InstanceID,
	})
}

// SendSticker godoc
// @Summary      Send a sticker
// @Description  Sends a WebP sticker to a WhatsApp number
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                 true  "Instance ID"
// @Param        body      body      dto.SendStickerRequest  true  "Sticker parameters"
// @Success      200       {object}  dto.SendStickerResponse
// @Router       /message/sendSticker/{instance} [post]
func (s *Message) SendSticker(ctx echo.Context) error {
	var request dto.SendStickerRequest
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
	time.Sleep(time.Millisecond * time.Duration(request.Delay))

	res, err := s.whatsmiau.SendSticker(c, &whatsmiau.SendStickerRequest{
		InstanceID: request.InstanceID,
		StickerURL: request.Sticker,
		RemoteJID:  jid,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.SendSticker failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send sticker")
	}

	return ctx.JSON(http.StatusOK, dto.SendStickerResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Number,
			FromMe:    true,
			Id:        res.ID,
		},
		Status:           "sent",
		MessageType:      "stickerMessage",
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       request.InstanceID,
	})
}

// SendLocation godoc
// @Summary      Send a location message
// @Description  Sends a geographic location to a WhatsApp number
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                  true  "Instance ID"
// @Param        body      body      dto.SendLocationRequest  true  "Location parameters"
// @Success      200       {object}  dto.SendLocationResponse
// @Router       /message/sendLocation/{instance} [post]
func (s *Message) SendLocation(ctx echo.Context) error {
	var request dto.SendLocationRequest
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
	time.Sleep(time.Millisecond * time.Duration(request.Delay))

	res, err := s.whatsmiau.SendLocation(c, &whatsmiau.SendLocationRequest{
		InstanceID: request.InstanceID,
		RemoteJID:  jid,
		Latitude:   request.Latitude,
		Longitude:  request.Longitude,
		Name:       request.Name,
		Address:    request.Address,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.SendLocation failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send location")
	}

	return ctx.JSON(http.StatusOK, dto.SendLocationResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Number,
			FromMe:    true,
			Id:        res.ID,
		},
		Status:           "sent",
		MessageType:      "locationMessage",
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       request.InstanceID,
	})
}

// SendContact godoc
// @Summary      Send one or more contacts
// @Description  Sends contact cards (vCard) to a WhatsApp number
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                 true  "Instance ID"
// @Param        body      body      dto.SendContactRequest  true  "Contact parameters"
// @Success      200       {object}  dto.SendContactResponse
// @Router       /message/sendContact/{instance} [post]
func (s *Message) SendContact(ctx echo.Context) error {
	var request dto.SendContactRequest
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

	contacts := make([]whatsmiau.SendContactItem, 0, len(request.Contact))
	for _, c := range request.Contact {
		contacts = append(contacts, whatsmiau.SendContactItem{
			FullName:     c.FullName,
			Wuid:         c.Wuid,
			PhoneNumber:  c.PhoneNumber,
			Organization: c.Organization,
			Email:        c.Email,
			URL:          c.URL,
		})
	}

	c := ctx.Request().Context()
	time.Sleep(time.Millisecond * time.Duration(request.Delay))

	res, err := s.whatsmiau.SendContact(c, &whatsmiau.SendContactRequest{
		InstanceID: request.InstanceID,
		RemoteJID:  jid,
		Contacts:   contacts,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.SendContact failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send contact")
	}

	messageType := "contactMessage"
	if len(contacts) > 1 {
		messageType = "contactsArrayMessage"
	}

	return ctx.JSON(http.StatusOK, dto.SendContactResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Number,
			FromMe:    true,
			Id:        res.ID,
		},
		Status:           "sent",
		MessageType:      messageType,
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       request.InstanceID,
	})
}

// SendPoll godoc
// @Summary      Send a poll
// @Description  Sends a multiple-choice poll to a WhatsApp number
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string              true  "Instance ID"
// @Param        body      body      dto.SendPollRequest  true  "Poll parameters"
// @Success      200       {object}  dto.SendPollResponse
// @Router       /message/sendPoll/{instance} [post]
func (s *Message) SendPoll(ctx echo.Context) error {
	var request dto.SendPollRequest
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
	time.Sleep(time.Millisecond * time.Duration(request.Delay))

	res, err := s.whatsmiau.SendPoll(c, &whatsmiau.SendPollRequest{
		InstanceID:      request.InstanceID,
		RemoteJID:       jid,
		Name:            request.Name,
		SelectableCount: request.SelectableCount,
		Values:          request.Values,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.SendPoll failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send poll")
	}

	return ctx.JSON(http.StatusOK, dto.SendPollResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Number,
			FromMe:    true,
			Id:        res.ID,
		},
		Status:           "sent",
		MessageType:      "pollCreationMessage",
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       request.InstanceID,
	})
}

// SendStatus godoc
// @Summary      Send a status broadcast
// @Description  Sends a status (text/image/audio/video) to status@broadcast
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                true  "Instance ID"
// @Param        body      body      dto.SendStatusRequest  true  "Status parameters"
// @Success      200       {object}  dto.SendStatusResponse
// @Router       /message/sendStatus/{instance} [post]
func (s *Message) SendStatus(ctx echo.Context) error {
	var request dto.SendStatusRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	c := ctx.Request().Context()
	res, err := s.whatsmiau.SendStatus(c, &whatsmiau.SendStatusRequest{
		InstanceID:      request.InstanceID,
		Type:            request.Type,
		Content:         request.Content,
		Caption:         request.Caption,
		BackgroundColor: request.BackgroundColor,
		Font:            request.Font,
		StatusJidList:   request.StatusJidList,
		AllContacts:     request.AllContacts,
	})
	if err != nil {
		zap.L().Error("Whatsmiau.SendStatus failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send status")
	}

	return ctx.JSON(http.StatusOK, dto.SendStatusResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: "status@broadcast",
			FromMe:    true,
			Id:        res.ID,
		},
		Status:           "sent",
		MessageType:      request.Type + "Message",
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       request.InstanceID,
	})
}
