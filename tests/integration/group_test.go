//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGroupFlow testa o ciclo de vida completo de um grupo: criação, consulta,
// atualização de metadados, convite, configurações e saída.
// Cada sub-teste depende do estado gerado pelo anterior; se um falhar,
// os seguintes são pulados via t.SkipNow() para evitar falsos negativos.
func TestGroupFlow(t *testing.T) {
	var groupJID string

	t.Run("CreateGroup", func(t *testing.T) {
		resp := do(t, http.MethodPost, groupURL(t, "create"), map[string]any{
			"subject":      "WhatsMiau Test",
			"participants": []string{},
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var body map[string]any
		mustDecode(t, resp, &body)
		jid, _ := body["id"].(string)
		require.NotEmpty(t, jid, "resposta deve conter o campo 'id' com o JID do grupo")
		groupJID = jid
		assert.Equal(t, true, body["isCommunity"] == false || body["isCommunity"] == nil)
	})

	if groupJID == "" {
		t.Skip("CreateGroup falhou — pulando testes dependentes")
	}

	t.Run("FindGroupInfos", func(t *testing.T) {
		resp := do(t, http.MethodGet, groupURLQuery(t, "findGroupInfos", "groupJid="+groupJID), nil)
		defer drainClose(resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]any
		mustDecode(t, resp, &body)
		assert.Equal(t, groupJID, body["id"])
		assert.Equal(t, "WhatsMiau Test", body["subject"])
		assert.NotNil(t, body["participants"])
	})

	t.Run("FetchAllGroups", func(t *testing.T) {
		resp := do(t, http.MethodGet, groupURLQuery(t, "fetchAllGroups", "getParticipants=false"), nil)
		defer drainClose(resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var groups []map[string]any
		mustDecode(t, resp, &groups)
		require.NotEmpty(t, groups)

		found := false
		for _, g := range groups {
			if g["id"] == groupJID {
				found = true
				break
			}
		}
		assert.True(t, found, "grupo criado deve aparecer em fetchAllGroups")
	})

	t.Run("FindParticipants", func(t *testing.T) {
		resp := do(t, http.MethodGet, groupURLQuery(t, "participants", "groupJid="+groupJID), nil)
		defer drainClose(resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]any
		mustDecode(t, resp, &body)
		_, ok := body["participants"]
		assert.True(t, ok, "resposta deve conter o campo 'participants'")
	})

	t.Run("UpdateGroupSubject", func(t *testing.T) {
		resp := do(t, http.MethodPost, groupURL(t, "updateGroupSubject"), map[string]any{
			"groupJid": groupJID,
			"subject":  "WhatsMiau Test Atualizado",
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		// confirma que o nome foi alterado
		check := do(t, http.MethodGet, groupURLQuery(t, "findGroupInfos", "groupJid="+groupJID), nil)
		var info map[string]any
		mustDecode(t, check, &info)
		assert.Equal(t, "WhatsMiau Test Atualizado", info["subject"])
	})

	t.Run("UpdateGroupDescription", func(t *testing.T) {
		resp := do(t, http.MethodPost, groupURL(t, "updateGroupDescription"), map[string]any{
			"groupJid":    groupJID,
			"description": "Grupo de testes automatizados do WhatsMiau",
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("InviteCode", func(t *testing.T) {
		resp := do(t, http.MethodGet, groupURLQuery(t, "inviteCode", "groupJid="+groupJID), nil)
		defer drainClose(resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]any
		mustDecode(t, resp, &body)
		assert.NotEmpty(t, body["inviteCode"])
		assert.NotEmpty(t, body["inviteUrl"])
	})

	t.Run("InviteInfo", func(t *testing.T) {
		// Busca o invite code atual
		codeResp := do(t, http.MethodGet, groupURLQuery(t, "inviteCode", "groupJid="+groupJID), nil)
		var codeBody map[string]any
		mustDecode(t, codeResp, &codeBody)
		code, _ := codeBody["inviteCode"].(string)
		require.NotEmpty(t, code)

		resp := do(t, http.MethodGet, groupURLQuery(t, "inviteInfo", "inviteCode="+code), nil)
		defer drainClose(resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]any
		mustDecode(t, resp, &body)
		assert.Equal(t, groupJID, body["id"])
	})

	t.Run("RevokeInviteCode", func(t *testing.T) {
		resp := do(t, http.MethodPost, groupURL(t, "revokeInviteCode"), map[string]any{
			"groupJid": groupJID,
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var body map[string]any
		mustDecode(t, resp, &body)
		assert.NotEmpty(t, body["inviteCode"], "deve retornar o novo código de convite")
		assert.NotEmpty(t, body["inviteUrl"])
	})

	t.Run("UpdateSettingAnnouncement", func(t *testing.T) {
		resp := do(t, http.MethodPost, groupURL(t, "updateSetting"), map[string]any{
			"groupJid": groupJID,
			"action":   "announcement",
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		// Reverte para modo normal
		revert := do(t, http.MethodPost, groupURL(t, "updateSetting"), map[string]any{
			"groupJid": groupJID,
			"action":   "not_announcement",
		})
		drainClose(revert)
	})

	t.Run("UpdateSettingLocked", func(t *testing.T) {
		resp := do(t, http.MethodPost, groupURL(t, "updateSetting"), map[string]any{
			"groupJid": groupJID,
			"action":   "locked",
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		revert := do(t, http.MethodPost, groupURL(t, "updateSetting"), map[string]any{
			"groupJid": groupJID,
			"action":   "unlocked",
		})
		drainClose(revert)
	})

	t.Run("ToggleEphemeral", func(t *testing.T) {
		// Ativa mensagens temporárias de 24h
		resp := do(t, http.MethodPost, groupURL(t, "toggleEphemeral"), map[string]any{
			"groupJid":   groupJID,
			"expiration": 86400,
		})
		defer drainClose(resp)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		// Desativa
		revert := do(t, http.MethodPost, groupURL(t, "toggleEphemeral"), map[string]any{
			"groupJid":   groupJID,
			"expiration": 0,
		})
		drainClose(revert)
	})

	t.Run("LeaveGroup", func(t *testing.T) {
		resp := do(t, http.MethodDelete, groupURLQuery(t, "leaveGroup", "groupJid="+groupJID), nil)
		defer drainClose(resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Verificar que findGroupInfos retorna 4xx (não 500) após sair
		check := do(t, http.MethodGet, groupURLQuery(t, "findGroupInfos", "groupJid="+groupJID), nil)
		drainClose(check)
		assert.True(t, check.StatusCode >= 400 && check.StatusCode < 500,
			"findGroupInfos após LeaveGroup deve retornar 4xx, got %d", check.StatusCode)
	})
}

// TestGroupUpdateParticipant testa add/remove/promote/demote de participantes.
// Requer TEST_PARTICIPANT_NUMBER definida.
func TestGroupUpdateParticipant(t *testing.T) {
	number := participantNumber()
	if number == "" {
		t.Skip("TEST_PARTICIPANT_NUMBER não definida — pulando testes de participantes")
	}

	// Cria grupo com o participante
	createResp := do(t, http.MethodPost, groupURL(t, "create"), map[string]any{
		"subject":      "WhatsMiau Partic Test",
		"participants": []string{number},
	})
	var created map[string]any
	mustDecode(t, createResp, &created)
	groupJID, _ := created["id"].(string)
	require.NotEmpty(t, groupJID)

	t.Cleanup(func() {
		resp := do(t, http.MethodDelete, groupURLQuery(t, "leaveGroup", "groupJid="+groupJID), nil)
		drainClose(resp)
	})

	for _, tc := range []struct {
		name   string
		action string
	}{
		{"Promote", "promote"},
		{"Demote", "demote"},
		{"Remove", "remove"},
		{"Add", "add"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp := do(t, http.MethodPost, groupURL(t, "updateParticipant"), map[string]any{
				"groupJid":     groupJID,
				"action":       tc.action,
				"participants": []string{number},
			})
			defer drainClose(resp)
			require.Equal(t, http.StatusCreated, resp.StatusCode)

			var body map[string]any
			mustDecode(t, resp, &body)
			assert.NotNil(t, body["participants"])
		})
	}
}

// TestGroupSendInvite testa o envio de convite para um número externo.
// Requer TEST_PARTICIPANT_NUMBER definida.
func TestGroupSendInvite(t *testing.T) {
	number := participantNumber()
	if number == "" {
		t.Skip("TEST_PARTICIPANT_NUMBER não definida — pulando testes de envio de convite")
	}

	// Cria grupo temporário
	createResp := do(t, http.MethodPost, groupURL(t, "create"), map[string]any{
		"subject":      "WhatsMiau Invite Test",
		"participants": []string{},
	})
	var created map[string]any
	mustDecode(t, createResp, &created)
	groupJID, _ := created["id"].(string)
	require.NotEmpty(t, groupJID)

	t.Cleanup(func() {
		resp := do(t, http.MethodDelete, groupURLQuery(t, "leaveGroup", "groupJid="+groupJID), nil)
		drainClose(resp)
	})

	resp := do(t, http.MethodPost, groupURL(t, "sendInvite"), map[string]any{
		"groupJid":    groupJID,
		"description": "Convite de teste automatizado",
		"numbers":     []string{number},
	})
	defer drainClose(resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	mustDecode(t, resp, &body)
	assert.NotEmpty(t, body["inviteUrl"])
	assert.NotNil(t, body["sent"])
}

// TestGroupValidation cobre rejeições esperadas sem precisar de instância conectada.
func TestGroupValidation(t *testing.T) {
	t.Run("CreateGroup_MissingSubject", func(t *testing.T) {
		resp := do(t, http.MethodPost, groupURL(t, "create"), map[string]any{
			"participants": []string{},
		})
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("FindGroupInfos_InvalidJID", func(t *testing.T) {
		// JID com servidor de usuário (@s.whatsapp.net) em vez de grupo (@g.us)
		resp := do(t, http.MethodGet, groupURLQuery(t, "findGroupInfos", "groupJid=5511999999999%40s.whatsapp.net"), nil)
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("UpdateParticipant_InvalidAction", func(t *testing.T) {
		resp := do(t, http.MethodPost, groupURL(t, "updateParticipant"), map[string]any{
			"groupJid":     "123456789@g.us",
			"action":       "kick",
			"participants": []string{"5511999999999"},
		})
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("UpdateSetting_InvalidAction", func(t *testing.T) {
		resp := do(t, http.MethodPost, groupURL(t, "updateSetting"), map[string]any{
			"groupJid": "123456789@g.us",
			"action":   "silent",
		})
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("ToggleEphemeral_InvalidExpiration", func(t *testing.T) {
		resp := do(t, http.MethodPost, groupURL(t, "toggleEphemeral"), map[string]any{
			"groupJid":   "123456789@g.us",
			"expiration": 999,
		})
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("UpdateGroupSubject_MissingGroupJid", func(t *testing.T) {
		resp := do(t, http.MethodPost, groupURL(t, "updateGroupSubject"), map[string]any{
			"subject": "Test",
		})
		defer drainClose(resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("FindGroupInfos_NotConnectedInstance", func(t *testing.T) {
		// Usa ID de instância que sabidamente não existe
		path := fmt.Sprintf("/v1/instance/%s/group/findGroupInfos?groupJid=123456789@g.us",
			"instance-que-nao-existe-xyz")
		resp := do(t, http.MethodGet, path, nil)
		defer drainClose(resp)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
