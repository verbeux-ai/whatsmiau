package dto

type ManagerLoginRequest struct {
	ApiKey string `form:"apiKey"`
}

type ManagerCreateInstanceRequest struct {
	InstanceName string `form:"instanceName" validate:"required"`
}

type ManagerInstanceCard struct {
	ID        string
	RemoteJID string
	Status    string
}

type ManagerUpdateInstanceRequest struct {
	WebhookURL      string   `form:"webhookUrl" validate:"omitempty,http_url"`
	WebhookBase64   *bool    `form:"webhookBase64"`
	WebhookByEvents *bool    `form:"webhookByEvents"`
	WebhookEvents   []string `form:"webhookEvents"`

	ProxyHost     string `form:"proxyHost"`
	ProxyPort     string `form:"proxyPort"`
	ProxyProtocol string `form:"proxyProtocol"`
	ProxyUsername string `form:"proxyUsername"`
	ProxyPassword string `form:"proxyPassword"`
}
