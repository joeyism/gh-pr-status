package main

import (
	"sort"
	"strings"
	"time"
)

type sortField string

const (
	sortFieldUpdated  sortField = "updated"
	sortFieldCreated  sortField = "created"
	sortFieldRepo     sortField = "repo"
	sortFieldAuthor   sortField = "author"
	sortFieldTitle    sortField = "title"
	sortFieldCI       sortField = "ci"
	sortFieldReview   sortField = "review"
	sortFieldMerge    sortField = "merge"
	sortFieldComments sortField = "comments"
	sortFieldNumber   sortField = "number"
)

type sortDir string

const (
	sortAsc  sortDir = "asc"
	sortDesc sortDir = "desc"
)

const (
	defaultSortField = sortFieldUpdated
	defaultSortDir   = sortDesc
)

type prComparator func(a, b PullRequest) int

type sortOption struct {
	ID      sortField
	Label   string
	Compare prComparator
}

var sortFieldOrder = []sortField{
	sortFieldUpdated,
	sortFieldCreated,
	sortFieldRepo,
	sortFieldAuthor,
	sortFieldTitle,
	sortFieldCI,
	sortFieldReview,
	sortFieldMerge,
	sortFieldComments,
	sortFieldNumber,
}

// rankMap assigns a rank to each known value. HIGHER rank = "worse" so that
// desc (which flips the comparator) puts worse entries first. Unknown values
// get the "" (middle) rank.
var ciRankMap = map[string]int{
	"SUCCESS":  0,
	"":         1,
	"PENDING":  2,
	"FAILURE":  3,
	"ERROR":    3, // FAILURE and ERROR share the worst rank
}
var reviewRankMap = map[string]int{
	"APPROVED":          0,
	"":                  1,
	"REVIEW_REQUIRED":   2,
	"CHANGES_REQUESTED": 3,
}
var mergeRankMap = map[string]int{
	"MERGEABLE":  0,
	"":           1,
	"CONFLICTING": 2,
}

func rankOf(m map[string]int, v string) int {
	if r, ok := m[v]; ok {
		return r
	}
	// Unknown values: place at the "" (middle) rank.
	return m[""]
}

func compareStrings(a, b string) int {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func compareTime(a, b time.Time) int {
	if a.Before(b) {
		return -1
	}
	if a.After(b) {
		return 1
	}
	return 0
}

// All Compare functions return ascending-natural order ("lower / better /
// earlier first"). comparePRForSort flips the whole result for desc so that
// "desc = worst / latest / most first" for every field including tiebreaks.
var sortOptions = map[sortField]sortOption{
	sortFieldUpdated: {ID: sortFieldUpdated, Label: "Updated", Compare: func(a, b PullRequest) int {
		return compareTime(a.UpdatedAt, b.UpdatedAt)
	}},
	sortFieldCreated: {ID: sortFieldCreated, Label: "Created", Compare: func(a, b PullRequest) int {
		return compareTime(a.CreatedAt, b.CreatedAt)
	}},
	sortFieldRepo: {ID: sortFieldRepo, Label: "Repo", Compare: func(a, b PullRequest) int {
		return compareStrings(a.Repo, b.Repo)
	}},
	sortFieldAuthor: {ID: sortFieldAuthor, Label: "Author", Compare: func(a, b PullRequest) int {
		return compareStrings(a.Author, b.Author)
	}},
	sortFieldTitle: {ID: sortFieldTitle, Label: "Title", Compare: func(a, b PullRequest) int {
		return compareStrings(a.Title, b.Title)
	}},
	sortFieldCI: {ID: sortFieldCI, Label: "CI", Compare: func(a, b PullRequest) int {
		return rankOf(ciRankMap, a.CheckStatus) - rankOf(ciRankMap, b.CheckStatus)
	}},
	sortFieldReview: {ID: sortFieldReview, Label: "Review", Compare: func(a, b PullRequest) int {
		return rankOf(reviewRankMap, a.ReviewDecision) - rankOf(reviewRankMap, b.ReviewDecision)
	}},
	sortFieldMerge: {ID: sortFieldMerge, Label: "Merge", Compare: func(a, b PullRequest) int {
		return rankOf(mergeRankMap, a.Mergeable) - rankOf(mergeRankMap, b.Mergeable)
	}},
	sortFieldComments: {ID: sortFieldComments, Label: "Comments", Compare: func(a, b PullRequest) int {
		// Normalize -1 (unknown/truncated) to 0 for sort purposes.
		au, bu := a.UnresolvedThreads, b.UnresolvedThreads
		if au < 0 {
			au = 0
		}
		if bu < 0 {
			bu = 0
		}
		if au != bu {
			return au - bu
		}
		if a.TotalComments != b.TotalComments {
			return a.TotalComments - b.TotalComments
		}
		return a.TotalThreads - b.TotalThreads
	}},
	sortFieldNumber: {ID: sortFieldNumber, Label: "Number", Compare: func(a, b PullRequest) int {
		return a.Number - b.Number
	}},
}

// comparePRForSort returns -1/0/+1 placing a before b in the chosen direction.
// Includes fixed deterministic tiebreaks (UpdatedAt desc, then Number desc)
// that do not flip with direction, so equal-keyed PRs keep stable order.
func comparePRForSort(a, b PullRequest, field sortField, dir sortDir) int {
	opt, ok := sortOptions[field]
	if !ok {
		return 0
	}
	cmp := opt.Compare(a, b)
	if dir == sortDesc {
		cmp = -cmp
	}
	if cmp != 0 {
		return cmp
	}
	// Fixed tiebreaks: always UpdatedAt desc, then Number desc.
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		if a.UpdatedAt.After(b.UpdatedAt) {
			return -1
		}
		return 1
	}
	if a.Number != b.Number {
		if a.Number > b.Number {
			return -1
		}
		return 1
	}
	return 0
}

func sortArrow(dir sortDir) string {
	if dir == sortAsc {
		return "↑"
	}
	return "↓"
}

// indexOfSortField returns the index of f in sortFieldOrder, or 0 if absent.
func indexOfSortField(f sortField) int {
	for i, x := range sortFieldOrder {
		if x == f {
			return i
		}
	}
	return 0
}

func findPRIndexByNumber(prs []PullRequest, number int) int {
	for i, pr := range prs {
		if pr.Number == number {
			return i
		}
	}
	return -1
}

// sortPRsSlice sorts the slice in place using the given field/dir with stable
// tiebreaks. Exposed for end-to-end tests; the model's sortPRs uses it too.
func sortPRsSlice(prs []PullRequest, field sortField, dir sortDir) {
	sort.SliceStable(prs, func(i, j int) bool {
		return comparePRForSort(prs[i], prs[j], field, dir) < 0
	})
}

func parseSortField(raw string) (sortField, bool) {
	switch normalize(raw) {
	case "updated":
		return sortFieldUpdated, true
	case "created":
		return sortFieldCreated, true
	case "repo":
		return sortFieldRepo, true
	case "author":
		return sortFieldAuthor, true
	case "title":
		return sortFieldTitle, true
	case "ci":
		return sortFieldCI, true
	case "review":
		return sortFieldReview, true
	case "merge":
		return sortFieldMerge, true
	case "comments":
		return sortFieldComments, true
	case "number":
		return sortFieldNumber, true
	}
	return "", false
}

func parseSortDir(raw string) (sortDir, bool) {
	switch normalize(raw) {
	case "asc":
		return sortAsc, true
	case "desc":
		return sortDesc, true
	}
	return "", false
}
