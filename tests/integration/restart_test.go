//go:build integration

package integration_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestartInstance(t *testing.T) {
	id := instanceID(t)

	t.Run("RestartConnectedInstance", func(t *testing.T) {
		statusResp := do(t, http.MethodGet, "/v1/instance/"+id+"/status", nil)
		defer drainClose(statusResp)
		require.Equal(t, http.StatusOK, statusResp.StatusCode)

		var statusBody map[string]any
		mustDecode(t, statusResp, &statusBody)
		state, _ := statusBody["state"].(string)
		if state != "open" {
			t.Skipf("instance not connected (state=%s), skipping restart test", state)
		}

		resp := do(t, http.MethodPost, "/v1/instance/"+id+"/restart", nil)
		defer drainClose(resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]any
		mustDecode(t, resp, &body)
		assert.Equal(t, id, body["id"])
		assert.Equal(t, "connecting", body["state"])

		inst, ok := body["instance"].(map[string]any)
		require.True(t, ok, "response must contain 'instance' field")
		assert.Equal(t, id, inst["instanceName"])
		assert.Equal(t, "connecting", inst["status"])
	})

	t.Run("RestartNonExistentInstance", func(t *testing.T) {
		resp := do(t, http.MethodPost, "/v1/instance/non-existent-xyz/restart", nil)
		defer drainClose(resp)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("RestartEvoRoute", func(t *testing.T) {
		resp := do(t, http.MethodPost, "/v1/instance/restart/"+id, nil)
		defer drainClose(resp)
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest,
			"expected 200 or 400, got %d", resp.StatusCode)
	})
}
