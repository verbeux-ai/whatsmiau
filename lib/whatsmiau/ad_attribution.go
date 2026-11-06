package whatsmiau

import "go.mau.fi/whatsmeow/proto/waE2E"

func convertExternalAdReply(ad *waE2E.ContextInfo_ExternalAdReplyInfo) *WookMessageContextInfoExternalAdReply {
	if ad == nil {
		return nil
	}
	result := &WookMessageContextInfoExternalAdReply{
		Title:                             ad.GetTitle(),
		Body:                              ad.GetBody(),
		MediaType:                         ad.GetMediaType().String(),
		ThumbnailUrl:                      ad.GetThumbnailURL(),
		MediaUrl:                          ad.GetMediaURL(),
		Thumbnail:                         b64(ad.GetThumbnail()),
		SourceType:                        ad.GetSourceType(),
		SourceId:                          ad.GetSourceID(),
		SourceUrl:                         ad.GetSourceURL(),
		ContainsAutoReply:                 ad.ContainsAutoReply,
		RenderLargerThumbnail:             ad.RenderLargerThumbnail,
		ShowAdAttribution:                 ad.ShowAdAttribution,
		CtwaClid:                          ad.GetCtwaClid(),
		Ref:                               ad.GetRef(),
		ClickToWhatsappCall:               ad.ClickToWhatsappCall,
		AdContextPreviewDismissed:         ad.AdContextPreviewDismissed,
		SourceApp:                         ad.GetSourceApp(),
		AutomatedGreetingMessageShown:     ad.AutomatedGreetingMessageShown,
		GreetingMessageBody:               ad.GetGreetingMessageBody(),
		CtaPayload:                        ad.GetCtaPayload(),
		DisableNudge:                      ad.DisableNudge,
		OriginalImageUrl:                  ad.GetOriginalImageURL(),
		AutomatedGreetingMessageCtaType:   ad.GetAutomatedGreetingMessageCtaType(),
		WtwaAdFormat:                      ad.WtwaAdFormat,
		WtwaWebsiteUrl:                    ad.GetWtwaWebsiteURL(),
		AdPreviewUrl:                      ad.GetAdPreviewURL(),
		ContainsCtwaFlowsAutoReply:        ad.ContainsCtwaFlowsAutoReply,
		AgmThumbnailStrategy:              ad.AgmThumbnailStrategy,
		AgmTitleStrategy:                  ad.AgmTitleStrategy,
		AgmSubtitleStrategy:               ad.AgmSubtitleStrategy,
		AgmHeaderInteractionStrategy:      ad.AgmHeaderInteractionStrategy,
		ContainsCtwaFlowsAutoLabel:        ad.ContainsCtwaFlowsAutoLabel,
		ProductId:                         ad.GetProductID(),
		ContainsCtwaPromo:                 ad.ContainsCtwaPromo,
		CtwaPromoResponseId:               ad.GetCtwaPromoResponseID(),
		CtwaPromoOfferId:                  ad.GetCtwaPromoOfferID(),
		CtwaPromoAdEntryId:                ad.GetCtwaPromoAdEntryID(),
		CtwaPromoResponseExpiresAtSeconds: ad.CtwaPromoResponseExpiresAtSeconds,
		CtwaPromoSchemaVersion:            ad.CtwaPromoSchemaVersion,
		CtwaPromoAdgroupId:                ad.GetCtwaPromoAdgroupID(),
	}
	if ad.AdType != nil {
		result.AdType = ad.GetAdType().String()
	}
	return result
}

func convertAdUTM(utm *waE2E.ContextInfo_UTMInfo) *WookMessageUTMInfo {
	if utm == nil {
		return nil
	}
	return &WookMessageUTMInfo{UtmSource: utm.GetUtmSource(), UtmCampaign: utm.GetUtmCampaign()}
}

func convertQuotedAd(ad *waE2E.ContextInfo_AdReplyInfo) *WookMessageQuotedAd {
	if ad == nil {
		return nil
	}
	result := &WookMessageQuotedAd{AdvertiserName: ad.GetAdvertiserName(), JpegThumbnail: b64(ad.GetJPEGThumbnail()), Caption: ad.GetCaption()}
	if ad.MediaType != nil {
		result.MediaType = ad.GetMediaType().String()
	}
	return result
}

func compactAdReferral(ad *WookMessageContextInfoExternalAdReply) *WookMessageContextInfoExternalAdReply {
	if ad == nil || ad.Thumbnail == "" {
		return ad
	}
	referral := *ad
	referral.Thumbnail = ""
	return &referral
}
