package _interface

import (
	"github.com/verbeux-ai/whatsmiau/models"
	"golang.org/x/net/context"
)

type InstanceRepository interface {
	Create(ctx context.Context, instance *models.Instance) error
	List(ctx context.Context) ([]models.Instance, error)
}
