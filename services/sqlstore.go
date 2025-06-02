package services

import (
	"context"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.uber.org/zap"
	"time"
)

var sqlStoreInstance *sqlstore.Container

func SQLStore() *sqlstore.Container {
	ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()

	if sqlStoreInstance == nil {
		container, err := sqlstore.New(ctx, "sqlite3", "file:data.db?_foreign_keys=on", nil)
		if err != nil {
			zap.L().Fatal("failed to start sqlstore", zap.Error(err))
		}

		sqlStoreInstance = container
	}

	return sqlStoreInstance
}
