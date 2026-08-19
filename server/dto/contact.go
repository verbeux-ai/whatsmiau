package dto

type ListContactsQuery struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
}
