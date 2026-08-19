package dto

import "github.com/verbeux-ai/whatsmiau/models"

type SetWebhookRequest struct {
	InstanceID string                `param:"instance" validate:"required" swaggerignore:"true"`
	Webhook    SetWebhookRequestData `json:"webhook" validate:"required"`
}

type SetWebhookRequestData struct {
	Enabled  *bool             `json:"enabled,omitempty"`
	URL      string            `json:"url,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	ByEvents *bool             `json:"byEvents,omitempty"`
	Base64   *bool             `json:"base64,omitempty"`
	Events   []string          `json:"events,omitempty"`
}

type SetWebhookResponse struct {
	Webhook *models.InstanceWebhook `json:"webhook"`
}

type FindWebhookRequest struct {
	InstanceID string `param:"instance" validate:"required"`
}

type FindWebhookResponse struct {
	Webhook *models.InstanceWebhook `json:"webhook"`
}
