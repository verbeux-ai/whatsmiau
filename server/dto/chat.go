package dto

type ReadMessagesRequest struct {
	InstanceID   string                    `param:"instance" validate:"required"`
	ReadMessages []ReadMessagesRequestItem `json:"readMessages" validate:"required,min=1"`
}
type ReadMessagesRequestItem struct {
	RemoteJid string `json:"remoteJid" validate:"required"`
	//FromMe    bool   `json:"fromMe"` ignored
	Sender string `json:"sender"` // required if group
	ID     string `json:"id" validate:"required"`
}

type SendPresenceRequestPresence string

const (
	PresenceComposing SendPresenceRequestPresence = "composing"
	PresenceAvailable SendPresenceRequestPresence = "available"
)

type SendPresenceRequestType string

const (
	PresenceTypeText  SendPresenceRequestType = "text"
	PresenceTypeAudio SendPresenceRequestType = "audio"
)

type SendChatPresenceRequest struct {
	InstanceID string                      `param:"instance" validate:"required"`
	Number     string                      `json:"number"`
	Delay      int                         `json:"delay,omitempty" validate:"omitempty,min=0,max=300000"`
	Presence   SendPresenceRequestPresence `json:"presence"`
	Type       SendPresenceRequestType     `json:"type"`
}

type SendChatPresenceResponse struct {
	Presence SendPresenceRequestPresence `json:"presence"`
}
