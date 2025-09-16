package dto

import "github.com/verbeux-ai/whatsmiau/models"

type CreateInstanceRequest struct {
	ID               string `json:"id,omitempty" validate:"required_without=InstanceName"`
	InstanceName     string `json:"instanceName,omitempty" validate:"required_without=InstanceID"`
	*models.Instance        // optional arguments
}

type CreateInstanceResponse struct {
	*models.Instance
}

type UpdateInstanceRequest struct {
	ID      string `json:"id,omitempty" param:"id" validate:"required"`
	Webhook struct {
		Base64 bool `json:"base64,omitempty"`
	} `json:"webhook,omitempty"`
}

type UpdateInstanceResponse struct {
	*models.Instance
}

type ListInstancesRequest struct {
	InstanceName string `query:"instanceName,omitempty"`
	ID           string `query:"id,omitempty"`
}

type ListInstancesResponse struct {
	*models.Instance

	OwnerJID string `json:"ownerJid,omitempty"`
}

type ConnectInstanceRequest struct {
	ID string `param:"id" validate:"required"`
}

type ConnectInstanceResponse struct {
	Message   string `json:"message,omitempty"`
	Connected bool   `json:"connected,omitempty"`
	Base64    string `json:"base64,omitempty"`
	*models.Instance
}

type StatusInstanceRequest struct {
	ID string `param:"id" validate:"required"`
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
	ID string `param:"id" validate:"required"`
}

type DeleteInstanceResponse struct {
	Message string `json:"message,omitempty"`
}

type LogoutInstanceRequest struct {
	ID string `param:"id" validate:"required"`
}

type LogoutInstanceResponse struct {
	Message string `json:"message,omitempty"`
}
