package whatsmiau

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func adWebhookForSizeTest(t *testing.T) *WookEvent[WookMessageData] {
	t.Helper()
	var context WookMessageContextInfo
	if err := json.Unmarshal([]byte(adAttributionContextJSON), &context); err != nil {
		t.Fatal(err)
	}
	return &WookEvent[WookMessageData]{
		Instance: "ads-test", Event: WookMessagesUpsert,
		Data: &WookMessageData{
			Key:     &WookKey{Id: "ADS-SIZE", RemoteJid: "5511000000000@s.whatsapp.net"},
			Message: &WookMessageRaw{Conversation: "hello"}, MessageType: "conversation",
			ContextInfo: &context, Referral: compactAdReferral(context.ExternalAdReply),
			CtwaClid: context.ExternalAdReply.CtwaClid,
		},
	}
}

func mustMarshalAdWebhook(t *testing.T, body any) []byte {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestWebhookPayloadKeepsExistingThumbnailsWithinLimit(t *testing.T) {
	event := adWebhookForSizeTest(t)
	event.Data.ContextInfo.ExternalAdReply.Thumbnail = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{255}, 512*1024))
	before := mustMarshalAdWebhook(t, event)
	encoded, omitted, err := marshalWebhookPayload(event)
	if err != nil || omitted || !bytes.Equal(encoded, before) {
		t.Fatalf("normal payload changed: omitted=%v error=%v", omitted, err)
	}
	if len(encoded) >= webhookPayloadLimit {
		t.Fatal("fixture must fit within the manager limit")
	}
}

func TestWebhookPayloadOmitsOnlyAdThumbnailBytesWhenOversized(t *testing.T) {
	largeThumbnail := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{255}, 3*1024*1024))
	for _, kind := range []string{"external ad", "quoted ad", "referral", "both ads"} {
		t.Run(kind, func(t *testing.T) {
			event := adWebhookForSizeTest(t)
			switch kind {
			case "external ad":
				event.Data.ContextInfo.ExternalAdReply.Thumbnail = largeThumbnail
			case "quoted ad":
				event.Data.ContextInfo.QuotedAd.JpegThumbnail = largeThumbnail
			case "referral":
				event.Data.Referral.Thumbnail = largeThumbnail
			case "both ads":
				event.Data.ContextInfo.ExternalAdReply.Thumbnail = largeThumbnail
				event.Data.ContextInfo.QuotedAd.JpegThumbnail = largeThumbnail
			}
			var input any = event
			if kind == "quoted ad" {
				input = *event // Value envelopes follow the same delivery contract.
			}
			before := mustMarshalAdWebhook(t, input)
			if len(before) <= webhookPayloadLimit {
				t.Fatal("fixture did not exceed the manager limit")
			}
			encoded, omitted, err := marshalWebhookPayload(input)
			if err != nil || !omitted || len(encoded) > webhookPayloadLimit {
				t.Fatalf("oversized payload was not compacted: bytes=%d omitted=%v error=%v", len(encoded), omitted, err)
			}
			var expected, actual map[string]interface{}
			if err := json.Unmarshal(before, &expected); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &actual); err != nil {
				t.Fatal(err)
			}
			data := expected["data"].(map[string]interface{})
			context := data["contextInfo"].(map[string]interface{})
			delete(context["externalAdReply"].(map[string]interface{}), "thumbnail")
			delete(context["quotedAd"].(map[string]interface{}), "jpegThumbnail")
			delete(data["referral"].(map[string]interface{}), "thumbnail")
			if !reflect.DeepEqual(actual, expected) {
				t.Fatal("compaction changed message content, identifiers, URLs or other metadata")
			}
			if !bytes.Equal(before, mustMarshalAdWebhook(t, input)) {
				t.Fatal("compaction mutated the original event")
			}
			t.Logf("payload reduced from %d to %d bytes; tracking preserved", len(before), len(encoded))
		})
	}
}

