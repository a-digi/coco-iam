package deleted

import (
	"fmt"
	"os"
	"path/filepath"
)

// ArchiveMigrationReport summarises MigrateLegacyArchiveDir's work so
// main.go can surface "n archives moved" in the boot log. Moved lists
// the source entry names that were relocated; Skipped carries ones
// whose destination already existed (idempotent guard).
type ArchiveMigrationReport struct {
	Moved    []string
	Skipped  []string
	Failures map[string]error
}

// MigrateLegacyArchiveDir moves every entry out of `legacyRoot` and
// into `newRoot`, preserving original subfolder names. `skip` is a
// set of entry names to leave in place — used when `newRoot` is a
// subdirectory of `legacyRoot` and we must not try to move the
// destination into itself.
//
// Idempotent: a missing legacyRoot returns an empty report + nil err;
// an already-present destination entry is skipped (never overwritten)
// so a partial previous run doesn't clobber history. The legacy root
// is removed once it's empty and not the same directory as newRoot.
//
// Safe to call on every boot — its first-run cost is constant-time
// when the legacy folder is gone.
func MigrateLegacyArchiveDir(legacyRoot, newRoot string, skip ...string) (ArchiveMigrationReport, error) {
	report := ArchiveMigrationReport{Failures: map[string]error{}}

	info, err := os.Stat(legacyRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return report, nil
		}
		return report, fmt.Errorf("stat %s: %w", legacyRoot, err)
	}
	if !info.IsDir() {
		return report, fmt.Errorf("%s is not a directory", legacyRoot)
	}

	if err := os.MkdirAll(newRoot, 0o755); err != nil {
		return report, fmt.Errorf("mkdir %s: %w", newRoot, err)
	}

	skipSet := make(map[string]struct{}, len(skip))
	for _, s := range skip {
		skipSet[s] = struct{}{}
	}

	entries, err := os.ReadDir(legacyRoot)
	if err != nil {
		return report, fmt.Errorf("read %s: %w", legacyRoot, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if _, ok := skipSet[name]; ok {
			continue
		}
		src := filepath.Join(legacyRoot, name)
		dst := filepath.Join(newRoot, name)
		// If src and dst resolve to the same location (same root with
		// newRoot == legacyRoot, or a no-op), skip silently.
		absSrc, _ := filepath.Abs(src)
		absDst, _ := filepath.Abs(dst)
		if absSrc == absDst {
			continue
		}
		if _, err := os.Stat(dst); err == nil {
			// Don't overwrite — leave the legacy copy in place so the
			// admin can reconcile manually.
			report.Skipped = append(report.Skipped, name)
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			report.Failures[name] = err
			continue
		}
		report.Moved = append(report.Moved, name)
	}

	// Remove the legacy root when fully drained AND it's distinct
	// from newRoot (removing the destination would be catastrophic).
	absLegacy, _ := filepath.Abs(legacyRoot)
	absNew, _ := filepath.Abs(newRoot)
	if absLegacy != absNew {
		remaining, rerr := os.ReadDir(legacyRoot)
		if rerr == nil && len(remaining) == 0 {
			_ = os.Remove(legacyRoot)
		}
	}
	return report, nil
}
