package whatsmiau

import "go.mau.fi/whatsmeow/proto/waE2E"

func (s *Whatsmiau) convertMessageContext(ci *waE2E.ContextInfo) WookMessageContextInfo {
	if ci == nil {
		return WookMessageContextInfo{}
	}
	context := WookMessageContextInfo{
		EphemeralSettingTimestamp:          i64(ci.GetEphemeralSettingTimestamp()),
		StanzaId:                           ci.GetStanzaID(),
		Participant:                        ci.GetParticipant(),
		Expiration:                         int(ci.GetExpiration()),
		MentionedJid:                       ci.GetMentionedJID(),
		ConversionSource:                   ci.GetConversionSource(),
		ConversionData:                     b64(ci.GetConversionData()),
		ConversionDelaySeconds:             ci.ConversionDelaySeconds,
		CtwaPayload:                        b64(ci.GetCtwaPayload()),
		CtwaSignals:                        ci.GetCtwaSignals(),
		EntryPointConversionSource:         ci.GetEntryPointConversionSource(),
		EntryPointConversionApp:            ci.GetEntryPointConversionApp(),
		EntryPointConversionDelaySeconds:   ci.EntryPointConversionDelaySeconds,
		EntryPointConversionExternalSource: ci.GetEntryPointConversionExternalSource(),
		EntryPointConversionExternalMedium: ci.GetEntryPointConversionExternalMedium(),
		TrustBannerAction:                  ci.GetTrustBannerAction(),
		ExternalAdReply:                    convertExternalAdReply(ci.GetExternalAdReply()),
		Utm:                                convertAdUTM(ci.GetUtm()),
		QuotedAd:                           convertQuotedAd(ci.GetQuotedAd()),
		SmbClientCampaignId:                ci.GetSmbClientCampaignID(),
		SmbServerCampaignId:                ci.GetSmbServerCampaignID(),
		AlwaysShowAdAttribution:            ci.AlwaysShowAdAttribution,
	}
	if dm := ci.GetDisappearingMode(); dm != nil {
		context.DisappearingMode = &ContextInfoDisappearingMode{
			Initiator:     dm.GetInitiator().String(),
			Trigger:       dm.GetTrigger().String(),
			InitiatedByMe: dm.GetInitiatedByMe(),
		}
	}
	if quoted := ci.GetQuotedMessage(); quoted != nil {
		_, context.QuotedMessage, _ = s.parseWAMessage(quoted)
	}
	return context
}
