package diff

import (
	"errors"
	"testing"

	"github.com/SuperMarioYL/weightcanary/internal/state"
)

func TestDiffIsTheM2Seam(t *testing.T) {
	_, err := Diff(nil, &state.Snapshot{})
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Diff should surface the m2 seam, got %v", err)
	}
}