func TestWebhookPayloadDoesNotRemoveOtherDataToMeetLimit(t *testing.T) {
	largeText := strings.Repeat("x", webhookPayloadLimit)
	event := adWebhookForSizeTest(t)
	event.Data.Message.Conversation = largeText
	event.Data.ContextInfo.ExternalAdReply.Thumbnail = ""
	event.Data.ContextInfo.QuotedAd.JpegThumbnail = ""
	before := mustMarshalAdWebhook(t, event)
	encoded, omitted, err := marshalWebhookPayload(event)
	if err != nil || omitted || !bytes.Equal(encoded, before) {
		t.Fatal("non-thumbnail content was removed or changed")
	}

	event.Data.ContextInfo.ExternalAdReply.Thumbnail = "AQID"
	encoded, omitted, err = marshalWebhookPayload(event)
	if err != nil || !omitted || len(encoded) <= webhookPayloadLimit {
		t.Fatal("a large message must not be silently truncated")
	}
	var actual WookEvent[WookMessageData]
	if err := json.Unmarshal(encoded, &actual); err != nil {
		t.Fatal(err)
	}
	if actual.Data.Message.Conversation != largeText || actual.Data.CtwaClid != "CLICK-001" {
		t.Fatal("message content or click ID was lost")
	}
}

func TestWebhookPayloadLeavesOtherEventTypesAndEncodingErrorsUntouched(t *testing.T) {
	event := adWebhookForSizeTest(t)
	event.Data.ContextInfo.ExternalAdReply.Thumbnail = strings.Repeat("x", webhookPayloadLimit)
	event.Event = WookMessagesSet
	for _, input := range []any{event, *event, map[string]interface{}{"text": strings.Repeat("x", webhookPayloadLimit)}} {
		encoded, omitted, err := marshalWebhookPayload(input)
		if err != nil || omitted || !bytes.Equal(encoded, mustMarshalAdWebhook(t, input)) {
			t.Fatal("unrelated event was changed")
		}
	}
	for _, input := range []any{nil, (*WookEvent[WookMessageData])(nil), &WookEvent[WookMessageData]{Instance: strings.Repeat("x", webhookPayloadLimit), Event: WookMessagesUpsert}} {
		if _, omitted, err := marshalWebhookPayload(input); err != nil || omitted {
			t.Fatalf("nil/empty payload changed: omitted=%v error=%v", omitted, err)
		}
	}
	if _, omitted, err := marshalWebhookPayload(make(chan int)); err == nil || omitted {
		t.Fatal("serialization error was hidden")
	}
}

type adWebhookSizeTransport func(*http.Request) (*http.Response, error)

func (f adWebhookSizeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestProcessEmitDeliversTrackingAfterOmittingOversizedAdThumbnails(t *testing.T) {
	event := adWebhookForSizeTest(t)
	event.Data.ContextInfo.ExternalAdReply.Thumbnail = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{255}, 3*1024*1024))
	requests := 0
	service := &Whatsmiau{webhookClient: &http.Client{Transport: adWebhookSizeTransport(func(request *http.Request) (*http.Response, error) {
		requests++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if len(body) > webhookPayloadLimit {
			t.Fatal("payload would be rejected by the manager with HTTP 413")
		}
		if request.Method != http.MethodPost || request.URL.String() != "https://manager.example.test/internal/webhook" || request.Header.Get("Content-Type") != "application/json" || request.Header.Get("Authorization") != "Bearer internal-test-key" {
			t.Fatal("delivery destination or authentication changed")
		}
		var received WookEvent[WookMessageData]
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatal(err)
		}
		if received.Instance != "ads-test" || received.Event != WookMessagesUpsert || received.Data.Key.Id != "ADS-SIZE" || received.Data.CtwaClid != "CLICK-001" || received.Data.ContextInfo.ExternalAdReply.SourceId != "9007199254740993" || received.Data.ContextInfo.EntryPointConversionSource != "ctwa_ad" || received.Data.Referral.CtwaClid != "CLICK-001" {
			t.Fatal("ADS tracking did not reach the manager")
		}
		if received.Data.ContextInfo.ExternalAdReply.Thumbnail != "" || received.Data.ContextInfo.ExternalAdReply.MediaType != "IMAGE" || received.Data.ContextInfo.CtwaPayload != "b3BhcXVl" {
			t.Fatal("thumbnail omission changed existing API types or opaque metadata")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}}
	service.processEmit(emitter{
		url: "https://manager.example.test/internal/webhook", headers: map[string]string{"Authorization": "Bearer internal-test-key"}, data: event,
	})
	if requests != 1 {
		t.Fatalf("requests=%d, expected one successful delivery", requests)
	}
	if event.Data.ContextInfo.ExternalAdReply.Thumbnail == "" {
		t.Fatal("delivery mutated the original message")
	}
}
