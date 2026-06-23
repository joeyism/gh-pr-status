package main

import (
	"testing"
	"time"
)

func TestSortArrow(t *testing.T) {
	if got := sortArrow(sortAsc); got != "↑" {
		t.Fatalf("sortArrow(asc) = %q, want %q", got, "↑")
	}
	if got := sortArrow(sortDesc); got != "↓" {
		t.Fatalf("sortArrow(desc) = %q, want %q", got, "↓")
	}
}

func TestComparePRForSort_Updated(t *testing.T) {
	earlier := PullRequest{Number: 1, UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	later := PullRequest{Number: 2, UpdatedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)}

	if got := comparePRForSort(earlier, later, sortFieldUpdated, sortAsc); got >= 0 {
		t.Fatalf("asc: earlier vs later = %d, want < 0", got)
	}
	if got := comparePRForSort(later, earlier, sortFieldUpdated, sortAsc); got <= 0 {
		t.Fatalf("asc: later vs earlier = %d, want > 0", got)
	}
	if got := comparePRForSort(earlier, later, sortFieldUpdated, sortDesc); got <= 0 {
		t.Fatalf("desc: earlier vs later = %d, want > 0", got)
	}
	if got := comparePRForSort(later, earlier, sortFieldUpdated, sortDesc); got >= 0 {
		t.Fatalf("desc: later vs earlier = %d, want < 0", got)
	}
}

func TestComparePRForSort_Created(t *testing.T) {
	earlier := PullRequest{Number: 1, CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	later := PullRequest{Number: 2, CreatedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)}

	if got := comparePRForSort(earlier, later, sortFieldCreated, sortAsc); got >= 0 {
		t.Fatalf("asc: earlier vs later = %d, want < 0", got)
	}
	if got := comparePRForSort(earlier, later, sortFieldCreated, sortDesc); got <= 0 {
		t.Fatalf("desc: earlier vs later = %d, want > 0", got)
	}
}

func TestComparePRForSort_StringFieldsCaseInsensitive(t *testing.T) {
	cases := []struct {
		field sortField
		a, b  PullRequest
	}{
		{sortFieldRepo, PullRequest{Number: 1, Repo: "alpha"}, PullRequest{Number: 2, Repo: "BETA"}},
		{sortFieldRepo, PullRequest{Number: 1, Repo: "Alpha"}, PullRequest{Number: 2, Repo: "beta"}},
		{sortFieldAuthor, PullRequest{Number: 1, Author: "Alice"}, PullRequest{Number: 2, Author: "BOB"}},
		{sortFieldTitle, PullRequest{Number: 1, Title: "apple"}, PullRequest{Number: 2, Title: "Banana"}},
	}
	for _, tc := range cases {
		t.Run(string(tc.field), func(t *testing.T) {
			if got := comparePRForSort(tc.a, tc.b, tc.field, sortAsc); got >= 0 {
				t.Fatalf("asc: a vs b = %d, want < 0", got)
			}
			if got := comparePRForSort(tc.a, tc.b, tc.field, sortDesc); got <= 0 {
				t.Fatalf("desc: a vs b = %d, want > 0", got)
			}
		})
	}
}

// CI rank — behavioral test: sort a mixed-status slice and assert full order.
// desc: FAILURE/ERROR > PENDING > "" > SUCCESS. asc: SUCCESS > "" > PENDING > FAILURE/ERROR.
// Distinct UpdatedAt values ensure tiebreak places #4 (FAILURE) before #5 (ERROR)
// in the tied rank-3 group, so the test reads as "FAILURE first".
func TestSortOrder_CI(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	prs := []PullRequest{
		{Number: 1, CheckStatus: "SUCCESS", UpdatedAt: base},
		{Number: 2, CheckStatus: "", UpdatedAt: base.Add(1 * time.Hour)},
		{Number: 3, CheckStatus: "PENDING", UpdatedAt: base.Add(2 * time.Hour)},
		{Number: 4, CheckStatus: "FAILURE", UpdatedAt: base.Add(4 * time.Hour)}, // latest among rank-3
		{Number: 5, CheckStatus: "ERROR", UpdatedAt: base.Add(3 * time.Hour)},
	}
	desc := append([]PullRequest(nil), prs...)
	sortPRsSlice(desc, sortFieldCI, sortDesc)
	wantDesc := []int{4, 5, 3, 2, 1} // FAILURE, ERROR, PENDING, "", SUCCESS
	for i, pr := range desc {
		if pr.Number != wantDesc[i] {
			t.Fatalf("desc position %d: got #%d (%s), want #%d", i, pr.Number, pr.CheckStatus, wantDesc[i])
		}
	}

	asc := append([]PullRequest(nil), prs...)
	sortPRsSlice(asc, sortFieldCI, sortAsc)
	wantAsc := []int{1, 2, 3, 4, 5} // SUCCESS, "", PENDING, FAILURE, ERROR (tiebreak by UpdatedAt desc)
	for i, pr := range asc {
		if pr.Number != wantAsc[i] {
			t.Fatalf("asc position %d: got #%d (%s), want #%d", i, pr.Number, pr.CheckStatus, wantAsc[i])
		}
	}
}

