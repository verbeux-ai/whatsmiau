package services

import (
	"sync"
	"time"

	"github.com/verbeux-ai/whatsmiau/repositories/messages"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

var (
	messageStore   *messages.Store
	messageStoreMu sync.Mutex
)

func MessageStore() *messages.Store {
	messageStoreMu.Lock()
	defer messageStoreMu.Unlock()

	if messageStore != nil {
		return messageStore
	}

	store := messages.New(MessagesDB())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := store.Init(ctx); err != nil {
		zap.L().Panic("failed to init messages store", zap.Error(err))
	}

	messageStore = store
	return messageStore
}

