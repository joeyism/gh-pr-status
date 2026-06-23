package main

import (
	"os"
	"strings"
	"testing"
)

func TestConfigSortConfig(t *testing.T) {
	cases := []struct {
		name        string
		cfg         Config
		wantField   sortField
		wantDir     sortDir
		wantWarnBy  string
		wantWarnDir string
	}{
		{
			name:      "empty returns defaults silently",
			cfg:       Config{},
			wantField: defaultSortField,
			wantDir:   defaultSortDir,
		},
		{
			name:      "valid preserved",
			cfg:       Config{SortBy: "repo", SortDir: "asc"},
			wantField: sortFieldRepo,
			wantDir:   sortAsc,
		},
		{
			name:        "invalid sort_by falls back to default with warning",
			cfg:         Config{SortBy: "bogus", SortDir: "asc"},
			wantField:   defaultSortField,
			wantDir:     sortAsc,
			wantWarnBy:  "bogus",
		},
		{
			name:        "invalid sort_dir falls back to default with warning",
			cfg:         Config{SortBy: "repo", SortDir: "sideways"},
			wantField:   sortFieldRepo,
			wantDir:     defaultSortDir,
			wantWarnDir: "sideways",
		},
		{
			name:        "both invalid falls back to both defaults with two warnings",
			cfg:         Config{SortBy: "bogus", SortDir: "sideways"},
			wantField:   defaultSortField,
			wantDir:     defaultSortDir,
			wantWarnBy:  "bogus",
			wantWarnDir: "sideways",
		},
		{
			name:      "case and whitespace normalized",
			cfg:       Config{SortBy: "  REPO  ", SortDir: "DESC"},
			wantField: sortFieldRepo,
			wantDir:   sortDesc,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			warnings := captureStderr(t, func() {
				gotField, gotDir := tc.cfg.SortConfig()
				if gotField != tc.wantField {
					t.Fatalf("field = %q, want %q", gotField, tc.wantField)
				}
				if gotDir != tc.wantDir {
					t.Fatalf("dir = %q, want %q", gotDir, tc.wantDir)
				}
			})
			if tc.wantWarnBy != "" && !strings.Contains(warnings, tc.wantWarnBy) {
				t.Fatalf("expected warning containing %q in stderr, got: %q", tc.wantWarnBy, warnings)
			}
			if tc.wantWarnDir != "" && !strings.Contains(warnings, tc.wantWarnDir) {
				t.Fatalf("expected warning containing %q in stderr, got: %q", tc.wantWarnDir, warnings)
			}
			if tc.wantWarnBy == "" && strings.Contains(warnings, "sort_by") {
				t.Fatalf("unexpected sort_by warning in stderr: %q", warnings)
			}
			if tc.wantWarnDir == "" && strings.Contains(warnings, "sort_dir") {
				t.Fatalf("unexpected sort_dir warning in stderr: %q", warnings)
			}
		})
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		buf := make([]byte, 1024)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				b.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		done <- b.String()
	}()

	fn()
	w.Close()
	return <-done
}
