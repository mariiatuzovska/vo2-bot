package apple

import (
	stderrors "errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/mariiatuzovska/vo2-bot/internal/errs"
)

func TestParseInstantWindow(t *testing.T) {
	from := "2026-04-22T00:00:00-04:00"
	to := "2026-04-23T00:00:00-04:00"

	gotFrom, gotTo, err := parseInstantWindow(from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantFrom, _ := time.Parse(time.RFC3339, from)
	wantTo, _ := time.Parse(time.RFC3339, to)
	if !gotFrom.Equal(wantFrom) || !gotTo.Equal(wantTo) {
		t.Fatalf("got [%v,%v) want [%v,%v)", gotFrom, gotTo, wantFrom, wantTo)
	}
}

func TestParseInstantWindowErrors(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
	}{
		{"both missing", "", ""},
		{"missing from", "", "2026-04-23T00:00:00Z"},
		{"missing to", "2026-04-22T00:00:00Z", ""},
		{"bare date from", "2026-04-22", "2026-04-23T00:00:00Z"},
		{"bare date to", "2026-04-22T00:00:00Z", "2026-04-23"},
		{"to equals from", "2026-04-22T00:00:00Z", "2026-04-22T00:00:00Z"},
		{"to before from", "2026-04-23T00:00:00Z", "2026-04-22T00:00:00Z"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseInstantWindow(tc.from, tc.to)
			if err == nil {
				t.Fatalf("expected error")
			}
			var apiErr *errs.Error
			if !stderrors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest {
				t.Fatalf("expected 400 errs.Error, got %T %v", err, err)
			}
		})
	}
}

func TestParseCSV(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", []string{}},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b ,c ", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
		{",", []string{}},
		{"Outdoor Run,Pool Swim", []string{"Outdoor Run", "Pool Swim"}},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := parseCSV(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestParseBool(t *testing.T) {
	tests := map[string]bool{
		"":         false,
		"true":     true,
		"false":    false,
		"1":        true,
		"0":        false,
		"nonsense": false,
	}
	for in, want := range tests {
		t.Run(in, func(t *testing.T) {
			if got := parseBool(in); got != want {
				t.Fatalf("parseBool(%q)=%v want %v", in, got, want)
			}
		})
	}
}

func TestParseLimit(t *testing.T) {
	t.Run("empty -> 0", func(t *testing.T) {
		n, err := parseLimit("")
		if err != nil || n != 0 {
			t.Fatalf("got (%d,%v) want (0,nil)", n, err)
		}
	})
	t.Run("valid", func(t *testing.T) {
		n, err := parseLimit("42")
		if err != nil || n != 42 {
			t.Fatalf("got (%d,%v) want (42,nil)", n, err)
		}
	})
	t.Run("non-numeric", func(t *testing.T) {
		_, err := parseLimit("ten")
		assertBadRequest(t, err)
	})
	t.Run("negative", func(t *testing.T) {
		_, err := parseLimit("-1")
		assertBadRequest(t, err)
	})
}

func assertBadRequest(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error")
	}
	var apiErr *errs.Error
	if !stderrors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest {
		t.Fatalf("expected 400 errs.Error, got %T %v", err, err)
	}
}
