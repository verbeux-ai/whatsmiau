package models

type Instance struct {
	ID              string          `json:"id"`
	RejectCall      bool            `json:"rejectCall"`
	MsgCall         string          `json:"msgCall"`
	GroupsIgnore    bool            `json:"groupsIgnore"`
	AlwaysOnline    bool            `json:"alwaysOnline"`
	ReadMessages    bool            `json:"readMessages"`
	ReadStatus      bool            `json:"readStatus"`
	SyncFullHistory bool            `json:"syncFullHistory"`
	Webhook         InstanceWebhook `json:"webhook"`
}

type InstanceWebhook struct {
	Url      string                 `json:"url"`
	ByEvents bool                   `json:"byEvents"`
	Base64   bool                   `json:"base64"`
	Headers  InstanceWebhookHeaders `json:"headers"`
	Events   []string               `json:"events"`
}

type InstanceWebhookHeaders struct {
	Autorization string `json:"autorization"`
	ContentType  string `json:"Content-Type"` // Following EvolutionAPI Pattern
}
