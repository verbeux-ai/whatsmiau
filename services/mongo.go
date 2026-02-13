package services

import (
	"context"
	"sync"
	"time"

	"github.com/verbeux-ai/whatsmiau/env"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

var (
	mongoClient   *mongo.Client
	mongoClientMu sync.Mutex
)

func Mongo() *mongo.Client {
	mongoClientMu.Lock()
	defer mongoClientMu.Unlock()

	if mongoClient != nil {
		return mongoClient
	}

	uri := env.Env.MongoURI
	if uri == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		zap.L().Panic("failed to connect to mongo", zap.Error(err))
	}
	if err := client.Ping(ctx, nil); err != nil {
		zap.L().Panic("failed to ping mongo", zap.Error(err))
	}

	mongoClient = client
	return mongoClient
}

