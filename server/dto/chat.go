package dto

type ReadMessagesRequest struct {
	InstanceID   string                    `param:"instance" validate:"required" swaggerignore:"true"`
	ReadMessages []ReadMessagesRequestItem `json:"readMessages" validate:"required,min=1"`
}
type ReadMessagesRequestItem struct {
	RemoteJid string `json:"remoteJid" validate:"required"`
	//FromMe    bool   `json:"fromMe"` ignored
	Sender string `json:"sender"` // required if group
	ID     string `json:"id" validate:"required"`
	// Played marks a voice note as heard (blue microphone) instead of merely
	// read (blue ticks). Audio and PTT messages only.
	Played bool `json:"played"`
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
	InstanceID string                      `param:"instance" validate:"required" swaggerignore:"true"`
	Number     string                      `json:"number"`
	Delay      int                         `json:"delay,omitempty" validate:"omitempty,min=0,max=300000"`
	Presence   SendPresenceRequestPresence `json:"presence"`
	Type       SendPresenceRequestType     `json:"type"`
}

type SendChatPresenceResponse struct {
	Presence SendPresenceRequestPresence `json:"presence"`
}

type NumberExistsRequest struct {
	Numbers []string `json:"numbers"     validate:"required,min=1,dive,required"`
}

type DeleteMessageForEveryoneRequest struct {
	InstanceID  string `param:"instance" validate:"required" swaggerignore:"true"`
	ID          string `json:"id" validate:"required"`
	RemoteJid   string `json:"remoteJid" validate:"required"`
	Participant string `json:"participant,omitempty" validate:"omitempty"`
	FromMe      bool   `json:"fromMe"`
}

// UpdateMessageRequest mirrors Evolution API's POST /chat/updateMessage
// payload: the destination number, the new text and the key of the message
// being edited.
type UpdateMessageRequest struct {
	InstanceID string            `param:"instance" validate:"required" swaggerignore:"true"`
	Number     string            `json:"number" validate:"required"`
	Text       string            `json:"text" validate:"required"`
	Key        UpdateMessageKey  `json:"key" validate:"required"`
}

type UpdateMessageKey struct {
	Id          string `json:"id" validate:"required"`
	RemoteJid   string `json:"remoteJid" validate:"required"`
	FromMe      *bool  `json:"fromMe" validate:"required"`
	Participant string `json:"participant,omitempty"`
}

type UpdateMessageResponse struct {
	Key              MessageResponseKey      `json:"key"`
	Status           string                  `json:"status"`
	Message          SendTextResponseMessage `json:"message"`
	MessageType      string                  `json:"messageType"`
	MessageTimestamp int                     `json:"messageTimestamp"`
	InstanceId       string                  `json:"instanceId"`
}

// FetchProfilePictureRequest mirrors Evolution API's POST /chat/fetchProfilePictureUrl
// payload: the number or JID (user or group) whose profile picture URL is requested.
type FetchProfilePictureRequest struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
	Number     string `json:"number" validate:"required"`
}

type FetchProfilePictureResponse struct {
	Wuid              string  `json:"wuid"`
	ProfilePictureUrl *string `json:"profilePictureUrl"`
}

type SyncChatMessagesRequest struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
	Number     string `json:"number" validate:"required"`
	ID         string `json:"id" validate:"required"`
	Count      int    `json:"count" validate:"omitempty,min=1,max=100"`
	Since      string `json:"since,omitempty"`
	FromMe     *bool  `json:"fromMe,omitempty"`
}
