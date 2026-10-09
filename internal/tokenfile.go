package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// tokenSidecar is written next to the token file as <path>.json. It never
// contains the token itself.
type tokenSidecar struct {
	ExpiresAt           time.Time         `json:"expires_at"`
	Permissions         map[string]string `json:"permissions,omitempty"`
	RepositorySelection string            `json:"repository_selection,omitempty"`
	Repositories        []string          `json:"repositories,omitempty"`
}

// writeTokenFiles writes the token (plus a trailing newline) to path and its
// metadata to path.json, each atomically.
func writeTokenFiles(path string, token *mintedToken) error {
	if err := writeFileAtomic(path, []byte(token.GetToken()+"\n")); err != nil {
		return fmt.Errorf("unable to write token file: %w", err)
	}

	sidecar := tokenSidecar{}
	if token.details != nil {
		sidecar.ExpiresAt = token.details.ExpiresAt
		sidecar.Permissions = token.details.Permissions
		sidecar.RepositorySelection = token.details.RepositorySelection
		for _, r := range token.details.Repositories {
			name := r.FullName
			if name == "" {
				name = r.Name
			}
			sidecar.Repositories = append(sidecar.Repositories, name)
		}
	}
	b, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal token metadata: %w", err)
	}
	if err := writeFileAtomic(path+".json", append(b, '\n')); err != nil {
		return fmt.Errorf("unable to write token metadata file: %w", err)
	}

	return nil
}

// writeFileAtomic writes data to a 0600 temp file in the target directory,
// syncs it and renames it over path, so readers see either the old or the
// new content, never a partial write. The directory is created with 0700.
//
// A bind mount of the directory sees the rename; a bind mount of the file
// itself keeps pointing at the old inode.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName) // no-op after a successful rename
	}()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}

	// Persist the rename itself; best effort, not supported everywhere.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}

	return nil
}
