package dto

import (
	"encoding/json"

	"github.com/verbeux-ai/whatsmiau/models"
)

type CreateInstanceRequest struct {
	ID               string         `json:"id,omitempty" validate:"required_without=InstanceName"`
	InstanceName     string         `json:"instanceName,omitempty" validate:"required_without=InstanceID"`
	Migration        *MigrationData `json:"migration,omitempty"`
	*models.Instance                // optional arguments
}

type MigrationData struct {
	Creds   json.RawMessage   `json:"creds" validate:"required"`
	PreKeys []MigrationPreKey `json:"preKeys,omitempty"`
}

type MigrationPreKey struct {
	KeyID   uint32          `json:"keyId"`
	Private MigrationBuffer `json:"private"`
}

type MigrationBuffer struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type CreateInstanceResponse struct {
	*models.Instance
	Migration *MigrationResult `json:"migration,omitempty"`
}

type MigrationResult struct {
	JID       string `json:"jid"`
	LID       string `json:"lid,omitempty"`
	PreKeys   int    `json:"preKeysImported"`
	Connected bool   `json:"connected"`
}

type UpdateInstanceRequest struct {
	ID      string `json:"id,omitempty" param:"id" validate:"required" swaggerignore:"true"`
	Webhook struct {
		Enabled *bool    `json:"enabled,omitempty"`
		Base64  bool     `json:"base64,omitempty"`
		URL     string   `json:"url,omitempty"`
		Events  []string `json:"events,omitempty"`
	} `json:"webhook,omitempty"`
	models.InstanceProxy
}

type UpdateInstanceResponse struct {
	*models.Instance
}

type ListInstancesRequest struct {
	InstanceName string `query:"instanceName"`
	ID           string `query:"id"`
}

type ListInstancesResponse struct {
	*models.Instance

	OwnerJID     string `json:"ownerJid,omitempty"`
	InstanceName string `json:"instanceName,omitempty"`
}

type ConnectInstanceRequest struct {
	ID     string `param:"id" validate:"required" swaggerignore:"true"`
	Number string `json:"number" query:"number"`
}

type ConnectInstanceResponse struct {
	Message     string `json:"message,omitempty"`
	Connected   bool   `json:"connected,omitempty"`
	Base64      string `json:"base64,omitempty"`
	PairingCode string `json:"pairingCode,omitempty"`
	*models.Instance
}

type StatusInstanceRequest struct {
	ID string `param:"id" validate:"required" swaggerignore:"true"`
}

type StatusInstanceResponse struct {
	ID       string                                        `json:"id,omitempty"`
	Status   string                                        `json:"state,omitempty"`
	Instance *StatusInstanceResponseEvolutionCompatibility `json:"instance,omitempty"`
}

type StatusInstanceResponseEvolutionCompatibility struct {
	InstanceName string `json:"instanceName,omitempty"`
	State        string `json:"state,omitempty"`
}

type DeleteInstanceRequest struct {
	ID string `param:"id" validate:"required" swaggerignore:"true"`
}

type DeleteInstanceResponse struct {
	Message string `json:"message,omitempty"`
}

type LogoutInstanceRequest struct {
	ID string `param:"id" validate:"required" swaggerignore:"true"`
}

type LogoutInstanceResponse struct {
	Message string `json:"message,omitempty"`
}

type RestartInstanceRequest struct {
	ID string `param:"id" validate:"required" swaggerignore:"true"`
}

type RestartInstanceResponse struct {
	ID       string                          `json:"id,omitempty"`
	Status   string                          `json:"state,omitempty"`
	Instance *RestartInstanceEvoCompatibility `json:"instance,omitempty"`
}

type RestartInstanceEvoCompatibility struct {
	InstanceName string `json:"instanceName,omitempty"`
	Status       string `json:"status,omitempty"`
}
