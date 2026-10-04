// Package manifest handles loading utterance manifests from JSONL files.
// Audio files are never committed to the repo; manifests point to local paths.
package manifest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/kamalparashar/evalharness/internal/store"
)

// Entry represents one row in a manifest JSONL file.
type Entry struct {
	Dataset    string `json:"dataset"`
	Language   string `json:"language"`
	RefText    string `json:"ref_text"`
	AudioPath  string `json:"audio_path"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

// Load reads a JSONL manifest and inserts all utterances into the store.
// Returns the number of utterances loaded.
func Load(db *store.DB, manifestPath string) (int, error) {
	f, err := os.Open(manifestPath)
	if err != nil {
		return 0, fmt.Errorf("manifest: open %s: %w", manifestPath, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB lines
	count := 0
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			return count, fmt.Errorf("manifest: line %d: %w", lineNum, err)
		}

		if e.AudioPath == "" || e.RefText == "" {
			return count, fmt.Errorf("manifest: line %d: audio_path and ref_text are required", lineNum)
		}

		_, err := db.InsertUtterance(&store.Utterance{
			Dataset:    e.Dataset,
			Language:   e.Language,
			RefText:    e.RefText,
			AudioPath:  e.AudioPath,
			DurationMs: e.DurationMs,
		})
		if err != nil {
			return count, fmt.Errorf("manifest: line %d: insert: %w", lineNum, err)
		}
		count++
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("manifest: scan: %w", err)
	}

	return count, nil
}
