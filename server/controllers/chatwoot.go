package controllers

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib/whatsmiau"
	"github.com/verbeux-ai/whatsmiau/server/dto"
	"github.com/verbeux-ai/whatsmiau/utils"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
)

type Chatwoot struct {
	repo      interfaces.InstanceRepository
	whatsmiau *whatsmiau.Whatsmiau
}

func NewChatwoot(repository interfaces.InstanceRepository, whatsmiau *whatsmiau.Whatsmiau) *Chatwoot {
	return &Chatwoot{
		repo:      repository,
		whatsmiau: whatsmiau,
	}
}

// ChatwootWebhookBody representa o payload recebido do Chatwoot
type ChatwootWebhookBody struct {
	Event        string `json:"event"`
	MessageType  string `json:"message_type"`
	Content      string `json:"content"`
	ContentType  string `json:"content_type"`
	Private      bool   `json:"private"`
	ID           int    `json:"id"`
	Conversation struct {
		ID       int `json:"id"`
		Meta struct {
			Sender struct {
				PhoneNumber string `json:"phone_number"`
			} `json:"sender"`
		} `json:"meta"`
		ContactInbox struct {
			SourceID string `json:"source_id"`
		} `json:"contact_inbox"`
		Messages []struct {
			ID       int    `json:"id"`
			SourceID string `json:"source_id"`
		} `json:"messages"`
	} `json:"conversation"`
	Inbox struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"inbox"`
	ContentAttributes struct {
		Deleted bool `json:"deleted"`
	} `json:"content_attributes"`
	Sender struct {
		Name string `json:"name"`
	} `json:"sender"`
}

func (s *Chatwoot) ReceiveWebhook(ctx echo.Context) error {
	instanceID := ctx.Param("instance")
	
	// Log da request recebida
	zap.L().Info("=== CHATWOOT WEBHOOK RECEIVED ===",
		zap.String("instance_param", instanceID),
		zap.String("method", ctx.Request().Method),
		zap.String("url", ctx.Request().URL.String()),
		zap.String("remote_addr", ctx.Request().RemoteAddr),
	)
	
	if instanceID == "" {
		zap.L().Error("instance parameter is empty")
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "instance parameter is required")
	}

	var webhook ChatwootWebhookBody
	if err := ctx.Bind(&webhook); err != nil {
		zap.L().Error("failed to bind chatwoot webhook", 
			zap.Error(err),
			zap.String("error_detail", err.Error()),
		)
		return utils.HTTPFail(ctx, http.StatusUnprocessableEntity, err, "failed to bind request body")
	}

	// Log do evento recebido com todos os detalhes
	zap.L().Info("chatwoot webhook parsed",
		zap.String("instance", instanceID),
		zap.String("event", webhook.Event),
		zap.String("message_type", webhook.MessageType),
		zap.String("content_type", webhook.ContentType),
		zap.String("content", webhook.Content),
		zap.Bool("private", webhook.Private),
		zap.String("phone_number", webhook.Conversation.Meta.Sender.PhoneNumber),
		zap.String("source_id", webhook.Conversation.ContactInbox.SourceID),
	)

	// Ignora se não tiver conversação ou se for mensagem privada
	if webhook.Conversation.ID == 0 || webhook.Private {
		zap.L().Info("event ignored - no conversation or private message")
		return ctx.JSON(http.StatusOK, map[string]string{
			"message": "bot",
		})
	}

	// Ignora message_updated se não for delete
	if webhook.Event == "message_updated" && !webhook.ContentAttributes.Deleted {
		zap.L().Info("event ignored - message_updated without delete")
		return ctx.JSON(http.StatusOK, map[string]string{
			"message": "bot",
		})
	}

	// Processa apenas mensagens outgoing (do agente para o cliente)
	if webhook.Event != "message_created" || webhook.MessageType != "outgoing" {
		zap.L().Info("event ignored - not an outgoing message creation",
			zap.String("event", webhook.Event),
			zap.String("message_type", webhook.MessageType),
		)
		return ctx.JSON(http.StatusOK, map[string]string{
			"message": "bot",
		})
	}

	// Verifica se já existe source_id (mensagem veio do WhatsApp, não processar)
	if len(webhook.Conversation.Messages) > 0 && 
	   webhook.Conversation.Messages[0].SourceID != "" &&
	   strings.HasPrefix(webhook.Conversation.Messages[0].SourceID, "WAID:") {
		zap.L().Info("event ignored - message originated from whatsapp")
		return ctx.JSON(http.StatusOK, map[string]string{
			"message": "bot",
		})
	}

	zap.L().Info("processing message", 
		zap.String("content_type", webhook.ContentType),
		zap.String("content", webhook.Content),
	)

	// Processar baseado no tipo de conteúdo
	switch webhook.ContentType {
	case "text":
		return s.handleTextMessage(ctx, instanceID, webhook)
	default:
		zap.L().Warn("unsupported content type",
			zap.String("content_type", webhook.ContentType),
		)
		return ctx.JSON(http.StatusOK, map[string]string{
			"status": "ignored",
			"reason": "unsupported content type",
		})
	}
}

func (s *Chatwoot) handleTextMessage(ctx echo.Context, instanceID string, webhook ChatwootWebhookBody) error {
	zap.L().Info("=== HANDLING TEXT MESSAGE ===",
		zap.String("instance", instanceID),
		zap.String("content", webhook.Content),
	)
	
	// Extrair número do telefone do identificador do contato
	chatId := webhook.Conversation.Meta.Sender.PhoneNumber
	if chatId == "" {
		chatId = webhook.Conversation.ContactInbox.SourceID
	}
	
	zap.L().Info("extracted chat identifier", 
		zap.String("chat_id_raw", chatId),
	)
	
	if chatId == "" {
		zap.L().Error("chat identifier is empty in webhook")
		return utils.HTTPFail(ctx, http.StatusBadRequest, nil, "chat identifier not found in webhook")
	}

	// Limpar o número: remover + e caracteres especiais
	// Pode vir como: +5548986133374 ou 5548986133374@s.whatsapp.net
	phoneNumber := strings.Split(chatId, "@")[0]
	phoneNumber = strings.ReplaceAll(phoneNumber, "+", "")
	phoneNumber = strings.ReplaceAll(phoneNumber, " ", "")
	phoneNumber = strings.ReplaceAll(phoneNumber, "-", "")
	phoneNumber = strings.ReplaceAll(phoneNumber, "(", "")
	phoneNumber = strings.ReplaceAll(phoneNumber, ")", "")
	
	zap.L().Info("cleaned phone number", 
		zap.String("phone_original", chatId),
		zap.String("phone_clean", phoneNumber),
	)

	// Converter número para JID
	jid, err := numberToJid(phoneNumber)
	if err != nil {
		zap.L().Error("error converting number to jid", 
			zap.Error(err),
			zap.String("phone_original", chatId),
			zap.String("phone_clean", phoneNumber),
		)
		return utils.HTTPFail(ctx, http.StatusBadRequest, err, "invalid number format")
	}
	zap.L().Info("converted to JID successfully", zap.String("jid", jid.String()))

	// Formatar texto da mensagem (Chatwoot usa Markdown, WhatsApp usa formato próprio)
	// Converte: *texto* -> _texto_ (itálico), **texto** -> *texto* (negrito)
	messageText := webhook.Content
	messageText = strings.ReplaceAll(messageText, "**", "*")  // Negrito
	
	// Se tiver nome do sender, adicionar assinatura
	senderName := webhook.Sender.Name
	if senderName != "" {
		messageText = fmt.Sprintf("*%s:*\n%s", senderName, messageText)
	}
	
	zap.L().Info("formatted message text",
		zap.String("original", webhook.Content),
		zap.String("formatted", messageText),
	)

	// Enviar indicador de "digitando"
	zap.L().Info("sending typing indicator...")
	if err := s.whatsmiau.ChatPresence(&whatsmiau.ChatPresenceRequest{
		InstanceID: instanceID,
		RemoteJID:  jid,
		Presence:   types.ChatPresenceComposing,
	}); err != nil {
		zap.L().Error("Whatsmiau.ChatPresence failed", zap.Error(err))
	} else {
		zap.L().Info("typing indicator sent")
		// Delay simulando digitação
		time.Sleep(time.Millisecond * 500)
	}

	// Preparar e enviar mensagem
	sendText := &whatsmiau.SendText{
		Text:       messageText,
		InstanceID: instanceID,
		RemoteJID:  jid,
	}

	zap.L().Info("sending text message...",
		zap.String("text", messageText),
		zap.String("to_jid", jid.String()),
	)

	c := ctx.Request().Context()
	res, err := s.whatsmiau.SendText(c, sendText)
	if err != nil {
		zap.L().Error("Whatsmiau.SendText failed", 
			zap.Error(err),
			zap.String("error_detail", err.Error()),
		)
		return utils.HTTPFail(ctx, http.StatusInternalServerError, err, "failed to send text")
	}

	zap.L().Info("text message sent successfully!", 
		zap.String("message_id", res.ID),
	)

	// Enviar indicador de "parou de digitar"
	zap.L().Info("sending pause indicator...")
	if err := s.whatsmiau.ChatPresence(&whatsmiau.ChatPresenceRequest{
		InstanceID: instanceID,
		RemoteJID:  jid,
		Presence:   types.ChatPresencePaused,
	}); err != nil {
		zap.L().Error("Whatsmiau.ChatPresence pause failed", zap.Error(err))
	} else {
		zap.L().Info("pause indicator sent")
	}

	response := dto.SendTextResponse{
		Key: dto.MessageResponseKey{
			RemoteJid: chatId,
			FromMe:    true,
			Id:        res.ID,
		},
		Status: "sent",
		Message: dto.SendTextResponseMessage{
			Conversation: messageText,
		},
		MessageType:      "conversation",
		MessageTimestamp: int(res.CreatedAt.Unix() / 1000),
		InstanceId:       instanceID,
	}

	zap.L().Info("=== MESSAGE PROCESSED SUCCESSFULLY ===")

	return ctx.JSON(http.StatusOK, response)
}
