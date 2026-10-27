package services

import (
	"context"
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/XSAM/otelsql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/verbeux-ai/whatsmiau/env"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const sqliteDialect = "sqlite3"

var sqlStoreInstance *sqlstore.Container

var dbSystemByDialect = map[string]attribute.KeyValue{
	sqliteDialect: semconv.DBSystemNameSQLite,
	"postgres":    semconv.DBSystemNamePostgreSQL,
}

func SQLStore() *sqlstore.Container {
	ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()

	if sqlStoreInstance == nil {
		container, err := openSQLStore(ctx, env.Env.DBDialect, env.Env.DBURL)
		if err != nil {
			zap.L().Panic("failed to start sqlstore", zap.Error(err))
		}

		sqlStoreInstance = container
	}

	return sqlStoreInstance
}

func openSQLStore(ctx context.Context, dialect, address string) (*sqlstore.Container, error) {
	db, err := otelsql.Open(dialect, address,
		otelsql.WithAttributes(dbSystemByDialect[dialect]),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			SpanFilter: func(ctx context.Context, _ otelsql.Method, _ string, _ []driver.NamedValue) bool {
				return trace.SpanFromContext(ctx).IsRecording()
			},
			OmitConnectorConnect: true,
			OmitConnResetSession: true,
			OmitConnPrepare:      true,
			// go-sqlite3 only executes the statement on the first Next, so the rows span is the one measuring it.
			OmitRows:         dialect != sqliteDialect,
			RowsChildOfQuery: true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	container := sqlstore.NewWithDB(db, dialect, nil)
	if err := container.Upgrade(ctx); err != nil {
		_ = container.Close()
		return nil, fmt.Errorf("upgrade database: %w", err)
	}
	return container, nil
}
