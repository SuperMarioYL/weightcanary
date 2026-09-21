// Package report renders the per-model availability report from a snapshot.
package report

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/SuperMarioYL/weightcanary/internal/state"
)

// Render writes the per-model availability report. With files=true it also
// lists every observed file per registry.
func Render(w io.Writer, snap *state.Snapshot, files bool) error {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "weightcanary report — %d model(s) — generated %s\n\n", len(snap.Models), snap.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"))
	for i, rec := range snap.Models {
		if i > 0 {
			fmt.Fprintln(tw)
		}
		fmt.Fprintf(tw, "%s\n", rec.ModelID)
		renderRegistry(tw, "huggingface", rec.HF, nil)
		keys := sortedMirrorKeys(rec)
		for _, k := range keys {
			renderRegistry(tw, k, rec.Mirrors[k], &rec.HF)
		}
		if files {
			renderFiles(tw, "huggingface", rec.HF)
			for _, k := range keys {
				renderFiles(tw, k, rec.Mirrors[k])
			}
		}
	}
	return tw.Flush()
}

func sortedMirrorKeys(rec state.AvailabilityRecord) []string {
	keys := make([]string, 0, len(rec.Mirrors))
	for k := range rec.Mirrors {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func renderRegistry(tw *tabwriter.Writer, name string, st state.RegistryState, hf *state.RegistryState) {
	status, detail := "ok", ""
	switch {
	case st.Error != "":
		status, detail = "error", st.Error
	case !st.Present:
		status, detail = "absent", "repo missing or private"
	case name == "huggingface":
		detail = "commit " + shortSha(st.CommitSha)
	case hf == nil:
		detail = ""
	default:
		detail = mirrorDetail(*hf, st)
	}
	if st.Partial && st.Error == "" {
		if detail != "" {
			detail += "; "
		}
		detail += "partial listing — some LFS pointers undecoded (rate limit)"
	}
	fmt.Fprintf(tw, "  %s\t%s\t%d files\t%s\t%s\n", name, status, len(st.Files), humanSize(totalSize(st)), detail)
}

// mirrorDetail describes a mirror's sync state against the Hugging Face source.
func mirrorDetail(hf, mirror state.RegistryState) string {
	if hf.Error != "" || !hf.Present {
		return "sync unknown: source unavailable"
	}
	s := CompareMirror(hf, mirror)
	if s.InSync() {
		if s.Extra > 0 {
			return fmt.Sprintf("in sync (+%d extra on mirror)", s.Extra)
		}
		return "in sync"
	}
	var parts []string
	if s.Missing > 0 {
		parts = append(parts, fmt.Sprintf("%d missing (%d weights)", s.Missing, s.MissingWeights))
	}
	if s.Changed > 0 {
		parts = append(parts, fmt.Sprintf("%d changed", s.Changed))
	}
	detail := "divergent: " + strings.Join(parts, ", ")
	if s.Extra > 0 {
		detail += fmt.Sprintf(", +%d extra on mirror", s.Extra)
	}
	return detail
}

func renderFiles(tw *tabwriter.Writer, name string, st state.RegistryState) {
	if st.Error != "" || !st.Present {
		fmt.Fprintf(tw, "    %s: not listed (%s)\n", name, statusWord(st))
		return
	}
	fmt.Fprintf(tw, "    %s: %d files\n", name, len(st.Files))
	paths := make([]string, 0, len(st.Files))
	for p := range st.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		f := st.Files[p]
		sha := f.LfsSha256
		if len(sha) > 12 {
			sha = sha[:12]
		}
		fmt.Fprintf(tw, "      %s\t%s\t%s\n", p, humanSize(f.Size), sha)
	}
}

func statusWord(st state.RegistryState) string {
	if st.Error != "" {
		return "error"
	}
	return "absent"
}

func totalSize(st state.RegistryState) int64 {
	var n int64
	for _, f := range st.Files {
		n += f.Size
	}
	return n
}

func shortSha(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// MirrorSync is the cross-registry divergence of one mirror against the
// Hugging Face source listing.
type MirrorSync struct {
	Missing        int // source files absent from the mirror
	MissingWeights int // of which Git LFS weight files
	Changed        int // present on both but size or sha256 differs
	Extra          int // mirror-only files
}

func (s MirrorSync) InSync() bool { return s.Missing == 0 && s.Changed == 0 }

// CompareMirror diffs a mirror listing against the source listing. LFS weight
// files are compared by sha256 and size; plain files by size only, because
// the registries expose different hash conventions for them.
func CompareMirror(hf, mirror state.RegistryState) MirrorSync {
	var s MirrorSync
	for path, hfFile := range hf.Files {
		mf, ok := mirror.Files[path]
		if !ok {
			s.Missing++
			if hfFile.LfsSha256 != "" {
				s.MissingWeights++
			}
			continue
		}
		if !sameContent(hfFile, mf) {
			s.Changed++
		}
	}
	for path := range mirror.Files {
		if _, ok := hf.Files[path]; !ok {
			s.Extra++
		}
	}
	return s
}

// sameContent compares one source file against its mirror copy. The sha256 is
// only comparable when both sides expose it: some mirrors store big plain
// files (tokenizer.json, the weight index) as Git LFS objects while the
// source keeps them as plain git files — same bytes, different plumbing. In
// that case, and for plain files generally, size is the honest signal.
func sameContent(a, b state.FileState) bool {
	if a.LfsSha256 != "" && b.LfsSha256 != "" {
		return a.LfsSha256 == b.LfsSha256 && a.Size == b.Size
	}
	return a.Size == b.Size
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
