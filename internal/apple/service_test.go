package apple

import (
	"context"
	stderrors "errors"
	"net/http"
	"testing"

	"github.com/mariiatuzovska/vo2-bot/internal/errs"
)

// fakeSource lets Import error paths run without filesystem or DB.
type fakeSource struct {
	data      []byte
	readErr   error
	latest    string
	latestErr error
}

func (f *fakeSource) Read(string) ([]byte, error) { return f.data, f.readErr }
func (f *fakeSource) Latest() (string, error)     { return f.latest, f.latestErr }

func assertStatus(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with status %d, got nil", want)
	}
	var apiErr *errs.Error
	if !stderrors.As(err, &apiErr) {
		t.Fatalf("expected *errs.Error, got %T: %v", err, err)
	}
	if apiErr.Status != want {
		t.Fatalf("expected status %d, got %d (%v)", want, apiErr.Status, apiErr)
	}
}

func TestImport_SourceMissing(t *testing.T) {
	svc := &Service{Source: &fakeSource{}}
	_, err := svc.Import(context.Background(), ImportRequest{})
	assertStatus(t, err, http.StatusBadRequest)
}

func TestImport_SourceGCS_NotImplemented(t *testing.T) {
	svc := &Service{Source: &fakeSource{}}
	_, err := svc.Import(context.Background(), ImportRequest{Source: "gcs", Name: "x.zip"})
	assertStatus(t, err, http.StatusBadRequest)
}

func TestImport_SourceUnknown(t *testing.T) {
	svc := &Service{Source: &fakeSource{}}
	_, err := svc.Import(context.Background(), ImportRequest{Source: "ftp", Name: "x.zip"})
	assertStatus(t, err, http.StatusBadRequest)
}

func TestImport_LocalLatestErrPropagates(t *testing.T) {
	wantErr := errs.NewNotFound("nothing here")
	svc := &Service{Source: &fakeSource{latestErr: wantErr}}
	_, err := svc.Import(context.Background(), ImportRequest{Source: "local"})
	if !stderrors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
}

func TestImport_ReadErrPropagates(t *testing.T) {
	wantErr := errs.NewNotFound("missing")
	svc := &Service{Source: &fakeSource{readErr: wantErr}}
	_, err := svc.Import(context.Background(), ImportRequest{Source: "local", Name: "x.zip"})
	if !stderrors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
}

func TestImport_InvalidZip_422(t *testing.T) {
	svc := &Service{Source: &fakeSource{data: []byte("not a zip")}}
	_, err := svc.Import(context.Background(), ImportRequest{Source: "local", Name: "x.zip"})
	assertStatus(t, err, http.StatusUnprocessableEntity)
}

func TestImport_MissingJSONInZip_422(t *testing.T) {
	zipBytes := makeArchive(t, map[string]string{"workout.gpx": "<gpx/>"})
	svc := &Service{Source: &fakeSource{data: zipBytes}}
	_, err := svc.Import(context.Background(), ImportRequest{Source: "local", Name: "x.zip"})
	assertStatus(t, err, http.StatusUnprocessableEntity)
}

