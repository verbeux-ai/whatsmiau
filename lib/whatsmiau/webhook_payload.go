package whatsmiau

import "encoding/json"

const webhookPayloadLimit = 4 * 1024 * 1024

func marshalWebhookPayload(body any) ([]byte, bool, error) {
	encoded, err := json.Marshal(body)
	if err != nil || len(encoded) <= webhookPayloadLimit {
		return encoded, false, err
	}

	var compacted any
	switch event := body.(type) {
	case *WookEvent[WookMessageData]:
		if event == nil || event.Event != WookMessagesUpsert {
			return encoded, false, nil
		}
		message, changed := withoutAdThumbnailBytes(event.Data)
		if !changed {
			return encoded, false, nil
		}
		copy := *event
		copy.Data = message
		compacted = &copy
	case WookEvent[WookMessageData]:
		if event.Event != WookMessagesUpsert {
			return encoded, false, nil
		}
		message, changed := withoutAdThumbnailBytes(event.Data)
		if !changed {
			return encoded, false, nil
		}
		event.Data = message
		compacted = event
	default:
		return encoded, false, nil
	}

	encoded, err = json.Marshal(compacted)
	return encoded, true, err
}

// Remove only optional ad image bytes from an outgoing copy. Keep the original
// event, message content, attribution identifiers, URLs and all other metadata.
func withoutAdThumbnailBytes(message *WookMessageData) (*WookMessageData, bool) {
	if message == nil {
		return nil, false
	}
	result := *message
	changed := false
	if message.ContextInfo != nil {
		context := *message.ContextInfo
		if ad := context.ExternalAdReply; ad != nil && ad.Thumbnail != "" {
			copy := *ad
			copy.Thumbnail = ""
			context.ExternalAdReply = &copy
			changed = true
		}
		if ad := context.QuotedAd; ad != nil && ad.JpegThumbnail != "" {
			copy := *ad
			copy.JpegThumbnail = ""
			context.QuotedAd = &copy
			changed = true
		}
		if changed {
			result.ContextInfo = &context
		}
	}
	if referral := result.Referral; referral != nil && referral.Thumbnail != "" {
		copy := *referral
		copy.Thumbnail = ""
		result.Referral = &copy
		changed = true
	}
	if !changed {
		return message, false
	}
	return &result, true
}