// FAILURE and ERROR share the same rank (both worst). Test the raw Compare so
// tiebreak (Number/UpdatedAt) doesn't interfere with the rank-equality intent.
func TestCIRank_FailureEqualsError(t *testing.T) {
	failure := PullRequest{Number: 1, CheckStatus: "FAILURE"}
	errPR := PullRequest{Number: 2, CheckStatus: "ERROR"}
	opt := sortOptions[sortFieldCI]
	if opt.Compare == nil {
		t.Fatal("CI Compare is nil")
	}
	if got := opt.Compare(failure, errPR); got != 0 {
		t.Fatalf("CI Compare(FAILURE, ERROR) = %d, want 0 (same rank)", got)
	}
}

// Review rank — desc: CHANGES_REQUESTED > REVIEW_REQUIRED > "" > APPROVED.
func TestSortOrder_Review(t *testing.T) {
	prs := []PullRequest{
		{Number: 1, ReviewDecision: "APPROVED"},
		{Number: 2, ReviewDecision: ""},
		{Number: 3, ReviewDecision: "REVIEW_REQUIRED"},
		{Number: 4, ReviewDecision: "CHANGES_REQUESTED"},
	}
	desc := append([]PullRequest(nil), prs...)
	sortPRsSlice(desc, sortFieldReview, sortDesc)
	wantDesc := []int{4, 3, 2, 1}
	for i, pr := range desc {
		if pr.Number != wantDesc[i] {
			t.Fatalf("desc position %d: got #%d (%s), want #%d", i, pr.Number, pr.ReviewDecision, wantDesc[i])
		}
	}

	asc := append([]PullRequest(nil), prs...)
	sortPRsSlice(asc, sortFieldReview, sortAsc)
	wantAsc := []int{1, 2, 3, 4}
	for i, pr := range asc {
		if pr.Number != wantAsc[i] {
			t.Fatalf("asc position %d: got #%d (%s), want #%d", i, pr.Number, pr.ReviewDecision, wantAsc[i])
		}
	}
}

// Merge rank — desc: CONFLICTING > "" > MERGEABLE.
func TestSortOrder_Merge(t *testing.T) {
	prs := []PullRequest{
		{Number: 1, Mergeable: "MERGEABLE"},
		{Number: 2, Mergeable: ""},
		{Number: 3, Mergeable: "CONFLICTING"},
	}
	desc := append([]PullRequest(nil), prs...)
	sortPRsSlice(desc, sortFieldMerge, sortDesc)
	wantDesc := []int{3, 2, 1}
	for i, pr := range desc {
		if pr.Number != wantDesc[i] {
			t.Fatalf("desc position %d: got #%d (%s), want #%d", i, pr.Number, pr.Mergeable, wantDesc[i])
		}
	}

	asc := append([]PullRequest(nil), prs...)
	sortPRsSlice(asc, sortFieldMerge, sortAsc)
	wantAsc := []int{1, 2, 3}
	for i, pr := range asc {
		if pr.Number != wantAsc[i] {
			t.Fatalf("asc position %d: got #%d (%s), want #%d", i, pr.Number, pr.Mergeable, wantAsc[i])
		}
	}
}

