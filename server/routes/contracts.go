package routes

import "net/http"

// Operation describes one documented HTTP operation. The inventory is used by
// the Swagger contract test to ensure compatibility aliases remain first-class
// documented operations instead of being accidentally dropped.
type Operation struct {
	Method string
	Path   string
}

// DocumentedV1Operations returns every authenticated /v1 operation registered
// by the application, including the two machine-readable Swagger downloads.
func DocumentedV1Operations() []Operation {
	operations := []Operation{{Method: http.MethodGet, Path: "/v1"}}
	appendOperation := func(method string, paths ...string) {
		for _, path := range paths {
			operations = append(operations, Operation{Method: method, Path: path})
		}
	}

	appendOperation(http.MethodPost, "/v1/instance", "/v1/instance/create", "/v1/instance/{id}/connect", "/v1/instance/{id}/logout", "/v1/instance/{id}/restart", "/v1/instance/restart/{id}")
	appendOperation(http.MethodGet, "/v1/instance", "/v1/instance/fetchInstances", "/v1/instance/connect/{id}", "/v1/instance/connect/{id}/image", "/v1/instance/{id}/status", "/v1/instance/connectionState/{id}")
	appendOperation(http.MethodDelete, "/v1/instance/{id}", "/v1/instance/logout/{id}", "/v1/instance/delete/{id}")
	appendOperation(http.MethodPut, "/v1/instance/update/{id}")

	appendOperation(http.MethodPost, "/v1/instance/{instance}/calls", "/v1/instance/{instance}/calls/{callID}/answer", "/v1/instance/{instance}/calls/{callID}/reject", "/v1/instance/{instance}/calls/{callID}/hangup")
	appendOperation(http.MethodGet, "/v1/instance/{instance}/calls", "/v1/instance/{instance}/calls/{callID}/audio")

	for _, action := range []string{"text", "audio", "document", "image", "video", "sticker", "location", "contact", "poll", "status", "list", "buttons"} {
		appendOperation(http.MethodPost, "/v1/instance/{instance}/message/"+action)
	}
	for _, action := range []string{"sendText", "sendWhatsAppAudio", "sendMedia", "sendPtv", "sendSticker", "sendLocation", "sendContact", "sendPoll", "sendStatus", "sendReaction", "sendList", "sendButtons"} {
		appendOperation(http.MethodPost, "/v1/message/"+action+"/{instance}")
	}

	appendOperation(http.MethodPost, "/v1/instance/{instance}/chat/presence", "/v1/instance/{instance}/chat/read-messages", "/v1/chat/markMessageAsRead/{instance}", "/v1/chat/sendPresence/{instance}", "/v1/chat/whatsappNumbers/{instance}")
	appendOperation(http.MethodDelete, "/v1/instance/{instance}/chat/deleteMessageForEveryone", "/v1/chat/deleteMessageForEveryone/{instance}")

	groupActions := []struct {
		method string
		action string
	}{
		{http.MethodPost, "create"},
		{http.MethodPost, "updateGroupSubject"},
		{http.MethodPost, "updateGroupPicture"},
		{http.MethodPost, "updateGroupDescription"},
		{http.MethodGet, "findGroupInfos"},
		{http.MethodGet, "fetchAllGroups"},
		{http.MethodGet, "participants"},
		{http.MethodGet, "inviteCode"},
		{http.MethodGet, "inviteInfo"},
		{http.MethodGet, "acceptInviteCode"},
		{http.MethodPost, "sendInvite"},
		{http.MethodPost, "revokeInviteCode"},
		{http.MethodPost, "updateParticipant"},
		{http.MethodPost, "updateSetting"},
		{http.MethodPost, "setMemberAddMode"},
		{http.MethodPost, "toggleEphemeral"},
		{http.MethodDelete, "leaveGroup"},
	}
	for _, action := range groupActions {
		appendOperation(action.method, "/v1/instance/{instance}/group/"+action.action, "/v1/group/"+action.action+"/{instance}")
	}
	appendOperation(http.MethodPost, "/v1/instance/{instance}/community/setGroupAddMode")

	for _, action := range []string{"create", "createSubGroup", "linkGroup", "unlinkGroup", "setJoinApprovalMode", "requestParticipants/update"} {
		appendOperation(http.MethodPost, "/v1/instance/{instance}/community/"+action)
	}
	for _, action := range []string{"subGroups", "linkedGroupsParticipants", "requestParticipants"} {
		appendOperation(http.MethodGet, "/v1/instance/{instance}/community/"+action)
	}

	appendOperation(http.MethodPost, "/v1/webhook/set/{instance}")
	appendOperation(http.MethodGet, "/v1/webhook/find/{instance}", "/v1/swagger.json", "/v1/swagger.yaml")
	return operations
}

// DocumentedDocumentationOperations returns browser documentation routes. The
// UI asset route is represented with a named parameter in Swagger and a
// wildcard in Echo because the Swagger UI library serves multiple asset names.
func DocumentedDocumentationOperations() []Operation {
	return []Operation{
		{Method: http.MethodGet, Path: "/docs"},
		{Method: http.MethodGet, Path: "/docs/swagger.json"},
		{Method: http.MethodGet, Path: "/docs/swagger.yaml"},
		{Method: http.MethodGet, Path: "/docs/{asset}"},
	}
}
