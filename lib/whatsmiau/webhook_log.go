package whatsmiau

import "unicode/utf8"

const maxAdLogFieldBytes = 256

func sanitizeMessageDataForLog(message *WookMessageData) *WookMessageData {
	if message == nil {
		return nil
	}
	result := *message
	result.CtwaClid = boundAdLogField(message.CtwaClid)
	if message.Message != nil {
		raw := *message.Message
		raw.Base64 = ""
		result.Message = &raw
	}
	if message.ContextInfo != nil {
		context := *message.ContextInfo
		context.CtwaPayload = ""
		context.ConversionData = ""
		context.CtwaSignals = ""
		context.ConversionSource = boundAdLogField(context.ConversionSource)
		context.EntryPointConversionSource = boundAdLogField(context.EntryPointConversionSource)
		context.EntryPointConversionApp = boundAdLogField(context.EntryPointConversionApp)
		context.EntryPointConversionExternalSource = boundAdLogField(context.EntryPointConversionExternalSource)
		context.EntryPointConversionExternalMedium = boundAdLogField(context.EntryPointConversionExternalMedium)
		context.SmbClientCampaignId = boundAdLogField(context.SmbClientCampaignId)
		context.SmbServerCampaignId = boundAdLogField(context.SmbServerCampaignId)
		if context.Utm != nil {
			context.Utm = &WookMessageUTMInfo{
				UtmSource:   boundAdLogField(context.Utm.UtmSource),
				UtmCampaign: boundAdLogField(context.Utm.UtmCampaign),
			}
		}
		context.ExternalAdReply = sanitizeAdReplyForLog(context.ExternalAdReply)
		if context.QuotedAd != nil {
			context.QuotedAd = &WookMessageQuotedAd{MediaType: boundAdLogField(context.QuotedAd.MediaType)}
		}
		result.ContextInfo = &context
	}
	result.Referral = sanitizeAdReplyForLog(message.Referral)
	return &result
}

func sanitizeAdReplyForLog(ad *WookMessageContextInfoExternalAdReply) *WookMessageContextInfoExternalAdReply {
	if ad == nil {
		return nil
	}

	return &WookMessageContextInfoExternalAdReply{
		SourceType:            boundAdLogField(ad.SourceType),
		SourceId:              boundAdLogField(ad.SourceId),
		CtwaClid:              boundAdLogField(ad.CtwaClid),
		SourceApp:             boundAdLogField(ad.SourceApp),
		MediaType:             boundAdLogField(ad.MediaType),
		AdType:                boundAdLogField(ad.AdType),
		ContainsAutoReply:     ad.ContainsAutoReply,
		RenderLargerThumbnail: ad.RenderLargerThumbnail,
		ShowAdAttribution:     ad.ShowAdAttribution,
	}
}

func boundAdLogField(value string) string {
	if len(value) <= maxAdLogFieldBytes {
		return value
	}
	end := maxAdLogFieldBytes - len("...")
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end] + "..."
}
