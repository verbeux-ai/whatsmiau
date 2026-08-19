//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type instanceListItem struct {
	ID        string `json:"id"`
	SaveMedia *bool  `json:"saveMedia"`
}

func updateInstanceURL(t *testing.T) string {
	return fmt.Sprintf("/v1/instance/update/%s", instanceID(t))
}

// fetchSaveMedia lê o valor persistido de saveMedia da instância de teste.
func fetchSaveMedia(t *testing.T) *bool {
	t.Helper()

	resp := do(t, http.MethodGet, fmt.Sprintf("/v1/instance?id=%s", instanceID(t)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var list []instanceListItem
	mustDecode(t, resp, &list)
	require.Len(t, list, 1, "instância de teste não encontrada")

	return list[0].SaveMedia
}

func setSaveMedia(t *testing.T, value bool) {
	t.Helper()

	resp := do(t, http.MethodPut, updateInstanceURL(t), map[string]any{"saveMedia": value})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	drainClose(resp)
}

// TestSaveMediaSurvivesUnrelatedUpdate cobre o modo de falha do ponteiro: se
// UpdateInstanceRequest.SaveMedia fosse declarado bool em vez de *bool, qualquer update
// que omitisse o campo gravaria false e desligaria a persistência sem ninguém pedir.
func TestSaveMediaSurvivesUnrelatedUpdate(t *testing.T) {
	original := fetchSaveMedia(t)
	t.Cleanup(func() {
		// Se o campo estava ausente (nil), não há como restaurar a ausência pela API —
		// omitir significa "não altere". Restaura para true, que é o comportamento
		// equivalente a nil.
		restore := true
		if original != nil {
			restore = *original
		}
		setSaveMedia(t, restore)
	})

	// O valor sob teste tem que ser true: se fosse false, um DTO declarado bool (o bug)
	// também gravaria false no update seguinte e o teste passaria sem detectar nada.
	setSaveMedia(t, true)
	got := fetchSaveMedia(t)
	require.NotNil(t, got, "saveMedia deveria estar persistido após update explícito")
	require.True(t, *got)

	// Update de um campo não relacionado, sem mencionar saveMedia. Usa groupsIgnore de
	// propósito, e não um campo de webhook, para não acionar de raspão o reset de
	// webhook.base64 e transformar isso num falso negativo.
	resp := do(t, http.MethodPut, updateInstanceURL(t), map[string]any{"groupsIgnore": false})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	drainClose(resp)

	got = fetchSaveMedia(t)
	require.NotNil(t, got, "update não relacionado apagou saveMedia")
	require.True(t, *got, "update não relacionado sobrescreveu saveMedia: o DTO precisa ser *bool")

	// Um false explícito precisa sobreviver à serialização para o Redis. Com bool em vez
	// de *bool no model, omitempty descartaria o campo e ele voltaria como ausente.
	setSaveMedia(t, false)
	got = fetchSaveMedia(t)
	require.NotNil(t, got, "false explícito desapareceu na serialização")
	require.False(t, *got)
}
