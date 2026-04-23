package apple

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTimeUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "hae format with negative offset",
			input: `"2026-04-22 21:17:30 -0400"`,
			want:  time.Date(2026, 4, 22, 21, 17, 30, 0, time.FixedZone("", -4*3600)),
		},
		{
			name:  "hae format with utc offset",
			input: `"2026-04-22 21:17:30 +0000"`,
			want:  time.Date(2026, 4, 22, 21, 17, 30, 0, time.UTC),
		},
		{
			name:  "empty string is zero value",
			input: `""`,
		},
		{
			name:  "json null is zero value",
			input: `null`,
		},
		{
			name:    "wrong format",
			input:   `"2026-04-22T21:17:30Z"`,
			wantErr: true,
		},
		{
			name:    "garbage",
			input:   `"not a time"`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got Time
			err := json.Unmarshal([]byte(tc.input), &got)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; parsed=%v", got.Time)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Time.Equal(tc.want) {
				t.Fatalf("got %v, want %v", got.Time, tc.want)
			}
		})
	}
}
