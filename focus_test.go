package main

import (
	"testing"
)

func TestBuildFocusRows_CollapsedOnlyPRs(t *testing.T) {
	v := &viewState{
		prs:      []PullRequest{{Number: 1}, {Number: 2}},
		expanded: map[int]bool{},
	}
	rows := buildFocusRows(v)
	if len(rows) != 2 {
		t.Fatalf("len=%d want 2", len(rows))
	}
	if rows[0].Kind != focusPR || rows[0].PRNumber != 1 {
		t.Fatalf("row0=%+v", rows[0])
	}
	if rows[1].Kind != focusPR || rows[1].PRNumber != 2 {
		t.Fatalf("row1=%+v", rows[1])
	}
}

func TestBuildFocusRows_ExpandedInsertsChecks(t *testing.T) {
	v := &viewState{
		prs: []PullRequest{
			{Number: 10, CheckRuns: []CheckRun{{Name: "build"}, {Name: "test"}}},
			{Number: 20, CheckRuns: []CheckRun{{Name: "lint"}}},
		},
		expanded: map[int]bool{10: true}, // only first expanded
	}
	rows := buildFocusRows(v)
	// PR10, check build, check test, PR20
	if len(rows) != 4 {
		t.Fatalf("len=%d want 4: %+v", len(rows), rows)
	}
	want := []focusKind{focusPR, focusCheck, focusCheck, focusPR}
	for i, k := range want {
		if rows[i].Kind != k {
			t.Errorf("rows[%d].Kind=%v want %v", i, rows[i].Kind, k)
		}
	}
	if rows[1].CheckIndex != 0 || rows[1].CheckName != "build" || rows[1].PRNumber != 10 {
		t.Errorf("rows[1]=%+v", rows[1])
	}
	if rows[2].CheckIndex != 1 || rows[2].CheckName != "test" || rows[2].PRNumber != 10 {
		t.Errorf("rows[2]=%+v", rows[2])
	}
}

func TestBuildFocusRows_MultiExpand(t *testing.T) {
	v := &viewState{
		prs: []PullRequest{
			{Number: 1, CheckRuns: []CheckRun{{Name: "a"}}},
			{Number: 2, CheckRuns: []CheckRun{{Name: "b"}, {Name: "c"}}},
		},
		expanded: map[int]bool{1: true, 2: true},
	}
	rows := buildFocusRows(v)
	// PR1, a, PR2, b, c
	if len(rows) != 5 {
		t.Fatalf("len=%d want 5: %+v", len(rows), rows)
	}
	want := []struct {
		kind       focusKind
		prNumber   int
		checkName  string
		checkIndex int
	}{
		{focusPR, 1, "", 0},
		{focusCheck, 1, "a", 0},
		{focusPR, 2, "", 0},
		{focusCheck, 2, "b", 0},
		{focusCheck, 2, "c", 1},
	}
	for i, w := range want {
		if rows[i].Kind != w.kind || rows[i].PRNumber != w.prNumber {
			t.Errorf("rows[%d]=%+v want kind=%v pr=%d", i, rows[i], w.kind, w.prNumber)
		}
		if w.kind == focusCheck {
			if rows[i].CheckName != w.checkName || rows[i].CheckIndex != w.checkIndex {
				t.Errorf("rows[%d] check=%+v want name=%q idx=%d", i, rows[i], w.checkName, w.checkIndex)
			}
		}
	}
}

func TestBuildFocusRows_EmptyChecksPlaceholder(t *testing.T) {
	v := &viewState{
		prs:      []PullRequest{{Number: 1, CheckRuns: []CheckRun{}}}, // non-nil empty
		expanded: map[int]bool{1: true},
	}
	rows := buildFocusRows(v)
	if len(rows) != 2 {
		t.Fatalf("len=%d want 2", len(rows))
	}
	if rows[1].Kind != focusPlaceholder || rows[1].Placeholder != placeholderEmpty {
		t.Fatalf("row1=%+v", rows[1])
	}
}

func TestBuildFocusRows_NilChecksLoadingPlaceholder(t *testing.T) {
	// Org-view semantics: nil means not loaded yet.
	v := &viewState{
		prs:      []PullRequest{{Number: 1, CheckRuns: nil}},
		expanded: map[int]bool{1: true},
	}
	rows := buildFocusRows(v)
	if len(rows) != 2 || rows[1].Kind != focusPlaceholder || rows[1].Placeholder != placeholderLoading {
		t.Fatalf("rows=%+v", rows)
	}
}

func TestBuildFocusRows_EmptyPRList(t *testing.T) {
	v := &viewState{expanded: map[int]bool{}}
	if rows := buildFocusRows(v); len(rows) != 0 {
		t.Fatalf("want empty, got %+v", rows)
	}
}

func TestFocusID_RoundTrip(t *testing.T) {
	rows := []focusRow{
		{Kind: focusPR, PRNumber: 7},
		{Kind: focusCheck, PRNumber: 7, CheckIndex: 1, CheckName: "test"},
		{Kind: focusPlaceholder, PRNumber: 7, Placeholder: placeholderLoading},
	}
	ids := []string{focusID(rows[0]), focusID(rows[1]), focusID(rows[2])}
	want := []string{"p:7", "c:7:1", "h:7:loading"}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("id[%d]=%q want %q", i, ids[i], want[i])
		}
	}
}

func TestIndexOfFocusID_FindsAndMisses(t *testing.T) {
	rows := []focusRow{
		{Kind: focusPR, PRNumber: 1},
		{Kind: focusCheck, PRNumber: 1, CheckIndex: 0, CheckName: "build"},
		{Kind: focusPR, PRNumber: 2},
	}
	if got := indexOfFocusID(rows, "c:1:0"); got != 1 {
		t.Fatalf("got %d want 1", got)
	}
	if got := indexOfFocusID(rows, "p:999"); got != -1 {
		t.Fatalf("got %d want -1", got)
	}
}

