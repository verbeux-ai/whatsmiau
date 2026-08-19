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
	var announcementJID string
	var subGroupJID string

	t.Run("CreateCommunity", func(t *testing.T) {
		const initialDescription = "Comunidade de testes automatizados"
		resp := do(t, http.MethodPost, communityURL(t, "create"), map[string]any{
			"subject":     "WhatsMiau Community Test",
			"description": initialDescription,
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
		assert.Equal(t, initialDescription, body["desc"], "descrição deve ser definida durante a criação")
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

		var groups []struct {
			ID                string `json:"id"`
			IsDefaultSubGroup bool   `json:"isDefaultSubGroup"`
		}
		mustDecode(t, resp, &groups)
		require.NotEmpty(t, groups, "comunidade deve ter ao menos o grupo de anúncios")
		for _, group := range groups {
			if group.IsDefaultSubGroup {
				announcementJID = group.ID
				break
			}
		}
		require.NotEmpty(t, announcementJID, "comunidade deve informar o JID do grupo de anúncios")
	})

	readGroupDescription := func(t *testing.T, groupJID string) string {
		t.Helper()
		resp := do(t, http.MethodGet, groupURLQuery(t, "findGroupInfos", "groupJid="+groupJID), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var info struct {
			Description string `json:"desc"`
		}
		mustDecode(t, resp, &info)
		return info.Description
	}

	readCommunityGroupAddMode := func(t *testing.T, communityJID string) string {
		t.Helper()
		resp := do(t, http.MethodGet, groupURLQuery(t, "findGroupInfos", "groupJid="+communityJID), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var info struct {
			GroupAddMode string `json:"groupAddMode"`
		}
		mustDecode(t, resp, &info)
		return info.GroupAddMode
	}

	t.Run("UpdateDescriptionViaCommunityParent", func(t *testing.T) {
		const description = "Descrição atualizada diretamente no parent"
		resp := do(t, http.MethodPost, groupURL(t, "updateGroupDescription"), map[string]any{
			"groupJid":    communityJID,
			"description": description,
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, description, readGroupDescription(t, communityJID))
	})

	cooldown()

	t.Run("UpdateDescriptionViaAnnouncementResolvesParent", func(t *testing.T) {
		if announcementJID == "" {
			t.Skip("SubGroups não retornou o grupo de anúncios")
		}
		const description = "Descrição atualizada usando o JID de avisos"
		announcementDescription := readGroupDescription(t, announcementJID)

		resp := do(t, http.MethodPost, groupURL(t, "updateGroupDescription"), map[string]any{
			"groupJid":    announcementJID,
			"description": description,
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, description, readGroupDescription(t, communityJID))
		assert.Equal(t, announcementDescription, readGroupDescription(t, announcementJID),
			"descrição própria do grupo de avisos não deve ser alterada")
	})

	cooldown()

	t.Run("UpdateDescriptionAgainViaAnnouncement", func(t *testing.T) {
		if announcementJID == "" {
			t.Skip("SubGroups não retornou o grupo de anúncios")
		}
		const description = "Segunda descrição atualizada usando o JID de avisos"
		resp := do(t, http.MethodPost, groupURL(t, "updateGroupDescription"), map[string]any{
			"groupJid":    announcementJID,
			"description": description,
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, description, readGroupDescription(t, communityJID))
	})

	t.Run("LinkedGroupsParticipants", func(t *testing.T) {
		resp := do(t, http.MethodGet,
			communityURLQuery(t, "linkedGroupsParticipants", "communityJid="+communityJID), nil)
		defer drainClose(resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// SetJoinApprovalMode aplica-se diretamente a subgrupos. SetGroupAddMode
	// aplica-se ao parent da comunidade e controla se membros que não são admins
	// podem adicionar ou vincular grupos à comunidade.
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

	t.Run("SetGroupAddMode_AdminOnly", func(t *testing.T) {
		resp := do(t, http.MethodPost, communityURL(t, "setGroupAddMode"), map[string]any{
			"communityJid": communityJID,
			"mode":         "admin_add",
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, "admin_add", readCommunityGroupAddMode(t, communityJID))
	})

	t.Run("SetGroupAddMode_AllMembers", func(t *testing.T) {
		resp := do(t, http.MethodPost, communityURL(t, "setGroupAddMode"), map[string]any{
			"communityJid": communityJID,
			"mode":         "all_member_add",
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, "all_member_add", readCommunityGroupAddMode(t, communityJID))
	})

	t.Run("SetGroupAddMode_RestoreAdminOnly", func(t *testing.T) {
		restore := do(t, http.MethodPost, communityURL(t, "setGroupAddMode"), map[string]any{
			"communityJid": communityJID,
			"mode":         "admin_add",
		})
		defer drainClose(restore)
		require.Equal(t, http.StatusCreated, restore.StatusCode)
		assert.Equal(t, "admin_add", readCommunityGroupAddMode(t, communityJID))
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

	t.Run("SetGroupAddMode_InvalidMode", func(t *testing.T) {
		resp := do(t, http.MethodPost, communityURL(t, "setGroupAddMode"), map[string]any{
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
