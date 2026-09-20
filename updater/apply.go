package updater

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ApplyInstall writes a verified artifact to destPath atomically.
// Callers must obtain plan from PlanInstall (invariant 9); digest is
// re-checked here as defense in depth before any filesystem mutation.
func ApplyInstall(plan InstallPlan, artifact []byte, destPath string) error {
	if plan.AppID == "" || plan.Version == "" || plan.SHA256 == "" {
		return errors.New("install plan is incomplete")
	}
	if destPath == "" {
		return errors.New("destination path is required")
	}
	if err := VerifyArtifactDigest(Manifest{SHA256: plan.SHA256}, artifact); err != nil {
		return err
	}

	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create destination dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".vitra-update-*")
	if err != nil {
		return fmt.Errorf("create temp artifact: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(artifact); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp artifact: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp artifact: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp artifact: %w", err)
	}
	if err := os.Rename(tmpName, destPath); err != nil {
		return fmt.Errorf("atomic replace: %w", err)
	}
	cleanup = false
	return nil
}
