package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
)

type Store struct{ db *sql.DB }

func OpenStore(path string) (*Store, Config, error) {
	var c Config
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, c, e
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, c, e
	}
	f.Close()
	if e = os.Chmod(path, 0600); e != nil {
		return nil, c, e
	}
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, c, e
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	fail := func(e error) (*Store, Config, error) { db.Close(); return nil, c, e }
	for _, q := range []string{"PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", "PRAGMA busy_timeout=5000", `CREATE TABLE IF NOT EXISTS configuration (id INTEGER PRIMARY KEY CHECK(id=1), value TEXT NOT NULL)`, `CREATE TABLE IF NOT EXISTS events (id INTEGER PRIMARY KEY AUTOINCREMENT, value TEXT NOT NULL)`} {
		if _, e = db.Exec(q); e != nil {
			return fail(e)
		}
	}
	var raw string
	e = db.QueryRow("SELECT value FROM configuration WHERE id=1").Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		c = DefaultConfig()
		if e = s.Save(c); e != nil {
			return fail(e)
		}
	} else if e != nil {
		return fail(e)
	} else if e = json.Unmarshal([]byte(raw), &c); e != nil {
		return fail(fmt.Errorf("configuration is invalid: %w", e))
	}
	oldSchema := c.SchemaVersion
	migrateConfig(&c)
	if e = validateConfig(c); e != nil {
		return fail(e)
	}
	if oldSchema != c.SchemaVersion {
		if e = s.Save(c); e != nil {
			return fail(e)
		}
	}
	return s, c, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Save(c Config) error {
	b, e := json.Marshal(c)
	if e != nil {
		return e
	}
	_, e = s.db.Exec("INSERT INTO configuration(id,value) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET value=excluded.value", string(b))
	return e
}
func (s *Store) Record(event Event) (Event, error) {
	b, e := json.Marshal(event)
	if e != nil {
		return event, e
	}
	tx, e := s.db.Begin()
	if e != nil {
		return event, e
	}
	defer tx.Rollback()
	r, e := tx.Exec("INSERT INTO events(value) VALUES(?)", string(b))
	if e != nil {
		return event, e
	}
	event.ID, e = r.LastInsertId()
	if e != nil {
		return event, e
	}
	if _, e = tx.Exec("DELETE FROM events WHERE id <= ?", event.ID-500); e != nil {
		return event, e
	}
	return event, tx.Commit()
}
func (s *Store) Events(limit int) ([]Event, error) {
	rows, e := s.db.Query("SELECT id,value FROM events ORDER BY id DESC LIMIT ?", limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var id int64
		var raw string
		if e = rows.Scan(&id, &raw); e != nil {
			return nil, e
		}
		var ev Event
		if e = json.Unmarshal([]byte(raw), &ev); e != nil {
			return nil, e
		}
		ev.ID = id
		out = append(out, ev)
	}
	return out, rows.Err()
}
