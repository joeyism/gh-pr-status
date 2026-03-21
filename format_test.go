package main

import (
	"strings"
	"testing"
)

func TestFormatCIStatus(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   string
	}{
		{name: "success", status: "SUCCESS", want: "pass"},
		{name: "failure", status: "FAILURE", want: "fail"},
		{name: "pending", status: "PENDING", want: "run"},
		{name: "error", status: "ERROR", want: "err"},
		{name: "none", status: "", want: "--"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := formatCIStatus(tc.status)
			if !strings.Contains(out, tc.want) {
				t.Fatalf("formatCIStatus(%q) = %q, want substring %q", tc.status, out, tc.want)
			}
		})
	}
}

func TestFormatReviewStatus(t *testing.T) {
	cases := []struct {
		name     string
		decision string
		want     string
	}{
		{name: "approved", decision: "APPROVED", want: "approved"},
		{name: "changes", decision: "CHANGES_REQUESTED", want: "changes"},
		{name: "required", decision: "REVIEW_REQUIRED", want: "pending"},
		{name: "none", decision: "", want: "none"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := formatReviewStatus(tc.decision)
			if !strings.Contains(out, tc.want) {
				t.Fatalf("formatReviewStatus(%q) = %q, want substring %q", tc.decision, out, tc.want)
			}
		})
	}
}

func TestFormatComments(t *testing.T) {
	cases := []struct {
		name         string
		total        int
		unresolved   int
		totalThreads int
		want         []string
		dontWant     []string
	}{
		{name: "empty", total: 0, unresolved: 0, totalThreads: 0, want: []string{"💬 0"}, dontWant: []string{"unresolved"}},
		{name: "comments no threads", total: 4, unresolved: 0, totalThreads: 0, want: []string{"💬 4"}, dontWant: []string{"unresolved"}},
		{name: "resolved threads", total: 22, unresolved: 0, totalThreads: 9, want: []string{"💬 22", "0 unresolved"}},
		{name: "has unresolved", total: 22, unresolved: 3, totalThreads: 9, want: []string{"💬 22", "3 unresolved"}},
		{name: "truncated", total: 22, unresolved: -1, totalThreads: 9, want: []string{"💬 22", "? unresolved"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := formatComments(tc.total, tc.unresolved, tc.totalThreads)
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Fatalf("formatComments(...) = %q, want substring %q", out, want)
				}
			}
			for _, dont := range tc.dontWant {
				if strings.Contains(out, dont) {
					t.Fatalf("formatComments(...) = %q, unexpected substring %q", out, dont)
				}
			}
		})
	}
}

func TestFormatMergeable(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   string
	}{
		{name: "mergeable", status: "MERGEABLE", want: "ready"},
		{name: "conflicting", status: "CONFLICTING", want: "conflict"},
		{name: "unknown", status: "UNKNOWN", want: "unknown"},
		{name: "none", status: "", want: "unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := formatMergeable(tc.status)
			if !strings.Contains(out, tc.want) {
				t.Fatalf("formatMergeable(%q) = %q, want substring %q", tc.status, out, tc.want)
			}
		})
	}
}

func TestFormatCheckRun(t *testing.T) {
	cases := []struct {
		name string
		cr   CheckRun
		want []string
	}{
		{name: "passed", cr: CheckRun{Name: "build", Status: "COMPLETED", Conclusion: "SUCCESS"}, want: []string{"build", "passed", "✓"}},
		{name: "failed", cr: CheckRun{Name: "lint", Status: "COMPLETED", Conclusion: "FAILURE"}, want: []string{"lint", "failed", "✗"}},
		{name: "in progress", cr: CheckRun{Name: "test", Status: "IN_PROGRESS", Conclusion: ""}, want: []string{"test", "in_progress"}},
		{name: "skipped", cr: CheckRun{Name: "deploy", Status: "COMPLETED", Conclusion: "SKIPPED"}, want: []string{"deploy", "skipped", "→"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := formatCheckRun(tc.cr)
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Fatalf("formatCheckRun(%+v) = %q, want substring %q", tc.cr, out, want)
				}
			}
		})
	}
}
