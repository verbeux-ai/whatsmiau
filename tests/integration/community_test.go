//go:build integration

package integration_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommunityFlow testa o ciclo de vida completo de uma comunidade:
// criação, subgrupos, link/unlink, configurações de join e participantes.
func TestCommunityFlow(t *testing.T) {
	var communityJID string
	var subGroupJID string

	t.Run("CreateCommunity", func(t *testing.T) {
		resp := do(t, http.MethodPost, communityURL(t, "create"), map[string]any{
			"subject":     "WhatsMiau Community Test",
			"description": "Comunidade de testes automatizados",
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var body map[string]any
		mustDecode(t, resp, &body)
		jid, _ := body["id"].(string)
		require.NotEmpty(t, jid, "resposta deve conter o campo 'id'")
		communityJID = jid

		isCommunity, _ := body["isCommunity"].(bool)
		assert.True(t, isCommunity, "isCommunity deve ser true para comunidades")
	})

	if communityJID == "" {
		t.Skip("CreateCommunity falhou — pulando testes dependentes")
	}

	cooldown() // evita rate-limit ao criar subgrupo logo após a comunidade

	t.Run("CreateSubGroup", func(t *testing.T) {
		resp := do(t, http.MethodPost, communityURL(t, "createSubGroup"), map[string]any{
			"subject":      "Subgrupo de Teste",
			"parentJid":    communityJID,
			"participants": []string{},
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var body map[string]any
		mustDecode(t, resp, &body)
		jid, _ := body["id"].(string)
		require.NotEmpty(t, jid, "resposta deve conter o campo 'id' do subgrupo")
		subGroupJID = jid
	})

	t.Run("SubGroups", func(t *testing.T) {
		resp := do(t, http.MethodGet, communityURLQuery(t, "subGroups", "communityJid="+communityJID), nil)
		defer drainClose(resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var groups []map[string]any
		mustDecode(t, resp, &groups)
		require.NotEmpty(t, groups, "comunidade deve ter ao menos o grupo de anúncios")
	})

	t.Run("LinkedGroupsParticipants", func(t *testing.T) {
		resp := do(t, http.MethodGet,
			communityURLQuery(t, "linkedGroupsParticipants", "communityJid="+communityJID), nil)
		defer drainClose(resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// SetJoinApprovalMode e SetMemberAddMode aplicam-se a subgrupos, não ao parent da comunidade.
	// Usamos subGroupJID quando disponível; caso contrário o sub-teste é pulado.
	t.Run("SetJoinApprovalMode_True", func(t *testing.T) {
		if subGroupJID == "" {
			t.Skip("CreateSubGroup falhou — pulando SetJoinApprovalMode")
		}
		resp := do(t, http.MethodPost, communityURL(t, "setJoinApprovalMode"), map[string]any{
			"communityJid": subGroupJID,
			"mode":         true,
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("SetJoinApprovalMode_False", func(t *testing.T) {
		if subGroupJID == "" {
			t.Skip("CreateSubGroup falhou — pulando SetJoinApprovalMode")
		}
		resp := do(t, http.MethodPost, communityURL(t, "setJoinApprovalMode"), map[string]any{
			"communityJid": subGroupJID,
			"mode":         false,
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("SetMemberAddMode_AdminOnly", func(t *testing.T) {
		if subGroupJID == "" {
			t.Skip("CreateSubGroup falhou — pulando SetMemberAddMode")
		}
		resp := do(t, http.MethodPost, communityURL(t, "setMemberAddMode"), map[string]any{
			"communityJid": subGroupJID,
			"mode":         "admin_add",
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("SetMemberAddMode_AllMembers", func(t *testing.T) {
		if subGroupJID == "" {
			t.Skip("CreateSubGroup falhou — pulando SetMemberAddMode")
		}
		resp := do(t, http.MethodPost, communityURL(t, "setMemberAddMode"), map[string]any{
			"communityJid": subGroupJID,
			"mode":         "all_member_add",
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("RequestParticipants", func(t *testing.T) {
		resp := do(t, http.MethodGet,
			communityURLQuery(t, "requestParticipants", "communityJid="+communityJID), nil)
		defer drainClose(resp)
		// 200 com lista vazia ou com pedidos pendentes
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("UnlinkSubGroup", func(t *testing.T) {
		if subGroupJID == "" {
			t.Skip("CreateSubGroup falhou — pulando UnlinkSubGroup")
		}
		resp := do(t, http.MethodPost, communityURL(t, "unlinkGroup"), map[string]any{
			"parentJid": communityJID,
			"childJid":  subGroupJID,
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("LinkGroup", func(t *testing.T) {
		if subGroupJID == "" {
			t.Skip("CreateSubGroup falhou — pulando LinkGroup")
		}
		resp := do(t, http.MethodPost, communityURL(t, "linkGroup"), map[string]any{
			"parentJid": communityJID,
			"childJid":  subGroupJID,
		})
		defer drainClose(resp)
		// 200 se linkado com sucesso; 403 se instância não é admin (aceitável)
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusForbidden,
			"linkGroup deve retornar 200 ou 403, got %d", resp.StatusCode)
	})
}


// TestCommunityValidation cobre rejeições esperadas nos endpoints de comunidade.
func TestCommunityValidation(t *testing.T) {
	t.Run("CreateCommunity_MissingSubject", func(t *testing.T) {
		resp := do(t, http.MethodPost, communityURL(t, "create"), map[string]any{
			"description": "sem subject",
		})
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("CreateSubGroup_MissingParentJid", func(t *testing.T) {
		resp := do(t, http.MethodPost, communityURL(t, "createSubGroup"), map[string]any{
			"subject":      "Sub sem parent",
			"participants": []string{},
		})
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("CreateSubGroup_InvalidParentJid", func(t *testing.T) {
		resp := do(t, http.MethodPost, communityURL(t, "createSubGroup"), map[string]any{
			"subject":      "Sub com jid inválido",
			"parentJid":    "nao-e-um-jid@s.whatsapp.net",
			"participants": []string{},
		})
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("LinkGroup_MissingChildJid", func(t *testing.T) {
		resp := do(t, http.MethodPost, communityURL(t, "linkGroup"), map[string]any{
			"parentJid": "123456789@g.us",
		})
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("SetMemberAddMode_InvalidMode", func(t *testing.T) {
		resp := do(t, http.MethodPost, communityURL(t, "setMemberAddMode"), map[string]any{
			"communityJid": "123456789@g.us",
			"mode":         "everyone",
		})
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("UpdateRequestParticipants_InvalidAction", func(t *testing.T) {
		resp := do(t, http.MethodPost, communityURL(t, "requestParticipants/update"), map[string]any{
			"communityJid": "123456789@g.us",
			"action":       "ignore",
			"participants": []string{"5511999999999"},
		})
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("SubGroups_MissingCommunityJid", func(t *testing.T) {
		resp := do(t, http.MethodGet, communityURL(t, "subGroups"), nil)
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
