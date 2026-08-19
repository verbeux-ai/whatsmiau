package instances

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/verbeux-ai/whatsmiau/models"
)

func newTestRepository(t *testing.T) (*RedisInstance, *redis.Client, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedis(client), client, server
}

func TestCreateIsAtomic(t *testing.T) {
	repo, _, _ := newTestRepository(t)
	ctx := context.Background()

	if err := repo.Create(ctx, &models.Instance{ID: "same"}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if err := repo.Create(ctx, &models.Instance{ID: "same"}); !errors.Is(err, ErrorAlreadyExists) {
		t.Fatalf("second create error = %v, want ErrorAlreadyExists", err)
	}
}

type deleteBeforeConditionalSetHook struct {
	server *miniredis.Miniredis
	key    string
}

func (h deleteBeforeConditionalSetHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if cmd.Name() == "set" && strings.EqualFold(cmd.Args()[len(cmd.Args())-1].(string), "XX") {
		h.server.Del(h.key)
	}
	return ctx, nil
}

func (deleteBeforeConditionalSetHook) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (deleteBeforeConditionalSetHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (deleteBeforeConditionalSetHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func TestUpdateDoesNotResurrectDeletedKey(t *testing.T) {
	repo, client, server := newTestRepository(t)
	ctx := context.Background()
	instance := &models.Instance{ID: "race"}

	if err := repo.Create(ctx, instance); err != nil {
		t.Fatalf("create: %v", err)
	}
	client.AddHook(deleteBeforeConditionalSetHook{server: server, key: repo.key(instance.ID)})

	_, err := repo.Update(ctx, instance.ID, &models.Instance{RemoteJID: "123@s.whatsapp.net"})
	if !errors.Is(err, ErrorNotFound) {
		t.Fatalf("update error = %v, want ErrorNotFound", err)
	}
	if server.Exists(repo.key(instance.ID)) {
		t.Fatal("conditional update resurrected the deleted Redis key")
	}
}

func TestDeleteReportsMissingInstance(t *testing.T) {
	repo, _, _ := newTestRepository(t)
	if err := repo.Delete(context.Background(), "missing"); !errors.Is(err, ErrorNotFound) {
		t.Fatalf("delete error = %v, want ErrorNotFound", err)
	}
}

// TestUpdateOnlyTouchesSaveMediaWhenProvided guards the pointer semantics of SaveMedia.
// The value under test is true on purpose: with false, an update that wrongly forced the
// field would also produce false and the test would pass without detecting anything.
func TestUpdateOnlyTouchesSaveMediaWhenProvided(t *testing.T) {
	repo, _, _ := newTestRepository(t)
	ctx := context.Background()

	enabled, disabled := true, false
	if err := repo.Create(ctx, &models.Instance{ID: "media", SaveMedia: &enabled}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// An update that does not mention SaveMedia must leave it alone.
	updated, err := repo.Update(ctx, "media", &models.Instance{RemoteJID: "123@s.whatsapp.net"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.SaveMedia == nil {
		t.Fatal("update without SaveMedia cleared the stored value")
	}
	if !*updated.SaveMedia {
		t.Error("update without SaveMedia overwrote the stored value")
	}

	// An explicit false must be persisted, and must survive serialization: with a plain
	// bool on the model, omitempty would drop it and it would read back as absent.
	if _, err := repo.Update(ctx, "media", &models.Instance{SaveMedia: &disabled}); err != nil {
		t.Fatalf("update to false: %v", err)
	}

	stored, err := repo.List(ctx, "media")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(stored))
	}
	if stored[0].SaveMedia == nil {
		t.Fatal("explicit false vanished when serializing to Redis")
	}
	if *stored[0].SaveMedia {
		t.Error("explicit false was not persisted")
	}
}
