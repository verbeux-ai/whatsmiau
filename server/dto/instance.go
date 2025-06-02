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

type ListInstancesResponse struct {
	Instances []models.Instance `json:"instances"`
}
