package instances

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/models"
	"golang.org/x/net/context"
)

// These verify if RedisInstance follows instances interface pattern
var _ interfaces.InstanceRepository = (*RedisInstance)(nil)

var ErrorNotFound = errors.New("not found")
var ErrorAlreadyExists = errors.New("instance already exists")

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

	result, err := s.List(ctx, instance.ID)
	if err != nil {
		return err
	}

	if len(result) > 0 {
		return ErrorAlreadyExists
	}

	data, err := json.Marshal(instance)
	if err != nil {
		return err
	}
	return s.db.Set(ctx, s.key(instance.ID), data, redis.KeepTTL).Err()
}

func (s *RedisInstance) Update(ctx context.Context, id string, toUpdate *models.Instance) (*models.Instance, error) {
	if id == "" {
		return nil, ErrInstanceIDEmpty
	}

	result, err := s.List(ctx, id)
	if err != nil {
		return nil, err
	}

	if len(result) <= 0 {
		return nil, ErrorNotFound
	}

	oldInstance := result[0]
	if len(toUpdate.RemoteJID) > 0 {
		oldInstance.RemoteJID = toUpdate.RemoteJID
	}
	if toUpdate.Webhook.Url != "" {
		oldInstance.Webhook.Url = toUpdate.Webhook.Url
	}
	if toUpdate.Webhook.ByEvents != nil {
		oldInstance.Webhook.ByEvents = toUpdate.Webhook.ByEvents
	}
	if toUpdate.Webhook.Base64 != nil {
		oldInstance.Webhook.Base64 = toUpdate.Webhook.Base64
	}
	if toUpdate.Webhook.Headers != nil {
		if oldInstance.Webhook.Headers == nil {
			oldInstance.Webhook.Headers = map[string]string{}
		}
		for k, v := range toUpdate.Webhook.Headers {
			oldInstance.Webhook.Headers[k] = v
		}
	}
	if toUpdate.Webhook.Events != nil && len(toUpdate.Webhook.Events) > 0 {
		oldInstance.Webhook.Events = toUpdate.Webhook.Events
	}

	data, err := json.Marshal(oldInstance)
	if err != nil {
		return nil, err
	}

	return &oldInstance, s.db.Set(ctx, s.key(id), data, redis.KeepTTL).Err()
}

func (s *RedisInstance) List(ctx context.Context, id string) ([]models.Instance, error) {
	var (
		cursor uint64
		keys   []string
	)

	pattern := "instance_"
	if len(id) > 0 {
		pattern = fmt.Sprintf("instance_%s", id)
	} else {
		pattern += "*"
	}

	for {
		batch, newCursor, err := s.db.Scan(ctx, cursor, pattern, 100).Result()
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

func (s *RedisInstance) Delete(ctx context.Context, id string) error {
	if id == "" {
		return ErrInstanceIDEmpty
	}

	result, err := s.List(ctx, id)
	if err != nil {
		return err
	}

	if len(result) <= 0 {
		return ErrorNotFound
	}

	return s.db.Del(ctx, s.key(id)).Err()
}
