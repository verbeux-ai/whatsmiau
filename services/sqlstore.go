package services

import (
	"context"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/verbeux-ai/whatsmiau/env"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.uber.org/zap"
)

var sqlStoreInstance *sqlstore.Container

func SQLStore() *sqlstore.Container {
	ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()

	if sqlStoreInstance == nil {
		container, err := sqlstore.New(ctx, env.Env.DBDialect, env.Env.DBURL, nil)
		if err != nil {
			zap.L().Fatal("failed to start sqlstore", zap.Error(err))
		}

		sqlStoreInstance = container
	}

	return sqlStoreInstance
}
