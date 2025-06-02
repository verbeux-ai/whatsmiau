package instances

import (
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/verbeux-ai/whatsmiau/interface"
	"github.com/verbeux-ai/whatsmiau/models"
	"golang.org/x/net/context"
)

// These verify if RedisInstance follows instances interface pattern
var _ _interface.InstanceRepository = (*RedisInstance)(nil)

type RedisInstance struct {
	db *redis.Client
}

func (s *RedisInstance) key(id string) string {
	return fmt.Sprintf("instance_%s", id)
}

func NewRedis(client *redis.Client) *RedisInstance {
	return &RedisInstance{
		db: client,
	}
}

func (s *RedisInstance) Create(ctx context.Context, instance *models.Instance) error {
	if instance.ID == "" {
		return ErrInstanceIDEmpty
	}

	data, err := json.Marshal(instance)
	if err != nil {
		return err
	}
	return s.db.Set(ctx, s.key(instance.ID), data, redis.KeepTTL).Err()
}

func (s *RedisInstance) List(ctx context.Context) ([]models.Instance, error) {
	var (
		cursor uint64
		keys   []string
	)
	for {
		batch, newCursor, err := s.db.Scan(ctx, cursor, "instance_*", 100).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		cursor = newCursor
		if cursor == 0 {
			break
		}
	}

	if len(keys) == 0 {
		return []models.Instance{}, nil
	}

	rawVals, err := s.db.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	var instances []models.Instance
	for _, raw := range rawVals {
		if raw == nil {
			continue
		}
		strVal, ok := raw.(string)
		if !ok {
			continue
		}
		var inst models.Instance
		if err := json.Unmarshal([]byte(strVal), &inst); err != nil {
			continue
		}
		instances = append(instances, inst)
	}

	return instances, nil
}
