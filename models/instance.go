package models

type Instance struct {
	ID              string          `json:"id,omitempty"`
	RejectCall      bool            `json:"rejectCall,omitempty"`
	MsgCall         string          `json:"msgCall,omitempty"`
	GroupsIgnore    bool            `json:"groupsIgnore,omitempty"`
	AlwaysOnline    bool            `json:"alwaysOnline,omitempty"`
	ReadMessages    bool            `json:"readMessages,omitempty"`
	ReadStatus      bool            `json:"readStatus,omitempty"`
	SyncFullHistory bool            `json:"syncFullHistory,omitempty"`
	RemoteJID       string          `json:"remoteJID,omitempty"`
	Webhook         InstanceWebhook `json:"webhook,omitempty"`
}

type InstanceWebhook struct {
	Url      string                 `json:"url"`
	ByEvents bool                   `json:"byEvents"`
	Base64   bool                   `json:"base64"`
	Headers  InstanceWebhookHeaders `json:"headers"`
	Events   []string               `json:"events"`
}

type InstanceWebhookHeaders struct {
	Authorization string `json:"authorization"`
	ContentType   string `json:"Content-Type"` // Following EvolutionAPI Pattern
}