func TestRemapFocus_KeepsCheckAfterResort(t *testing.T) {
	before := []focusRow{
		{Kind: focusPR, PRNumber: 2},
		{Kind: focusPR, PRNumber: 1},
		{Kind: focusCheck, PRNumber: 1, CheckIndex: 0, CheckName: "build"},
	}
	after := []focusRow{
		{Kind: focusPR, PRNumber: 1},
		{Kind: focusCheck, PRNumber: 1, CheckIndex: 0, CheckName: "build"},
		{Kind: focusPR, PRNumber: 2},
	}
	got := remapFocusIndex(before, 2 /* on build */, after)
	if got != 1 {
		t.Fatalf("got %d want 1", got)
	}
}

func TestRemapFocus_CheckGoneFallsBackToParentPR(t *testing.T) {
	before := []focusRow{
		{Kind: focusPR, PRNumber: 1},
		{Kind: focusCheck, PRNumber: 1, CheckIndex: 0, CheckName: "build"},
	}
	after := []focusRow{
		{Kind: focusPR, PRNumber: 1},
	}
	got := remapFocusIndex(before, 1, after)
	if got != 0 {
		t.Fatalf("got %d want 0 (parent PR)", got)
	}
}

func TestRemapFocus_PRGoneClampsToLast(t *testing.T) {
	before := []focusRow{{Kind: focusPR, PRNumber: 1}, {Kind: focusPR, PRNumber: 2}}
	after := []focusRow{{Kind: focusPR, PRNumber: 3}}
	got := remapFocusIndex(before, 1, after)
	if got != 0 {
		t.Fatalf("got %d want 0 (only row)", got)
	}
}

func TestRemapFocus_EmptyAfter(t *testing.T) {
	before := []focusRow{{Kind: focusPR, PRNumber: 1}}
	got := remapFocusIndex(before, 0, nil)
	if got != 0 {
		t.Fatalf("got %d want 0", got)
	}
}

func TestRemapFocus_LoadingToChecksKeepsParentOrFirstCheck(t *testing.T) {
	before := []focusRow{
		{Kind: focusPR, PRNumber: 1},
		{Kind: focusPlaceholder, PRNumber: 1, Placeholder: placeholderLoading},
	}
	after := []focusRow{
		{Kind: focusPR, PRNumber: 1},
		{Kind: focusCheck, PRNumber: 1, CheckIndex: 0, CheckName: "ci"},
	}
	got := remapFocusIndex(before, 1, after)
	if got != 1 {
		t.Fatalf("got %d want 1 (first check after load)", got)
	}
}

func TestParentPR_FromCheckAndPlaceholder(t *testing.T) {
	prs := []PullRequest{
		{Number: 1, URL: "https://example/1"},
		{Number: 2, URL: "https://example/2"},
	}
	rows := []focusRow{
		{Kind: focusPR, PRIndex: 0, PRNumber: 1},
		{Kind: focusCheck, PRIndex: 0, PRNumber: 1, CheckIndex: 0},
		{Kind: focusPR, PRIndex: 1, PRNumber: 2},
	}
	pr, ok := parentPR(prs, rows, 1)
	if !ok || pr.Number != 1 {
		t.Fatalf("got %+v ok=%v", pr, ok)
	}
	_, ok = parentPR(prs, rows, 99)
	if ok {
		t.Fatal("expected !ok for OOB")
	}
	_, ok = parentPR(nil, nil, 0)
	if ok {
		t.Fatal("expected !ok empty")
	}
}

func TestOpenURL_PRUsesPRURL(t *testing.T) {
	prs := []PullRequest{{Number: 1, URL: "https://pr/1", CheckRuns: []CheckRun{
		{Name: "build", Permalink: "https://check/build", DetailsURL: "https://details/build"},
	}}}
	rows := buildFocusRows(&viewState{prs: prs, expanded: map[int]bool{1: true}})
	if u := openURLForFocus(prs, rows, 0); u != "https://pr/1" {
		t.Fatalf("got %q", u)
	}
}

func TestOpenURL_CheckPrefersPermalinkThenDetails(t *testing.T) {
	prs := []PullRequest{{Number: 1, URL: "https://pr/1", CheckRuns: []CheckRun{
		{Name: "a", Permalink: "https://p/a", DetailsURL: "https://d/a"},
		{Name: "b", Permalink: "", DetailsURL: "https://d/b"},
		{Name: "c", Permalink: "", DetailsURL: ""},
	}}}
	v := &viewState{prs: prs, expanded: map[int]bool{1: true}}
	rows := buildFocusRows(v)
	// indices: 0 PR, 1 a, 2 b, 3 c
	if u := openURLForFocus(prs, rows, 1); u != "https://p/a" {
		t.Fatalf("permalink: got %q", u)
	}
	if u := openURLForFocus(prs, rows, 2); u != "https://d/b" {
		t.Fatalf("details fallback: got %q", u)
	}
	if u := openURLForFocus(prs, rows, 3); u != "" {
		t.Fatalf("empty check urls: got %q want empty", u)
	}
}

func TestOpenURL_PlaceholderEmpty(t *testing.T) {
	prs := []PullRequest{{Number: 1, URL: "https://pr/1", CheckRuns: nil}}
	rows := buildFocusRows(&viewState{prs: prs, expanded: map[int]bool{1: true}})
	if u := openURLForFocus(prs, rows, 1); u != "" {
		t.Fatalf("got %q", u)
	}
}
