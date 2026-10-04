// Package store provides the SQLite persistence layer for evalharness.
package store

import (
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

// DB wraps a SQLite connection and provides typed queries.
type DB struct {
	conn *sql.DB
}

// Open opens (or creates) the SQLite database at path and applies the schema.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	if _, err := conn.Exec(schemaSQL); err != nil {
		conn.Close()
		return nil, fmt.Errorf("store: migrate: %w", err)
	}
	return &DB{conn: conn}, nil
}

// Close closes the underlying connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// --- Providers ---

// UpsertProvider inserts or finds a provider, returning its ID.
func (db *DB) UpsertProvider(kind, name, version string) (int64, error) {
	res, err := db.conn.Exec(
		`INSERT INTO providers (kind, name, version) VALUES (?, ?, ?)
		 ON CONFLICT(kind, name, version) DO NOTHING`, kind, name, version)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil || id == 0 {
		// Row already existed; look it up.
		row := db.conn.QueryRow(
			`SELECT id FROM providers WHERE kind = ? AND name = ? AND version = ?`,
			kind, name, version)
		if err := row.Scan(&id); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// --- Utterances ---

// Utterance represents a single audio utterance with reference text.
type Utterance struct {
	ID         int64
	Dataset    string
	Language   string
	RefText    string
	AudioPath  string
	DurationMs int64
}

// InsertUtterance inserts an utterance, ignoring duplicates.
func (db *DB) InsertUtterance(u *Utterance) (int64, error) {
	res, err := db.conn.Exec(
		`INSERT INTO utterances (dataset, language, ref_text, audio_path, duration_ms)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(dataset, audio_path) DO NOTHING`,
		u.Dataset, u.Language, u.RefText, u.AudioPath, u.DurationMs)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil || id == 0 {
		row := db.conn.QueryRow(
			`SELECT id FROM utterances WHERE dataset = ? AND audio_path = ?`,
			u.Dataset, u.AudioPath)
		if err := row.Scan(&id); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// ListUtterances returns all utterances for a given language (or all if lang is empty).
func (db *DB) ListUtterances(lang string) ([]Utterance, error) {
	q := `SELECT id, dataset, language, ref_text, audio_path, COALESCE(duration_ms, 0) FROM utterances`
	var args []interface{}
	if lang != "" {
		q += ` WHERE language = ?`
		args = append(args, lang)
	}
	q += ` ORDER BY id`
	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Utterance
	for rows.Next() {
		var u Utterance
		if err := rows.Scan(&u.ID, &u.Dataset, &u.Language, &u.RefText, &u.AudioPath, &u.DurationMs); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// --- STT Runs ---

// STTRun stores the result of a single STT invocation.
type STTRun struct {
	ID          int64
	UtteranceID int64
	ProviderID  int64
	Condition   string
	Hypothesis  string
	LatencyMs   int64
	CostUSD     float64
	InputHash   string
	Cached      bool
}

// InsertSTTRun inserts a run result, returning the ID.
func (db *DB) InsertSTTRun(r *STTRun) (int64, error) {
	res, err := db.conn.Exec(
		`INSERT INTO stt_runs (utterance_id, provider_id, condition, hypothesis, latency_ms, cost_usd, input_hash, cached)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(utterance_id, provider_id, condition, input_hash) DO UPDATE SET
		   hypothesis = excluded.hypothesis,
		   latency_ms = excluded.latency_ms,
		   cost_usd   = excluded.cost_usd,
		   cached     = excluded.cached`,
		r.UtteranceID, r.ProviderID, r.Condition, r.Hypothesis,
		r.LatencyMs, r.CostUSD, r.InputHash, r.Cached)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListSTTRuns returns all STT runs, optionally filtered by provider and/or condition.
func (db *DB) ListSTTRuns(providerID int64, condition string) ([]STTRun, error) {
	q := `SELECT r.id, r.utterance_id, r.provider_id, r.condition, r.hypothesis,
	             COALESCE(r.latency_ms, 0), COALESCE(r.cost_usd, 0), r.input_hash, r.cached
	      FROM stt_runs r WHERE 1=1`
	var args []interface{}
	if providerID > 0 {
		q += ` AND r.provider_id = ?`
		args = append(args, providerID)
	}
	if condition != "" {
		q += ` AND r.condition = ?`
		args = append(args, condition)
	}
	q += ` ORDER BY r.id`

	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []STTRun
	for rows.Next() {
		var r STTRun
		if err := rows.Scan(&r.ID, &r.UtteranceID, &r.ProviderID, &r.Condition,
			&r.Hypothesis, &r.LatencyMs, &r.CostUSD, &r.InputHash, &r.Cached); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Scores ---

// Score represents a computed metric.
type Score struct {
	ID         int64
	TargetType string
	TargetID   int64
	Scorer     string
	Value      float64
	DetailJSON string
}

// InsertScore inserts a score entry.
func (db *DB) InsertScore(s *Score) (int64, error) {
	res, err := db.conn.Exec(
		`INSERT INTO scores (target_type, target_id, scorer, value, detail_json)
		 VALUES (?, ?, ?, ?, ?)`,
		s.TargetType, s.TargetID, s.Scorer, s.Value, s.DetailJSON)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// --- Response Cache ---

// CacheLookup checks the cache for a previous response.
func (db *DB) CacheLookup(providerID int64, inputHash string) (string, bool, error) {
	var resp string
	err := db.conn.QueryRow(
		`SELECT response FROM response_cache WHERE provider_id = ? AND input_hash = ?`,
		providerID, inputHash).Scan(&resp)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return resp, true, nil
}

// CacheStore stores a response in the cache.
func (db *DB) CacheStore(providerID int64, inputHash, response string) error {
	_, err := db.conn.Exec(
		`INSERT INTO response_cache (provider_id, input_hash, response)
		 VALUES (?, ?, ?)
		 ON CONFLICT(provider_id, input_hash) DO UPDATE SET response = excluded.response`,
		providerID, inputHash, response)
	return err
}

// --- Helpers ---

// GetUtterance fetches a single utterance by ID.
func (db *DB) GetUtterance(id int64) (*Utterance, error) {
	u := &Utterance{}
	err := db.conn.QueryRow(
		`SELECT id, dataset, language, ref_text, audio_path, COALESCE(duration_ms, 0)
		 FROM utterances WHERE id = ?`, id).
		Scan(&u.ID, &u.Dataset, &u.Language, &u.RefText, &u.AudioPath, &u.DurationMs)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// HashInput produces a deterministic SHA-256 hex hash for cache keying.
func HashInput(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0}) // separator
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
