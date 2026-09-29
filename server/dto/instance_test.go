package dto

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConnectInstanceResponseAlwaysReportsConnected(t *testing.T) {
	body, err := json.Marshal(ConnectInstanceResponse{Message: "waiting for QR code generation"})
	if err != nil {
		t.Fatalf("marshal connect response: %v", err)
	}

	if !strings.Contains(string(body), `"connected":false`) {
		t.Fatalf("connected flag missing when false: %s", body)
	}
}
