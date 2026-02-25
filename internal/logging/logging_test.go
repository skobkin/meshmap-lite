package logging

import (
	"log/slog"
	"testing"
	"time"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		wantErr bool
		want    slog.Level
	}{
		{name: "debug", input: "debug", want: slog.LevelDebug},
		{name: "info", input: "info", want: slog.LevelInfo},
		{name: "empty defaults to info", input: "", want: slog.LevelInfo},
		{name: "warn", input: "warn", want: slog.LevelWarn},
		{name: "warning alias", input: "warning", want: slog.LevelWarn},
		{name: "error", input: "error", want: slog.LevelError},
		{name: "invalid", input: "verbose", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseLevel(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q", tc.input)
				}

				return
			}
			if err != nil {
				t.Fatalf("parseLevel(%q) unexpected error: %v", tc.input, err)
			}

			gotLevel := got.Level()
			if gotLevel != tc.want {
				t.Fatalf("parseLevel(%q) = %v, want %v", tc.input, gotLevel, tc.want)
			}
		})
	}
}

func TestReplaceAttrsTimeFormatting(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 2, 26, 1, 2, 3, 456000000, time.UTC)
	attr := slog.Time(slog.TimeKey, ts)

	got := replaceAttrs(nil, attr)
	if got.Key != slog.TimeKey {
		t.Fatalf("replaceAttrs key = %q, want %q", got.Key, slog.TimeKey)
	}
	if got.Value.Kind() != slog.KindString {
		t.Fatalf("replaceAttrs value kind = %v, want %v", got.Value.Kind(), slog.KindString)
	}

	const want = "2026-02-26 01:02:03.456"
	if got.Value.String() != want {
		t.Fatalf("replaceAttrs time = %q, want %q", got.Value.String(), want)
	}
}
