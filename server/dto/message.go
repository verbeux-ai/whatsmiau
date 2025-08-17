package dto

type SendTextRequest struct {
	InstanceID       string                `param:"instance"`
	Number           string                `json:"number,omitempty" validate:"required"` // JID
	Text             string                `json:"text,omitempty" validate:"required"`
	Delay            int                   `json:"delay,omitempty"`
	Quoted           *MessageRequestQuoted `json:"quoted,omitempty"`
	LinkPreview      bool                  `json:"linkPreview,omitempty"`
	MentionsEveryOne bool                  `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string              `json:"mentioned,omitempty"`
}

type MessageRequestQuoted struct {
	Key     QuotedKey     `json:"key,omitempty"`
	Message QuotedMessage `json:"message,omitempty"`
}

type QuotedKey struct {
	Id string `json:"id,omitempty"`
}

type QuotedMessage struct {
	Conversation string `json:"conversation,omitempty"`
}

type MessageResponseKey struct {
	RemoteJid string `json:"remoteJid,omitempty"`
	FromMe    bool   `json:"fromMe,omitempty"`
	Id        string `json:"id,omitempty"`
}

type SendTextResponse struct {
	Key              MessageResponseKey          `json:"key"`
	PushName         string                      `json:"pushName"`
	Status           string                      `json:"status"`
	Message          SendTextResponseMessage     `json:"message"`
	ContextInfo      SendTextResponseContextInfo `json:"contextInfo"`
	MessageType      string                      `json:"messageType"`
	MessageTimestamp int                         `json:"messageTimestamp"`
	InstanceId       string                      `json:"instanceId"`
	Source           string                      `json:"source"`
}

type SendTextResponseMessage struct {
	Conversation string `json:"conversation,omitempty"`
}

type SendTextResponseContextInfo struct {
	Participant   string                   `json:"participant,omitempty"`
	StanzaId      string                   `json:"stanzaId,omitempty"`
	QuotedMessage ContextInfoQuotedMessage `json:"quotedMessage,omitempty"`
}

type ContextInfoQuotedMessage struct {
	Conversation string `json:"conversation,omitempty"`
}
