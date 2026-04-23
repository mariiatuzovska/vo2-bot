package apple

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type parsedArchive struct {
	sha256   string
	workouts []json.RawMessage
	metrics  []Metric
}

// extract reads a zip archive, locates the single HealthAutoExport-*.json
// file, and parses its top-level data.workouts + data.metrics sections.
// workouts are returned as raw JSON so callers can both decode them into
// typed Workout values AND persist the untouched blob in apple_workouts.payload.
func extract(archive []byte) (*parsedArchive, error) {
	sum := sha256.Sum256(archive)
	hash := hex.EncodeToString(sum[:])

	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	var jsonFile *zip.File
	for _, f := range zr.File {
		base := filepath.Base(f.Name)
		if strings.HasPrefix(base, "HealthAutoExport-") && strings.HasSuffix(base, ".json") {
			jsonFile = f
			break
		}
	}
	if jsonFile == nil {
		return nil, fmt.Errorf("no HealthAutoExport-*.json in zip")
	}

	jf, err := jsonFile.Open()
	if err != nil {
		return nil, fmt.Errorf("open json: %w", err)
	}
	defer jf.Close()

	body, err := io.ReadAll(jf)
	if err != nil {
		return nil, fmt.Errorf("read json: %w", err)
	}

	var wire struct {
		Data struct {
			Workouts []json.RawMessage `json:"workouts"`
			Metrics  []Metric          `json:"metrics"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	return &parsedArchive{
		sha256:   hash,
		workouts: wire.Data.Workouts,
		metrics:  wire.Data.Metrics,
	}, nil
}
