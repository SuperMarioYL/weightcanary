// Package diff will compare consecutive snapshots of the availability ledger
// to detect takedowns (a file present in run N-1 and absent in run N) and
// mirror-divergence changes, with the first run acting as the baseline and
// dedupe collapsing one change into one alert.
//
// v0.1 milestone status: m1 ships the snapshot + report + ledger; this
// package is the m2 seam. The types below fix the contract; the engine lands
// with the m2 change-detection milestone.
package diff

import (
	"errors"

	"github.com/SuperMarioYL/weightcanary/internal/state"
)

// ErrNotImplemented marks the m2 seam: the run-to-run diff engine is not part
// of the m1 milestone.
var ErrNotImplemented = errors.New("diff: run-to-run diffing is not implemented yet (m2 milestone)")

// Change is one detected difference between two snapshots. Kind is one of
// "takedown" (present → absent), "restore" (absent → present) or
// "divergence" (a mirror's file set or LFS hashes moved relative to the
// source).
type Change struct {
	ModelID    string
	Kind       string
	Registry   string // "huggingface" or a mirror key
	File       string // empty for record-level changes
	Detail     string
	DetectedAt string // RFC3339
}

// Diff compares two snapshots run-over-run. It is intentionally not
// implemented in v0.1: wiring it up (baseline handling and alert dedupe) is
// the m2 milestone, together with the notify package it feeds.
func Diff(prev, curr *state.Snapshot) ([]Change, error) {
	return nil, ErrNotImplemented
}
