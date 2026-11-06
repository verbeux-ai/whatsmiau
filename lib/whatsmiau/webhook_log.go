package whatsmiau

func sanitizeMessageDataForLog(message *WookMessageData) *WookMessageData {
	if message == nil {
		return nil
	}
	result := *message
	if message.Message != nil {
		raw := *message.Message
		raw.Base64 = ""
		result.Message = &raw
	}
	if message.ContextInfo != nil {
		context := *message.ContextInfo
		context.CtwaPayload = ""
		context.ConversionData = ""
		context.ExternalAdReply = sanitizeAdReplyForLog(context.ExternalAdReply)
		result.ContextInfo = &context
	}
	result.Referral = sanitizeAdReplyForLog(message.Referral)
	return &result
}

func sanitizeAdReplyForLog(ad *WookMessageContextInfoExternalAdReply) *WookMessageContextInfoExternalAdReply {
	if ad == nil {
		return nil
	}
	result := *ad
	result.GreetingMessageBody = ""
	result.CtaPayload = ""
	return &result
}
