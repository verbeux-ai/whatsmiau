package models

type Instance struct {
	ID                string          `json:"id,omitempty"`
	RejectCall        bool            `json:"rejectCall,omitempty"`
	MsgCall           string          `json:"msgCall,omitempty"`
	GroupsIgnore      bool            `json:"groupsIgnore,omitempty"`
	AlwaysOnline      bool            `json:"alwaysOnline,omitempty"`
	ReadMessages      bool            `json:"readMessages,omitempty"`
	ReadStatus        bool            `json:"readStatus,omitempty"`
	SyncFullHistory   bool            `json:"syncFullHistory,omitempty"`
	SyncRecentHistory bool            `json:"syncRecentHistory,omitempty"`
	RemoteJID         string          `json:"remoteJID,omitempty"`
	Webhook           InstanceWebhook `json:"webhook,omitempty"`
	InstanceProxy
}

type InstanceProxy struct {
	ProxyHost     string `json:"proxyHost,omitempty"`
	ProxyPort     string `json:"proxyPort,omitempty"`
	ProxyProtocol string `json:"proxyProtocol,omitempty"`
	ProxyUsername string `json:"proxyUsername,omitempty"`
	ProxyPassword string `json:"proxyPassword,omitempty"`
}

type InstanceWebhook struct {
	Url      string            `json:"url,omitempty"`
	ByEvents *bool             `json:"byEvents,omitempty"`
	Base64   *bool             `json:"base64,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Events   []string          `json:"events,omitempty"`
}
