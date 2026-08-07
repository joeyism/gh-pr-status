package main

import "fmt"

type focusKind int

const (
	focusPR focusKind = iota
	focusCheck
	focusPlaceholder
)

const (
	placeholderLoading = "loading"
	placeholderEmpty   = "empty"
)

type focusRow struct {
	Kind        focusKind
	PRIndex     int
	PRNumber    int
	CheckIndex  int
	CheckName   string
	Placeholder string
}

func buildFocusRows(v *viewState) []focusRow {
	var rows []focusRow
	for i, pr := range v.prs {
		rows = append(rows, focusRow{
			Kind:     focusPR,
			PRIndex:  i,
			PRNumber: pr.Number,
		})

		if !v.expanded[pr.Number] {
			continue
		}
		switch {
		case pr.CheckRuns == nil:
			rows = append(rows, focusRow{
				Kind:        focusPlaceholder,
				PRIndex:     i,
				PRNumber:    pr.Number,
				Placeholder: placeholderLoading,
			})
		case len(pr.CheckRuns) == 0:
			rows = append(rows, focusRow{
				Kind:        focusPlaceholder,
				PRIndex:     i,
				PRNumber:    pr.Number,
				Placeholder: placeholderEmpty,
			})
		default:
			for j, cr := range pr.CheckRuns {
				rows = append(rows, focusRow{
					Kind:       focusCheck,
					PRIndex:    i,
					PRNumber:   pr.Number,
					CheckIndex: j,
					CheckName:  cr.Name,
				})
			}
		}
	}
	return rows
}

func focusID(r focusRow) string {
	switch r.Kind {
	case focusCheck:
		return fmt.Sprintf("c:%d:%d", r.PRNumber, r.CheckIndex)
	case focusPlaceholder:
		return fmt.Sprintf("h:%d:%s", r.PRNumber, r.Placeholder)
	default:
		return fmt.Sprintf("p:%d", r.PRNumber)
	}
}

func indexOfFocusID(rows []focusRow, id string) int {
	for i, r := range rows {
		if focusID(r) == id {
			return i
		}
	}
	return -1
}

func remapFocusIndex(before []focusRow, oldIdx int, after []focusRow) int {
	if len(after) == 0 {
		return 0
	}
	if len(before) == 0 || oldIdx < 0 || oldIdx >= len(before) {
		return 0
	}
	old := before[oldIdx]
	if i := indexOfFocusID(after, focusID(old)); i >= 0 {
		return i
	}
	// loading placeholder → first check for that PR, else empty placeholder, else PR
	if old.Kind == focusPlaceholder && old.Placeholder == placeholderLoading {
		for i, r := range after {
			if r.PRNumber == old.PRNumber && r.Kind == focusCheck {
				return i
			}
		}
		if i := indexOfFocusID(after, fmt.Sprintf("h:%d:%s", old.PRNumber, placeholderEmpty)); i >= 0 {
			return i
		}
	}
	// check/placeholder gone → parent PR
	if i := indexOfFocusID(after, fmt.Sprintf("p:%d", old.PRNumber)); i >= 0 {
		return i
	}
	// entire PR gone (or unrecoverable) → clamp to last row
	return len(after) - 1
}

func parentPR(prs []PullRequest, rows []focusRow, idx int) (PullRequest, bool) {
	if idx < 0 || idx >= len(rows) {
		return PullRequest{}, false
	}
	pi := rows[idx].PRIndex
	if pi < 0 || pi >= len(prs) {
		return PullRequest{}, false
	}
	return prs[pi], true
}

func openURLForFocus(prs []PullRequest, rows []focusRow, idx int) string {
	if idx < 0 || idx >= len(rows) {
		return ""
	}
	r := rows[idx]
	pr, ok := parentPR(prs, rows, idx)
	if !ok {
		return ""
	}
	switch r.Kind {
	case focusPR:
		return pr.URL
	case focusCheck:
		if r.CheckIndex < 0 || r.CheckIndex >= len(pr.CheckRuns) {
			return ""
		}
		cr := pr.CheckRuns[r.CheckIndex]
		if cr.Permalink != "" {
			return cr.Permalink
		}
		return cr.DetailsURL
	default:
		return ""
	}
}
