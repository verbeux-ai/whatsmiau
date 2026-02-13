package services

import (
	"database/sql"
	"sync"

	"github.com/verbeux-ai/whatsmiau/env"
	"go.uber.org/zap"
)

var (
	messagesDB   *sql.DB
	messagesDBMu sync.Mutex
)

// MessagesDB opens a generic SQL connection to the same database URL as the whatsmeow sqlstore.
// We use it for our own tables (history/message cache).
func MessagesDB() *sql.DB {
	messagesDBMu.Lock()
	defer messagesDBMu.Unlock()

	if messagesDB != nil {
		return messagesDB
	}

	db, err := sql.Open(env.Env.DBDialect, env.Env.DBURL)
	if err != nil {
		zap.L().Panic("failed to open messages db", zap.Error(err))
	}

	// Keep it conservative, sqlite doesn't like high concurrency.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	messagesDB = db
	return messagesDB
}

