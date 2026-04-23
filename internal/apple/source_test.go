package apple

import (
	stderrors "errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/mariiatuzovska/vo2-bot/internal/errs"
)

func TestValidateName(t *testing.T) {
	bad := []string{"", "../etc/passwd", "/etc/passwd", "sub/dir.zip", `dir\file.zip`, "..\\foo.zip"}
	for _, n := range bad {
		t.Run("reject_"+n, func(t *testing.T) {
			err := validateName(n)
			if err == nil {
				t.Fatalf("expected error for %q", n)
			}
			var apiErr *errs.Error
			if !stderrors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest {
				t.Fatalf("expected 400, got %T %v", err, err)
			}
		})
	}
	good := []string{"HealthAutoExport_20260423132853.zip", "file.txt"}
	for _, n := range good {
		t.Run("accept_"+n, func(t *testing.T) {
			if err := validateName(n); err != nil {
				t.Fatalf("expected nil for %q, got %v", n, err)
			}
		})
	}
}

func TestLocalSourceRead(t *testing.T) {
	dir := t.TempDir()
	want := []byte("payload")
	if err := os.WriteFile(filepath.Join(dir, "file.zip"), want, 0o600); err != nil {
		t.Fatal(err)
	}
	src := &LocalSource{BaseDir: dir}

	got, err := src.Read("file.zip")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLocalSourceReadMissing(t *testing.T) {
	src := &LocalSource{BaseDir: t.TempDir()}
	_, err := src.Read("nope.zip")
	var apiErr *errs.Error
	if !stderrors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		t.Fatalf("expected 404, got %T %v", err, err)
	}
}

func TestLocalSourceLatest(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"HealthAutoExport_20260101000000.zip",
		"HealthAutoExport_20260423132853.zip", // newest
		"HealthAutoExport_20260301000000.zip",
		"unrelated.txt",
		"HealthAutoExport_notes.txt",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "HealthAutoExport_subdir.zip"), 0o755); err != nil {
		t.Fatal(err)
	}

	src := &LocalSource{BaseDir: dir}
	got, err := src.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got != "HealthAutoExport_20260423132853.zip" {
		t.Fatalf("got %q, want newest archive", got)
	}
}

func TestLocalSourceLatestEmpty(t *testing.T) {
	src := &LocalSource{BaseDir: t.TempDir()}
	_, err := src.Latest()
	var apiErr *errs.Error
	if !stderrors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		t.Fatalf("expected 404, got %T %v", err, err)
	}
}
