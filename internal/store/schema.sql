-- EvalHarness SQLite schema
-- Auto-applied on first run of evalctl

CREATE TABLE IF NOT EXISTS providers (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    kind        TEXT NOT NULL,  -- 'stt', 'tts', 'llm', 'agent'
    name        TEXT NOT NULL,
    version     TEXT NOT NULL DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(kind, name, version)
);

CREATE TABLE IF NOT EXISTS utterances (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    dataset     TEXT NOT NULL,
    language    TEXT NOT NULL,
    ref_text    TEXT NOT NULL,
    audio_path  TEXT NOT NULL,
    duration_ms INTEGER,
    UNIQUE(dataset, audio_path)
);

CREATE TABLE IF NOT EXISTS stt_runs (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    utterance_id  INTEGER NOT NULL REFERENCES utterances(id),
    provider_id   INTEGER NOT NULL REFERENCES providers(id),
    condition     TEXT NOT NULL DEFAULT 'clean',  -- 'clean', '8khz', '8khz_noise'
    hypothesis    TEXT NOT NULL DEFAULT '',
    latency_ms    INTEGER,
    cost_usd      REAL,
    input_hash    TEXT NOT NULL DEFAULT '',
    cached        BOOLEAN DEFAULT FALSE,
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(utterance_id, provider_id, condition, input_hash)
);

CREATE TABLE IF NOT EXISTS scenarios (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    language        TEXT NOT NULL,
    persona         TEXT NOT NULL DEFAULT '',
    goal            TEXT NOT NULL,
    criteria_json   TEXT NOT NULL DEFAULT '{}',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS calls (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    scenario_id     INTEGER NOT NULL REFERENCES scenarios(id),
    agent_id        INTEGER NOT NULL REFERENCES providers(id),
    run_index       INTEGER NOT NULL,
    transcript_json TEXT NOT NULL DEFAULT '[]',
    audio_path      TEXT NOT NULL DEFAULT '',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS scores (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    target_type TEXT NOT NULL,  -- 'stt_run', 'call'
    target_id   INTEGER NOT NULL,
    scorer      TEXT NOT NULL,  -- 'wer', 'cer', 'latency_p50', 'judge', etc.
    value       REAL NOT NULL,
    detail_json TEXT NOT NULL DEFAULT '{}',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS human_labels (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    call_id     INTEGER NOT NULL REFERENCES calls(id),
    label       TEXT NOT NULL,
    labeled_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Response cache: keyed by (provider, input_hash) so reruns are free
CREATE TABLE IF NOT EXISTS response_cache (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id INTEGER NOT NULL REFERENCES providers(id),
    input_hash  TEXT NOT NULL,
    response    TEXT NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider_id, input_hash)
);

CREATE INDEX IF NOT EXISTS idx_stt_runs_utterance ON stt_runs(utterance_id);
CREATE INDEX IF NOT EXISTS idx_stt_runs_provider ON stt_runs(provider_id);
CREATE INDEX IF NOT EXISTS idx_scores_target ON scores(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_cache_lookup ON response_cache(provider_id, input_hash);
