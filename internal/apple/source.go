package apple

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mariiatuzovska/vo2-bot/internal/errs"
)

// Source resolves HAE archives. The local implementation reads from a
// configured base directory; a future GCS implementation will satisfy the
// same interface.
type Source interface {
	// Read returns the raw bytes of the named archive.
	Read(name string) ([]byte, error)
	// Latest returns the filename of the most recent HealthAutoExport archive.
	Latest() (string, error)
}

type LocalSource struct {
	BaseDir string
}

func (s *LocalSource) Read(name string) ([]byte, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	path := filepath.Join(s.BaseDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errs.NewNotFound("").With("name", name)
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// Latest returns the newest HealthAutoExport_*.zip filename in BaseDir.
// HAE embeds a yyyymmddhhmmss stamp in the name so lexicographic max == newest.
func (s *LocalSource) Latest() (string, error) {
	entries, err := os.ReadDir(s.BaseDir)
	if err != nil {
		return "", fmt.Errorf("read dir %s: %w", s.BaseDir, err)
	}
	var latest string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasPrefix(n, "HealthAutoExport_") || !strings.HasSuffix(n, ".zip") {
			continue
		}
		if n > latest {
			latest = n
		}
	}
	if latest == "" {
		return "", errs.NewNotFound("no HealthAutoExport_*.zip in %s", s.BaseDir)
	}
	return latest, nil
}

func validateName(name string) error {
	if name == "" {
		return errs.NewBadRequest("name is required")
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return errs.NewBadRequest("name must be a plain filename, not a path")
	}
	if filepath.IsAbs(name) {
		return errs.NewBadRequest("name must be a plain filename, not an absolute path")
	}
	return nil
}
