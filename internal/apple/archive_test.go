package apple

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

const sampleHAEJSON = `{
  "data": {
    "workouts": [
      {
        "id": "7A71FBAD-1111-2222-3333-444455556666",
        "name": "Outdoor Run",
        "start": "2026-04-22 21:17:30 -0400",
        "end":   "2026-04-22 21:40:06 -0400",
        "duration": 1356,
        "isIndoor": false,
        "location": "Outdoor",
        "distance":      { "qty": 4.21, "units": "km" },
        "avgHeartRate":  { "qty": 155.9, "units": "bpm" }
      }
    ],
    "metrics": [
      {
        "name":  "vo2_max",
        "units": "ml/(kg·min)",
        "data":  [ { "date": "2026-04-22 00:00:00 -0400", "qty": 47.2, "source": "Apple Watch" } ]
      },
      {
        "name":  "heart_rate_variability",
        "units": "ms",
        "data":  [ { "date": "2026-04-22 00:00:00 -0400", "qty": 65.1, "source": "Apple Watch" } ]
      }
    ]
  }
}`

func makeArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractHappy(t *testing.T) {
	archive := makeArchive(t, map[string]string{
		"HealthAutoExport-2026-04.json":     sampleHAEJSON,
		"workout-7A71FBAD.gpx":              "<gpx></gpx>",
	})

	parsed, err := extract(archive)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	wantSum := sha256.Sum256(archive)
	if parsed.sha256 != hex.EncodeToString(wantSum[:]) {
		t.Fatalf("sha256 mismatch: got %s", parsed.sha256)
	}

	if len(parsed.workouts) != 1 {
		t.Fatalf("workouts: got %d, want 1", len(parsed.workouts))
	}

	var w Workout
	if err := json.Unmarshal(parsed.workouts[0], &w); err != nil {
		t.Fatalf("unmarshal workout: %v", err)
	}
	if w.ID != "7A71FBAD-1111-2222-3333-444455556666" || w.Name != "Outdoor Run" {
		t.Fatalf("workout: %+v", w)
	}
	if w.Distance == nil || w.Distance.Qty != 4.21 {
		t.Fatalf("distance: %+v", w.Distance)
	}

	if len(parsed.metrics) != 2 {
		t.Fatalf("metrics: got %d, want 2", len(parsed.metrics))
	}
	gotNames := []string{parsed.metrics[0].Name, parsed.metrics[1].Name}
	wantNames := map[string]bool{"vo2_max": true, "heart_rate_variability": true}
	for _, n := range gotNames {
		if !wantNames[n] {
			t.Fatalf("unexpected metric %q", n)
		}
	}
}

func TestExtractDeterministicSha(t *testing.T) {
	archive := makeArchive(t, map[string]string{"HealthAutoExport-x.json": sampleHAEJSON})
	a, err := extract(archive)
	if err != nil {
		t.Fatal(err)
	}
	b, err := extract(archive)
	if err != nil {
		t.Fatal(err)
	}
	if a.sha256 != b.sha256 {
		t.Fatalf("sha256 not deterministic: %s vs %s", a.sha256, b.sha256)
	}
}

func TestExtractMissingJSON(t *testing.T) {
	archive := makeArchive(t, map[string]string{"workout.gpx": "<gpx></gpx>"})
	_, err := extract(archive)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no HealthAutoExport") {
		t.Fatalf("got %v", err)
	}
}

func TestExtractInvalidZip(t *testing.T) {
	_, err := extract([]byte("not a zip"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractInvalidJSON(t *testing.T) {
	archive := makeArchive(t, map[string]string{"HealthAutoExport-x.json": "{ not json"})
	_, err := extract(archive)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "parse json") {
		t.Fatalf("got %v", err)
	}
}
