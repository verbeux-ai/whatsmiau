package interfaces

import (
	"github.com/verbeux-ai/whatsmiau/models"
	"golang.org/x/net/context"
)

type InstanceRepository interface {
	Create(ctx context.Context, instance *models.Instance) error
	List(ctx context.Context, id string) ([]models.Instance, error)
	Update(ctx context.Context, id string, instance *models.Instance) (*models.Instance, error)
	Delete(ctx context.Context, id string) error
}
