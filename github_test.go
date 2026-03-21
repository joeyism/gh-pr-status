package main

import (
	"testing"

	"github.com/shurcooL/githubv4"
)

func nodes(resolved ...bool) []struct{ IsResolved githubv4.Boolean } {
	var ns []struct{ IsResolved githubv4.Boolean }
	for _, r := range resolved {
		ns = append(ns, struct{ IsResolved githubv4.Boolean }{IsResolved: githubv4.Boolean(r)})
	}
	return ns
}

func TestCountUnresolved(t *testing.T) {
	cases := []struct {
		name       string
		nodes      []struct{ IsResolved githubv4.Boolean }
		totalCount int
		want       int
	}{
		{name: "empty", nodes: nodes(), totalCount: 0, want: 0},
		{name: "all resolved", nodes: nodes(true, true, true), totalCount: 3, want: 0},
		{name: "mixed", nodes: nodes(true, false, true), totalCount: 3, want: 1},
		{name: "all unresolved", nodes: nodes(false, false), totalCount: 2, want: 2},
		{name: "truncated resolved", nodes: nodes(true, true), totalCount: 5, want: -1},
		{name: "truncated unresolved", nodes: nodes(true, false), totalCount: 5, want: -1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := countUnresolved(tc.nodes, tc.totalCount)
			if got != tc.want {
				t.Fatalf("countUnresolved(nodes=%v, totalCount=%d) = %d, want %d", tc.nodes, tc.totalCount, got, tc.want)
			}
		})
	}
}
