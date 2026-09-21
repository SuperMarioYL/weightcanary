// Package state owns the availability ledger: the snapshot schema persisted to
// state.json plus its load and save. The file is meant to be committed to git,
// so history doubles as the audit trail of what was available and when.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// SchemaVersion is written into every snapshot; bump it when the ledger shape
// changes incompatibly.
const SchemaVersion = 1

// FileState is one file of a repo as observed by a registry poller.
type FileState struct {
	Size      int64  `json:"size"`
	LfsSha256 string `json:"lfs_sha256,omitempty"` // set for Git LFS weight files
	Present   bool   `json:"present"`
}

// RegistryState is the observed state of one repo on one registry.
type RegistryState struct {
	RepoID    string               `json:"repo_id"`
	Present   bool                 `json:"present"`
	CommitSha string               `json:"commit_sha,omitempty"`
	Files     map[string]FileState `json:"files,omitempty"`
	// Partial marks a listing where some files could not be fully resolved —
	// e.g. Gitee LFS pointer decoding blocked by rate limiting. File
	// presence and sizes stay real; per-file hashes may be missing, so the
	// report treats such rows as partially verified instead of divergence.
	Partial bool `json:"partial,omitempty"`
	// Error records a poll failure. A state with an Error is not a valid
	// observation: rendering surfaces it instead of comparing it.
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

// AvailabilityRecord is one watched model across registries — the core ledger
// primitive: per-model availability state that can be diffed over time.
type AvailabilityRecord struct {
	ModelID   string                   `json:"model_id"`
	HF        RegistryState            `json:"hf"`
	Mirrors   map[string]RegistryState `json:"mirrors,omitempty"`
	CheckedAt time.Time                `json:"checked_at"`
}

// Snapshot is one full pass over the watchlist.
type Snapshot struct {
	Version     int                  `json:"version"`
	GeneratedAt time.Time            `json:"generated_at"`
	Models      []AvailabilityRecord `json:"models"`
}

// Save writes the snapshot as pretty-printed JSON, via a temp file + rename so
// an interrupted run never leaves a half-written ledger behind.
func Save(path string, s *Snapshot) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load reads a snapshot written by Save.
func Load(path string) (*Snapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if s.Version != SchemaVersion {
		return nil, fmt.Errorf("%s: ledger schema version %d, want %d — regenerate the file with `weightcanary watch`", path, s.Version, SchemaVersion)
	}
	return &s, nil
}
