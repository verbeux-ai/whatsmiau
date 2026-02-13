package messages

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/verbeux-ai/whatsmiau/env"
)

type Store struct {
	db         *sql.DB
	isPostgres bool
}

func New(db *sql.DB) *Store {
	return &Store{
		db:         db,
		isPostgres: env.Env.DBDialect == "postgres",
	}
}

func (s *Store) Init(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS messages_cache (
			instance_id TEXT NOT NULL,
			remote_jid TEXT NOT NULL,
			message_id TEXT NOT NULL,
			ts BIGINT NOT NULL,
			wook_json TEXT NOT NULL,
			PRIMARY KEY (instance_id, message_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_cache_instance_remote_ts
			ON messages_cache(instance_id, remote_jid, ts DESC)`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			// Ignore duplicate index errors.
			if strings.Contains(stmt, "CREATE INDEX") && (strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "duplicate")) {
				continue
			}
			return err
		}
	}
	return nil
}

func (s *Store) Upsert(ctx context.Context, instanceID, remoteJid, messageID string, ts int64, wookJSON []byte) error {
	if instanceID == "" || remoteJid == "" || messageID == "" || len(wookJSON) == 0 {
		return nil
	}
	if ts <= 0 {
		ts = time.Now().Unix()
	}

	// Postgres and sqlite both support ON CONFLICT .. DO UPDATE.
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO messages_cache(instance_id, remote_jid, message_id, ts, wook_json)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT(instance_id, message_id)
		 DO UPDATE SET remote_jid=excluded.remote_jid, ts=excluded.ts, wook_json=excluded.wook_json`,
		instanceID, remoteJid, messageID, ts, string(wookJSON),
	)
	if err == nil {
		return nil
	}

	// sqlite uses ? placeholders.
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO messages_cache(instance_id, remote_jid, message_id, ts, wook_json)
		 VALUES (?,?,?,?,?)
		 ON CONFLICT(instance_id, message_id)
		 DO UPDATE SET remote_jid=excluded.remote_jid, ts=excluded.ts, wook_json=excluded.wook_json`,
		instanceID, remoteJid, messageID, ts, string(wookJSON),
	)
	return err
}

func (s *Store) List(ctx context.Context, instanceID, remoteJid string, before *int64, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var (
		rows *sql.Rows
		err  error
	)

	if before != nil {
		// Try postgres placeholders first, then sqlite.
		rows, err = s.db.QueryContext(ctx,
			`SELECT wook_json FROM messages_cache
			 WHERE instance_id=$1 AND remote_jid=$2 AND ts < $3
			 ORDER BY ts DESC
			 LIMIT $4`,
			instanceID, remoteJid, *before, limit,
		)
		if err != nil {
			rows, err = s.db.QueryContext(ctx,
				`SELECT wook_json FROM messages_cache
				 WHERE instance_id=? AND remote_jid=? AND ts < ?
				 ORDER BY ts DESC
				 LIMIT ?`,
				instanceID, remoteJid, *before, limit,
			)
		}
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT wook_json FROM messages_cache
			 WHERE instance_id=$1 AND remote_jid=$2
			 ORDER BY ts DESC
			 LIMIT $3`,
			instanceID, remoteJid, limit,
		)
		if err != nil {
			rows, err = s.db.QueryContext(ctx,
				`SELECT wook_json FROM messages_cache
				 WHERE instance_id=? AND remote_jid=?
				 ORDER BY ts DESC
				 LIMIT ?`,
				instanceID, remoteJid, limit,
			)
		}
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Return [] instead of null when empty.
	out := make([]json.RawMessage, 0)
	for rows.Next() {
		var sjson string
		if err := rows.Scan(&sjson); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(sjson))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func ParseBeforeParam(beforeStr string) (*int64, error) {
	if beforeStr == "" {
		return nil, nil
	}
	// Accept unix seconds or RFC3339.
	var sec int64
	_, err := fmt.Sscanf(beforeStr, "%d", &sec)
	if err == nil && sec > 0 {
		return &sec, nil
	}
	t, err := time.Parse(time.RFC3339, beforeStr)
	if err != nil {
		return nil, err
	}
	v := t.Unix()
	return &v, nil
}
