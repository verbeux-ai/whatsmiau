//go:build integration

package integration_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListContacts(t *testing.T) {
	id := instanceID(t)

	t.Run("RequiresPagination", func(t *testing.T) {
		resp := do(t, http.MethodGet, "/v1/instance/"+id+"/contacts", nil)
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("RejectsLimitAboveMaximum", func(t *testing.T) {
		resp := do(t, http.MethodGet, "/v1/instance/"+id+"/contacts?page=1&limit=101", nil)
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("ListsContactsForConnectedInstance", func(t *testing.T) {
		statusResp := do(t, http.MethodGet, "/v1/instance/"+id+"/status", nil)
		defer drainClose(statusResp)
		require.Equal(t, http.StatusOK, statusResp.StatusCode)

		var statusBody map[string]any
		mustDecode(t, statusResp, &statusBody)
		state, _ := statusBody["state"].(string)
		if state != "open" {
			t.Skipf("instance not connected (state=%s), skipping contacts list test", state)
		}

		resp := do(t, http.MethodGet, "/v1/instance/"+id+"/contacts?page=1&limit=10", nil)
		defer drainClose(resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]any
		mustDecode(t, resp, &body)
		data, ok := body["data"].([]any)
		require.True(t, ok, "response must contain data array")
		pagination, ok := body["pagination"].(map[string]any)
		require.True(t, ok, "response must contain pagination object")
		assert.Equal(t, float64(1), pagination["page"])
		assert.Equal(t, float64(10), pagination["limit"])
		_, totalOk := pagination["total"]
		_, totalPagesOk := pagination["totalPages"]
		assert.True(t, totalOk, "pagination must contain total")
		assert.True(t, totalPagesOk, "pagination must contain totalPages")
		assert.NotNil(t, data)
		if len(data) > 0 {
			first, ok := data[0].(map[string]any)
			require.True(t, ok, "contact item must be an object")
			assert.NotEmpty(t, first["id"])
			assert.NotEmpty(t, first["remoteJid"])
			assert.Equal(t, false, first["isGroup"])
			assert.Equal(t, true, first["isSaved"])
			assert.Equal(t, "contact", first["type"])
			_, hasProfilePicURL := first["profilePicUrl"]
			assert.True(t, hasProfilePicURL, "contact item must contain profilePicUrl")
		}
	})
}
