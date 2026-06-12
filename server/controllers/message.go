package controllers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
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

type Message struct {
	repo      interfaces.InstanceRepository
	whatsmiau *whatsmiau.Whatsmiau
}

func NewMessages(repository interfaces.InstanceRepository, whatsmiau *whatsmiau.Whatsmiau) *Message {
	return &Message{
		repo:      repository,
		whatsmiau: whatsmiau,
	}
}

var emojiRegex = regexp.MustCompile(`[\x{1F000}-\x{1FFFF}]|[\x{2300}-\x{23FF}]|[\x{2600}-\x{27BF}]|[\x{2B00}-\x{2BFF}]|[\x{2000}-\x{206F}]|[\x{2100}-\x{214F}]|[\x{2190}-\x{21FF}]`)

// SendText godoc
// @Summary      Send a text message
// @Description  Sends a text message to a WhatsApp number via the specified instance
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string              true  "Instance ID"
// @Param        body      body      dto.SendTextRequest  true  "Text message parameters"
// @Success      200       {object}  dto.SendTextResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/message/text [post]
// @Router       /message/sendText/{instance} [post]
func (s *Message) SendText(ctx echo.Context) error {
	var request dto.SendTextRequest
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

	sendText := &whatsmiau.SendText{
		Text:       request.Text,
		InstanceID: request.InstanceID,
		RemoteJID:  jid,
	}

	if request.Quoted != nil && len(request.Quoted.Key.Id) > 0 && len(request.Quoted.Message.Conversation) > 0 {
		sendText.QuoteMessage = request.Quoted.Message.Conversation
		sendText.QuoteMessageID = request.Quoted.Key.Id
	}

	c := ctx.Request().Context()
	if err := s.whatsmiau.ChatPresence(&whatsmiau.ChatPresenceRequest{
		InstanceID: request.InstanceID,
		RemoteJID:  jid,
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

// SendAudio godoc
// @Summary      Send an audio message
// @Description  Sends an audio file (by URL) as a WhatsApp voice message to the specified number
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string               true  "Instance ID"
// @Param        body      body      dto.SendAudioRequest  true  "Audio message parameters"
// @Success      200       {object}  dto.SendAudioResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/message/audio [post]
// @Router       /message/sendWhatsAppAudio/{instance} [post]
func (s *Message) SendAudio(ctx echo.Context) error {
	var request dto.SendAudioRequest
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

	sendText := &whatsmiau.SendAudioRequest{
		AudioURL:   request.Audio,
		InstanceID: request.InstanceID,
		RemoteJID:  jid,
	}

	if request.Quoted != nil && len(request.Quoted.Key.Id) > 0 && len(request.Quoted.Message.Conversation) > 0 {
		sendText.QuoteMessage = request.Quoted.Message.Conversation
		sendText.QuoteMessageID = request.Quoted.Key.Id
	}

	c := ctx.Request().Context()
	if err := s.whatsmiau.ChatPresence(&whatsmiau.ChatPresenceRequest{
		InstanceID: request.InstanceID,
		RemoteJID:  jid,
		Presence:   types.ChatPresenceComposing,
		Media:      types.ChatPresenceMediaAudio,
	}); err != nil {
		zap.L().Error("Whatsmiau.ChatPresence", zap.Error(err))
	} else {
		time.Sleep(time.Millisecond * time.Duration(request.Delay)) // TODO: create a more robust solution
	}

	res, err := s.whatsmiau.SendAudio(c, sendText)
	if err != nil {
		zap.L().Error("Whatsmiau.SendAudioRequest failed", zap.Error(err))
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

// SendMedia godoc
// @Summary      Send a media message (Evolution API)
// @Description  Sends a media file (image or document) based on the mediatype field
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string               true  "Instance ID"
// @Param        body      body      dto.SendMediaRequest  true  "Media message parameters"
// @Success      200       {object}  dto.SendDocumentResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /message/sendMedia/{instance} [post]
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
	case "video":
		return s.sendVideo(ctx, request.SendDocumentRequest, false)
	}

	return s.sendDocument(ctx, request.SendDocumentRequest)
}

// SendDocument godoc
// @Summary      Send a document
// @Description  Sends a document file by URL to a WhatsApp number
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                  true  "Instance ID"
// @Param        body      body      dto.SendDocumentRequest  true  "Document parameters"
// @Success      200       {object}  dto.SendDocumentResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/message/document [post]
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
	jid, err := numberToJid(request.Number)
	if err != nil {
		zap.L().Error("error converting number to jid", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid number format")
	}

	sendData := &whatsmiau.SendDocumentRequest{
		InstanceID: request.InstanceID,
		MediaURL:   request.Media,
		Caption:    request.Caption,
		FileName:   request.FileName,
		RemoteJID:  jid,
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

// SendImage godoc
// @Summary      Send an image
// @Description  Sends an image file by URL to a WhatsApp number
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                  true  "Instance ID"
// @Param        body      body      dto.SendDocumentRequest  true  "Image parameters"
// @Success      200       {object}  dto.SendDocumentResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/message/image [post]
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
	jid, err := numberToJid(request.Number)
	if err != nil {
		zap.L().Error("error converting number to jid", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid number format")
	}

	sendData := &whatsmiau.SendImageRequest{
		InstanceID: request.InstanceID,
		MediaURL:   request.Media,
		Caption:    request.Caption,
		RemoteJID:  jid,
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

// SendReaction godoc
// @Summary      Send a reaction to a message
// @Description  Sends an emoji reaction to a specific message identified by its key
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                   true  "Instance ID"
// @Param        body      body      dto.SendReactionRequest   true  "Reaction parameters"
// @Success      200       {object}  dto.SendReactionResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /message/sendReaction/{instance} [post]
func (s *Message) SendReaction(ctx echo.Context) error {
	var request dto.SendReactionRequest
	if err := ctx.Bind(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	if err := validator.New().Struct(&request); err != nil {
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid request body")
	}

	jid, err := numberToJid(request.Key.RemoteJid)
	if err != nil {
		zap.L().Error("error converting number to jid", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid number format")
	}

	if request.Reaction != "" && !emojiRegex.MatchString(request.Reaction) {
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "invalid reaction, must be a emoji")
	}

	sendReaction := &whatsmiau.SendReactionRequest{
		InstanceID: request.InstanceID,
		Reaction:   request.Reaction,
		RemoteJID:  jid,
		MessageID:  request.Key.Id,
		FromMe:     *request.Key.FromMe,
	}

	c := ctx.Request().Context()
	res, err := s.whatsmiau.SendReaction(c, sendReaction)
	if err != nil {
		zap.L().Error("Whatsmiau.SendReaction failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send reaction")
	}

	return ctx.JSON(http.StatusOK, dto.SendReactionResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Key.RemoteJid,
			FromMe:    *request.Key.FromMe,
			Id:        res.ID,
		},
		Status:           "sent",
		MessageType:      "reactionMessage",
		MessageTimestamp: int(res.CreatedAt.UnixMicro() / 1000),
		InstanceId:       request.InstanceID,
	})
}

// SendList godoc
// @Summary      Send a list message
// @Description  Sends an interactive list message with selectable options
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string              true  "Instance ID"
// @Param        body      body      dto.SendListRequest  true  "List message parameters"
// @Success      200       {object}  dto.SendListResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/message/list [post]
// @Router       /message/sendList/{instance} [post]
func (s *Message) SendList(ctx echo.Context) error {
	var request dto.SendListRequest
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

	// Convert DTO sections to service-layer sections
	var sections []whatsmiau.SendListSection
	for _, sec := range request.Sections {
		var rows []whatsmiau.SendListRow
		for _, r := range sec.Rows {
			rows = append(rows, whatsmiau.SendListRow{
				Title:       r.Title,
				Description: r.Description,
				RowId:       r.RowId,
			})
		}
		sections = append(sections, whatsmiau.SendListSection{
			Title: sec.Title,
			Rows:  rows,
		})
	}

	sendData := &whatsmiau.SendListRequest{
		InstanceID:  request.InstanceID,
		RemoteJID:   jid,
		Title:       request.Title,
		Description: request.Description,
		ButtonText:  request.ButtonText,
		FooterText:  request.FooterText,
		Sections:    sections,
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

	res, err := s.whatsmiau.SendList(c, sendData)
	if err != nil {
		zap.L().Error("Whatsmiau.SendList failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send list")
	}

	return ctx.JSON(http.StatusOK, dto.SendListResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Number,
			FromMe:    true,
			Id:        res.ID,
		},
		Status:           "sent",
		MessageType:      "listMessage",
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       request.InstanceID,
	})
}

// SendButtons godoc
// @Summary      Send a buttons message
// @Description  Sends an interactive buttons message (reply type) or PIX payment
// @Tags         Message
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        instance  path      string                  true  "Instance ID"
// @Param        body      body      dto.SendButtonsRequest   true  "Buttons message parameters"
// @Success      200       {object}  dto.SendButtonsResponse
// @Failure      400       {object}  utils.HTTPErrorResponse
// @Failure      422       {object}  utils.HTTPErrorResponse
// @Failure      500       {object}  utils.HTTPErrorResponse
// @Router       /instance/{instance}/message/buttons [post]
// @Router       /message/sendButtons/{instance} [post]
func (s *Message) SendButtons(ctx echo.Context) error {
	var request dto.SendButtonsRequest
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

	// Classify button types (already validated by oneof=reply pix)
	var hasReply, hasPix bool
	for _, btn := range request.Buttons {
		switch btn.Type {
		case "reply":
			hasReply = true
		case "pix":
			hasPix = true
		}
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

	// PIX flow: if any button is pix, use PIX handler
	if hasPix {
		return s.sendPixButtons(ctx, c, request, jid)
	}

	// Reply buttons flow
	if hasReply {
		return s.sendReplyButtons(ctx, c, request, jid)
	}

	return utils.HTTPFail(ctx, http.StatusBadRequest, fmt.Errorf("no valid buttons"), "no valid buttons provided")
}

func (s *Message) sendReplyButtons(ctx echo.Context, c context.Context, request dto.SendButtonsRequest, jid *types.JID) error {
	var buttons []whatsmiau.SendButtonItem
	for _, b := range request.Buttons {
		buttons = append(buttons, whatsmiau.SendButtonItem{
			DisplayText: b.DisplayText,
			Id:          b.Id,
		})
	}

	sendData := &whatsmiau.SendButtonsRequestData{
		InstanceID:  request.InstanceID,
		RemoteJID:   jid,
		Title:       request.Title,
		Description: request.Description,
		Footer:      request.Footer,
		Buttons:     buttons,
	}

	res, err := s.whatsmiau.SendButtons(c, sendData)
	if err != nil {
		zap.L().Error("Whatsmiau.SendButtons failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send buttons")
	}

	return ctx.JSON(http.StatusOK, dto.SendButtonsResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Number,
			FromMe:    true,
			Id:        res.ID,
		},
		Status:           "sent",
		MessageType:      "buttonsMessage",
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       request.InstanceID,
	})
}

func (s *Message) sendPixButtons(ctx echo.Context, c context.Context, request dto.SendButtonsRequest, jid *types.JID) error {
	// Find the pix button
	var pixBtn dto.SendButtonsRequestButton
	for _, b := range request.Buttons {
		if b.Type == "pix" {
			pixBtn = b
			break
		}
	}

	sendData := &whatsmiau.SendPixPaymentRequest{
		InstanceID:   request.InstanceID,
		RemoteJID:    jid,
		PixKey:       pixBtn.Key,
		PixKeyType:   pixBtn.KeyType,
		MerchantName: pixBtn.Name,
		DisplayText:  pixBtn.DisplayText,
		Currency:     pixBtn.Currency,
	}

	res, err := s.whatsmiau.SendPixPayment(c, sendData)
	if err != nil {
		zap.L().Error("Whatsmiau.SendPixPayment failed", zap.Error(err))
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send pix payment")
	}

	return ctx.JSON(http.StatusOK, dto.SendButtonsResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: request.Number,
			FromMe:    true,
			Id:        res.ID,
		},
		Status:           "sent",
		MessageType:      "buttonsMessage",
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       request.InstanceID,
	})
}
