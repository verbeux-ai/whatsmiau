package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/verbeux-ai/whatsmiau/models"
	"go.mau.fi/whatsmeow"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

func (s *Whatsmiau) getInstanceCached(id string) *models.Instance {
	instanceCached, ok := s.instanceCache.Load(id)
	if ok {
		return &instanceCached
	}

	ctx, c := context.WithTimeout(context.Background(), time.Second*5)
	defer c()

	res, err := s.repo.List(ctx, id)
	if err != nil {
		zap.L().Panic("failed to get instanceCached by instance", zap.Error(err))
	}

	if len(res) == 0 {
		zap.L().Warn("no instanceCached found by instance", zap.String("instance", id))
		return nil
	}

	s.instanceCache.Store(id, res[0])
	go func() {
		// expiry in 10sec
		time.Sleep(time.Second * 10)
		s.instanceCache.Delete(id)
	}()

	return &res[0]
}

func (s *Whatsmiau) StartEmitter(id string) whatsmeow.EventHandler {
	logFile, err := os.OpenFile(fmt.Sprintf("./%s-%s.jsonl", id, time.Now().Format(time.RFC3339)), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	return func(evt any) {
		//instance := s.getInstanceCached(id)
		data, _ := json.Marshal(evt)
		logFile.WriteString(string(data) + "\n")
	}
}
