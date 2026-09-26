package dto

import (
	"encoding/json"
	"testing"
)

func TestSendTextRequestDecodesCustomLinkPreview(t *testing.T) {
	var request SendTextRequest
	err := json.Unmarshal([]byte(`{
		"number": "5511999999999",
		"text": "Deal https://example.com/p/1",
		"linkPreview": true,
		"linkPreviewImage": "https://cdn.example.com/p/1.jpg",
		"linkPreviewTitle": "Product for $10",
		"linkPreviewDescription": "Deal of the day",
		"linkPreviewLarge": false
	}`), &request)
	if err != nil {
		t.Fatalf("unmarshal send text request: %v", err)
	}

	if request.LinkPreviewImage != "https://cdn.example.com/p/1.jpg" ||
		request.LinkPreviewTitle != "Product for $10" ||
		request.LinkPreviewDescription != "Deal of the day" {
		t.Fatalf("custom link preview fields not decoded: %+v", request)
	}
	// An explicit false must survive decoding: nil means "default (large)".
	if request.LinkPreviewLarge == nil || *request.LinkPreviewLarge {
		t.Fatalf("linkPreviewLarge=false not decoded: %v", request.LinkPreviewLarge)
	}
}

func TestSendTextRequestLinkPreviewLargeDefaultsToNil(t *testing.T) {
	var request SendTextRequest
	if err := json.Unmarshal([]byte(`{"number": "1", "text": "x", "linkPreview": true}`), &request); err != nil {
		t.Fatalf("unmarshal send text request: %v", err)
	}
	if request.LinkPreviewLarge != nil {
		t.Fatalf("expected nil linkPreviewLarge when omitted, got %v", *request.LinkPreviewLarge)
	}
}
