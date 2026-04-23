package apple

import (
	"testing"
	"time"
)

func TestToTimestamptz(t *testing.T) {
	t.Run("nil -> invalid", func(t *testing.T) {
		got := toTimestamptz(nil)
		if got.Valid {
			t.Fatalf("expected invalid, got %+v", got)
		}
	})
	t.Run("non-nil -> valid", func(t *testing.T) {
		now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
		got := toTimestamptz(&now)
		if !got.Valid || !got.Time.Equal(now) {
			t.Fatalf("got %+v, want valid=%v", got, now)
		}
	})
}