// Comments — desc: most unresolved first, -1 normalized to 0.
func TestSortOrder_Comments(t *testing.T) {
	prs := []PullRequest{
		{Number: 1, UnresolvedThreads: 0, TotalComments: 3, TotalThreads: 3},
		{Number: 2, UnresolvedThreads: -1, TotalComments: 5, TotalThreads: 5}, // unknown -> 0, tiebreak by TotalComments
		{Number: 3, UnresolvedThreads: 1, TotalComments: 2, TotalThreads: 2},
		{Number: 4, UnresolvedThreads: 2, TotalComments: 1, TotalThreads: 1},
	}
	desc := append([]PullRequest(nil), prs...)
	sortPRsSlice(desc, sortFieldComments, sortDesc)
	// desc: 2 unresolved (#4), 1 (#3), then tied at 0: TotalComments desc → #2(5) before #1(3)
	wantDesc := []int{4, 3, 2, 1}
	for i, pr := range desc {
		if pr.Number != wantDesc[i] {
			t.Fatalf("desc position %d: got #%d (unresolved=%d, total=%d), want #%d",
				i, pr.Number, pr.UnresolvedThreads, pr.TotalComments, wantDesc[i])
		}
	}

	asc := append([]PullRequest(nil), prs...)
	sortPRsSlice(asc, sortFieldComments, sortAsc)
	// asc: 0 unresolved first, tiebreak by TotalComments asc → #1(3) before #2(5), then #3, #4
	wantAsc := []int{1, 2, 3, 4}
	for i, pr := range asc {
		if pr.Number != wantAsc[i] {
			t.Fatalf("asc position %d: got #%d (unresolved=%d, total=%d), want #%d",
				i, pr.Number, pr.UnresolvedThreads, pr.TotalComments, wantAsc[i])
		}
	}
}

func TestComparePRForSort_Number(t *testing.T) {
	one := PullRequest{Number: 1}
	two := PullRequest{Number: 2}

	if got := comparePRForSort(one, two, sortFieldNumber, sortAsc); got >= 0 {
		t.Fatalf("asc 1 vs 2 = %d, want < 0", got)
	}
	if got := comparePRForSort(one, two, sortFieldNumber, sortDesc); got <= 0 {
		t.Fatalf("desc 1 vs 2 = %d, want > 0", got)
	}
}

// Tiebreak: when primary keys are equal, order by Number desc (stable across direction).
// a=#1 same-title-same-updated; b=#2 same. Number desc → b before a → compare(a,b) > 0.
func TestComparePRForSort_Tiebreak(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	a := PullRequest{Number: 1, UpdatedAt: t1, Title: "same"}
	b := PullRequest{Number: 2, UpdatedAt: t1, Title: "same"}

	if got := comparePRForSort(a, b, sortFieldTitle, sortAsc); got <= 0 {
		t.Fatalf("asc tiebreak: a vs b = %d, want > 0 (b has higher number, Number desc wins)", got)
	}
	if got := comparePRForSort(a, b, sortFieldTitle, sortDesc); got <= 0 {
		t.Fatalf("desc tiebreak: a vs b = %d, want > 0 (tiebreak unchanged across direction)", got)
	}
}

func TestFindPRIndexByNumber(t *testing.T) {
	prs := []PullRequest{
		{Number: 10, Title: "a"},
		{Number: 20, Title: "b"},
		{Number: 30, Title: "c"},
	}
	if got := findPRIndexByNumber(prs, 20); got != 1 {
		t.Fatalf("find 20 = %d, want 1", got)
	}
	if got := findPRIndexByNumber(prs, 99); got != -1 {
		t.Fatalf("find missing = %d, want -1", got)
	}
	if got := findPRIndexByNumber(nil, 1); got != -1 {
		t.Fatalf("find in nil = %d, want -1", got)
	}
}

func TestSortFieldOrderAndRegistry(t *testing.T) {
	if len(sortFieldOrder) != 10 {
		t.Fatalf("len(sortFieldOrder) = %d, want 10", len(sortFieldOrder))
	}
	for _, f := range sortFieldOrder {
		if _, ok := sortOptions[f]; !ok {
			t.Fatalf("field %q in sortFieldOrder but not in sortOptions", f)
		}
		if sortOptions[f].Label == "" {
			t.Fatalf("field %q has empty Label", f)
		}
		if sortOptions[f].Compare == nil {
			t.Fatalf("field %q has nil Compare", f)
		}
	}
}

func TestSortPRsSliceEndToEnd(t *testing.T) {
	prs := []PullRequest{
		{Number: 1, UpdatedAt: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)},
		{Number: 2, UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Number: 3, UpdatedAt: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)},
	}
	sortPRsSlice(prs, sortFieldUpdated, sortDesc)
	wantOrder := []int{1, 3, 2}
	for i, pr := range prs {
		if pr.Number != wantOrder[i] {
			t.Fatalf("position %d: got #%d, want #%d", i, pr.Number, wantOrder[i])
		}
	}
}
