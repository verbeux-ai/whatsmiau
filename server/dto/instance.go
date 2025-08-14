package dto

import "github.com/verbeux-ai/whatsmiau/models"

type CreateInstanceRequest struct {
	ID           string `json:"id" validate:"required_without=InstanceName"`
	InstanceName string `json:"instanceName" validate:"required_without=ID"`

	models.Instance // optional arguments
}

type CreateInstanceResponse struct {
	models.Instance
}

type ListInstancesRequest struct {
	InstanceName string `query:"instanceName"`
	ID           string `query:"id"`
}

type ConnectInstanceRequest struct {
	ID string `param:"id" validate:"required"`
}

type ConnectInstanceResponse struct {
	Message   string `json:"message"`
	Connected bool   `json:"connected"`
	Base64    string `json:"base64,omitempty"`
	models.Instance
}

type StatusInstanceRequest struct {
	ID string `param:"id" validate:"required"`
}

type StatusInstanceResponse struct {
	ID       string                                        `json:"id"`
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
