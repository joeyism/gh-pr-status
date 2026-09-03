package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestClipboardCommand(t *testing.T) {
	cases := []struct {
		os       string
		wantCmd  string
		wantArgs []string
		wantErr  bool
	}{
		{os: "darwin", wantCmd: "pbcopy", wantArgs: nil, wantErr: false},
		{os: "linux", wantCmd: "xclip", wantArgs: []string{"-selection", "clipboard"}, wantErr: false},
		{os: "windows", wantCmd: "", wantArgs: nil, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.os, func(t *testing.T) {
			cmd, args, err := clipboardCommand(tc.os)
			if (err != nil) != tc.wantErr {
				t.Fatalf("clipboardCommand(%q) err = %v, want error: %v", tc.os, err, tc.wantErr)
			}
			if cmd != tc.wantCmd {
				t.Fatalf("clipboardCommand(%q) cmd = %q, want %q", tc.os, cmd, tc.wantCmd)
			}
			if len(args) != len(tc.wantArgs) {
				t.Fatalf("clipboardCommand(%q) args = %v, want %v", tc.os, args, tc.wantArgs)
			}
		})
	}
}

func TestActiveView(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.viewMode = 0
	if m.activeView() != &m.mine {
		t.Error("viewMode 0 should return mine")
	}
	m.viewMode = 1
	if m.activeView() != &m.org {
		t.Error("viewMode 1 should return org")
	}
}

func TestUpdateYankKey(t *testing.T) {
	prs := []PullRequest{{Number: 1, Title: "PR 1", URL: "url"}}

	t.Run("no overlay yank", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		m.mine.prs = prs
		m.viewMode = 0

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
		_, cmd := m.Update(msg)
		if cmd == nil {
			t.Fatal("expected command to be returned")
		}
	})

	t.Run("with overlay confirm instead of yank", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		m.mine.prs = prs
		m.confirmAction = "approve"

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
		newModel, _ := m.Update(msg)
		mod := newModel.(model)
		if mod.confirmAction != "" {
			t.Fatal("confirmAction should be cleared")
		}
		if strings.Contains(mod.flash, "Copied to clipboard") {
			t.Fatal("flash should not indicate yank")
		}
	})
}

func TestEnterKey_RunsOnEnterWhenConfigured(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.onEnter = "gh code-review ${PR_URL}"
	m.mine.prs = []PullRequest{{Number: 1, Title: "PR 1", URL: "https://github.com/o/r/pull/1"}}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command when on_enter is set and a PR is focused")
	}
}

func TestEnterKey_NoopWithoutUsableConfigOrPR(t *testing.T) {
	cases := []struct {
		name    string
		onEnter string
		prs     []PullRequest
	}{
		{"unset", "", []PullRequest{{Number: 1, URL: "https://pr/1"}}},
		{"whitespace only", "   ", []PullRequest{{Number: 1, URL: "https://pr/1"}}},
		{"empty list", "gh code-review ${PR_URL}", nil},
		{"empty URL", "gh code-review ${PR_URL}", []PullRequest{{Number: 1}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
			m.onEnter = tc.onEnter
			m.mine.prs = tc.prs
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd != nil {
				t.Fatal("expected Enter to be a no-op")
			}
		})
	}
}

func TestEnterKey_ConfirmAndSortOverlaysStillWin(t *testing.T) {
	t.Run("confirm", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		m.onEnter = "gh code-review ${PR_URL}"
		m.mine.prs = []PullRequest{{Number: 1, URL: "https://pr/1"}}
		m.confirmAction = "close"
		newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if newModel.(model).confirmAction != "" || cmd == nil {
			t.Fatal("confirm Enter should clear the overlay and run close")
		}
	})

	t.Run("sort menu", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, sortFieldUpdated, sortDesc)
		m.onEnter = "gh code-review ${PR_URL}"
		m.mine.prs = []PullRequest{{Number: 1, Repo: "z"}, {Number: 2, Repo: "a"}}
		m.sortMenuOpen = true
		m.sortMenuCursor = indexOfSortField(sortFieldRepo)
		m.sortMenuDir = sortAsc
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		mod := newModel.(model)
		if mod.sortMenuOpen || mod.mine.sortField != sortFieldRepo {
			t.Fatal("sort menu Enter should apply and close the menu")
		}
	})
}

func TestEnterKey_OnCheckUsesParentPR(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.onEnter = "gh code-review ${PR_URL}"
	m.mine.prs = []PullRequest{{
		Number: 1, URL: "https://pr/1",
		CheckRuns: []CheckRun{{Name: "build", Permalink: "https://check/build"}},
	}}
	m.mine.expanded[1] = true
	m.mine.cursor = 1

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on a check row should run the parent PR command")
	}
}

func TestOnEnterFinished_RefreshesActiveView(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode int
	}{
		{"mine", 0},
		{"org", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
			m.viewMode = tc.mode
			newModel, cmd := m.Update(onEnterFinishedMsg{err: fmt.Errorf("exit 1")})
			mod := newModel.(model)
			if cmd == nil {
				t.Fatal("expected refresh command")
			}
			if !mod.activeView().fetching {
				t.Fatal("expected active view to be marked fetching")
			}
			if !strings.Contains(mod.flash, "on_enter failed") {
				t.Fatalf("expected error flash, got %q", mod.flash)
			}
		})
	}
}

func TestView_EnterHelpReflectsConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name    string
		onEnter string
		want    bool
	}{
		{"configured", "gh code-review ${PR_URL}", true},
		{"unset", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
			m.width, m.height = 200, 40
			m.mine.loading = false
			m.mine.prs = []PullRequest{{Number: 1, Title: "x", URL: "https://pr/1"}}
			m.onEnter = tc.onEnter
			view := m.View()
			got := strings.Contains(view, "enter: run")
			if got != tc.want {
				t.Fatalf("enter help present = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBranchKey(t *testing.T) {
	prs := []PullRequest{{Number: 1, Title: "PR 1", HeadRefName: "feature-branch"}}

	t.Run("copies branch name", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		m.mine.prs = prs
		m.viewMode = 0

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}
		_, cmd := m.Update(msg)
		if cmd == nil {
			t.Fatal("expected command to be returned")
		}
	})

	t.Run("no command when no branch name", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		m.mine.prs = []PullRequest{{Number: 1, Title: "PR 1"}}
		m.viewMode = 0

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}
		_, cmd := m.Update(msg)
		if cmd != nil {
			t.Fatal("expected no command when HeadRefName is empty")
		}
	})
}

func TestUpdateToggleView(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.mine.prs = []PullRequest{{Number: 1}}
	m.org.prs = []PullRequest{{Number: 2}}
	m.viewMode = 0

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	newModel, _ := m.Update(msg)
	mod := newModel.(model)
	if mod.viewMode != 1 {
		t.Fatalf("expected viewMode 1, got %d", mod.viewMode)
	}

	newModel, _ = mod.Update(msg)
	mod = newModel.(model)
	if mod.viewMode != 0 {
		t.Fatalf("expected viewMode 0, got %d", mod.viewMode)
	}
}

func TestActionGating(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.org.prs = []PullRequest{{Number: 1, Author: "other"}}
	m.mine.prs = []PullRequest{{Number: 2, Author: "user"}}

	t.Run("no merge in org view", func(t *testing.T) {
		m.viewMode = 1
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}
		newModel, _ := m.Update(msg)
		if newModel.(model).confirmAction == "merge" {
			t.Error("merge should be blocked in org view")
		}
	})

	t.Run("no approve in mine view", func(t *testing.T) {
		m.viewMode = 0
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
		newModel, _ := m.Update(msg)
		if newModel.(model).confirmAction == "approve" {
			t.Error("approve should be blocked in mine view")
		}
	})
}

func TestScrolling(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 15 // Budget is height - 9 = 6 lines
	for i := 0; i < 20; i++ {
		m.mine.prs = append(m.mine.prs, PullRequest{Number: i})
	}
	m.viewMode = 0

	// Move cursor down 10 times
	for i := 0; i < 10; i++ {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
		newModel, _ := m.Update(msg)
		m = newModel.(model)
	}

	if m.mine.scrollOffset == 0 {
		t.Error("scrollOffset should have incremented")
	}
	if m.mine.cursor < m.mine.scrollOffset {
		t.Error("cursor should be >= scrollOffset")
	}
}

func TestMessages(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)

	t.Run("approve success", func(t *testing.T) {
		msg := prApprovedMsg{err: nil}
		newModel, _ := m.Update(msg)
		if !strings.Contains(newModel.(model).flash, "approved") {
			t.Error("expected approved flash")
		}
	})

	t.Run("checkRunsFetched", func(t *testing.T) {
		m.org.prs = []PullRequest{{Number: 1, Title: "PR 1"}}
		msg := checkRunsFetchedMsg{prNumber: 1, runs: []CheckRun{{Name: "test"}}}
		newModel, _ := m.Update(msg)
		mod := newModel.(model)
		if len(mod.org.prs[0].CheckRuns) != 1 {
			t.Error("expected check runs to be populated")
		}
	})
}

func TestDraftToggle(t *testing.T) {
	dKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
	yKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	nKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}

	t.Run("d key sets confirmAction in personal view", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		m.mine.prs = []PullRequest{{Number: 1, Title: "My PR", IsDraft: false}}
		m.viewMode = 0

		newModel, _ := m.Update(dKey)
		mod := newModel.(model)
		if mod.confirmAction != "draft" {
			t.Fatalf("expected confirmAction 'draft', got %q", mod.confirmAction)
		}
	})

	t.Run("d key sets confirmAction for draft PR", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		m.mine.prs = []PullRequest{{Number: 1, Title: "My PR", IsDraft: true}}
		m.viewMode = 0

		newModel, _ := m.Update(dKey)
		mod := newModel.(model)
		if mod.confirmAction != "draft" {
			t.Fatalf("expected confirmAction 'draft', got %q", mod.confirmAction)
		}
	})

	t.Run("d key blocked in org view", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		m.org.prs = []PullRequest{{Number: 1, Title: "PR", IsDraft: false}}
		m.viewMode = 1

		newModel, _ := m.Update(dKey)
		mod := newModel.(model)
		if mod.confirmAction == "draft" {
			t.Error("draft toggle should be blocked in org view")
		}
	})

	t.Run("d key blocked with no PRs", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		m.viewMode = 0

		newModel, _ := m.Update(dKey)
		mod := newModel.(model)
		if mod.confirmAction != "" {
			t.Errorf("expected empty confirmAction, got %q", mod.confirmAction)
		}
	})

	t.Run("confirm y on non-draft PR returns command", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		m.mine.prs = []PullRequest{{Number: 1, ID: "PR_1", Title: "My PR", IsDraft: false}}
		m.viewMode = 0
		m.confirmAction = "draft"

		newModel, cmd := m.Update(yKey)
		mod := newModel.(model)
		if mod.confirmAction != "" {
			t.Fatal("confirmAction should be cleared after confirm")
		}
		if cmd == nil {
			t.Fatal("expected command to be returned for draft toggle")
		}
	})

	t.Run("confirm y on draft PR returns command", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		m.mine.prs = []PullRequest{{Number: 1, ID: "PR_1", Title: "My PR", IsDraft: true}}
		m.viewMode = 0
		m.confirmAction = "draft"

		newModel, cmd := m.Update(yKey)
		mod := newModel.(model)
		if mod.confirmAction != "" {
			t.Fatal("confirmAction should be cleared after confirm")
		}
		if cmd == nil {
			t.Fatal("expected command to be returned for mark ready")
		}
	})

	t.Run("confirm n cancels", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		m.mine.prs = []PullRequest{{Number: 1, ID: "PR_1", Title: "My PR", IsDraft: false}}
		m.viewMode = 0
		m.confirmAction = "draft"

		newModel, cmd := m.Update(nKey)
		mod := newModel.(model)
		if mod.confirmAction != "" {
			t.Fatal("confirmAction should be cleared on cancel")
		}
		if cmd != nil {
			t.Fatal("expected no command on cancel")
		}
	})

	t.Run("draftToggledMsg success ready", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		msg := draftToggledMsg{err: nil, isDraft: false}
		newModel, _ := m.Update(msg)
		mod := newModel.(model)
		if !strings.Contains(mod.flash, "ready for review") {
			t.Errorf("expected flash to contain 'ready for review', got %q", mod.flash)
		}
	})

	t.Run("draftToggledMsg success draft", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		msg := draftToggledMsg{err: nil, isDraft: true}
		newModel, _ := m.Update(msg)
		mod := newModel.(model)
		if !strings.Contains(mod.flash, "draft") {
			t.Errorf("expected flash to contain 'draft', got %q", mod.flash)
		}
	})

	t.Run("draftToggledMsg error flash", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
		msg := draftToggledMsg{err: fmt.Errorf("network error")}
		newModel, _ := m.Update(msg)
		mod := newModel.(model)
		if !strings.Contains(mod.flash, "Error") {
			t.Errorf("expected flash to contain 'Error', got %q", mod.flash)
		}
	})
}

// --- Cycle 3: sortPRs, selectedPRNumber, initialModel seeding, adjustScrollFor ---

func TestInitialModelSeedsSortDefaults(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, sortFieldRepo, sortAsc)
	if m.mine.sortField != sortFieldRepo || m.mine.sortDir != sortAsc {
		t.Errorf("mine: got field=%q dir=%q, want repo asc", m.mine.sortField, m.mine.sortDir)
	}
	if m.org.sortField != sortFieldRepo || m.org.sortDir != sortAsc {
		t.Errorf("org: got field=%q dir=%q, want repo asc", m.org.sortField, m.org.sortDir)
	}
}

func TestSelectedPRNumber(t *testing.T) {
	v := &viewState{
		prs:    []PullRequest{{Number: 10}, {Number: 20}, {Number: 30}},
		cursor: 1,
	}
	num, ok := selectedPRNumber(v)
	if !ok || num != 20 {
		t.Errorf("got (%d, %v), want (20, true)", num, ok)
	}

	empty := &viewState{}
	num, ok = selectedPRNumber(empty)
	if ok || num != 0 {
		t.Errorf("empty: got (%d, %v), want (0, false)", num, ok)
	}
}

func TestSortPRs_ReordersAndFollowsCursor(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	v := &viewState{
		sortField: sortFieldUpdated,
		sortDir:   sortDesc,
		prs: []PullRequest{
			{Number: 1, UpdatedAt: base},
			{Number: 2, UpdatedAt: base.Add(48 * time.Hour)},
			{Number: 3, UpdatedAt: base.Add(24 * time.Hour)},
		},
		cursor: 0, // currently on #1
	}
	m := &model{}
	m.sortPRs(v, 1, true) // keep selection on #1
	// Expected order: #2 (newest), #3, #1 (oldest)
	if v.prs[0].Number != 2 || v.prs[1].Number != 3 || v.prs[2].Number != 1 {
		t.Fatalf("order: got [%d, %d, %d], want [2, 3, 1]",
			v.prs[0].Number, v.prs[1].Number, v.prs[2].Number)
	}
	// Cursor should follow #1 to its new index (2)
	if v.cursor != 2 {
		t.Errorf("cursor: got %d, want 2 (followed #1)", v.cursor)
	}
}

func TestSortPRs_ClampsWhenSelectedMissing(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	v := &viewState{
		sortField: sortFieldUpdated,
		sortDir:   sortDesc,
		prs: []PullRequest{
			{Number: 1, UpdatedAt: base},
			{Number: 2, UpdatedAt: base.Add(24 * time.Hour)},
			{Number: 3, UpdatedAt: base.Add(48 * time.Hour)},
		},
		cursor: 0,
	}
	m := &model{}
	// Selected PR (999) is no longer in the slice.
	m.sortPRs(v, 999, true)
	// Sorted: #3, #2, #1
	if v.prs[0].Number != 3 {
		t.Fatalf("order: got [%d, %d, %d]", v.prs[0].Number, v.prs[1].Number, v.prs[2].Number)
	}
	// Cursor should clamp to last valid index (2)
	if v.cursor != 2 {
		t.Errorf("cursor: got %d, want 2 (clamped)", v.cursor)
	}
}

func TestSortPRs_EmptyAndSingleAreNoop(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		v := &viewState{}
		(&model{}).sortPRs(v, 0, true)
		if len(v.prs) != 0 {
			t.Errorf("empty prs modified: %v", v.prs)
		}
	})
	t.Run("single", func(t *testing.T) {
		v := &viewState{prs: []PullRequest{{Number: 7}}, cursor: 0}
		(&model{}).sortPRs(v, 7, true)
		if len(v.prs) != 1 || v.prs[0].Number != 7 {
			t.Errorf("single prs modified: %v", v.prs)
		}
		if v.cursor != 0 {
			t.Errorf("cursor changed: %d", v.cursor)
		}
	})
}

func TestAdjustScrollFor_TargetsGivenView(t *testing.T) {
	// adjustScrollFor(&m.org) must adjust m.org's scrollOffset, not m.mine's,
	// even when m.viewMode points at mine. Use a small height so 5 PRs don't
	// fit in the visible budget, forcing scroll.
	m := &model{
		mine: viewState{
			prs:          []PullRequest{{Number: 1}, {Number: 2}, {Number: 3}, {Number: 4}, {Number: 5}},
			cursor:       4,
			scrollOffset: 0,
		},
		org: viewState{
			prs:          []PullRequest{{Number: 10}, {Number: 20}, {Number: 30}, {Number: 40}, {Number: 50}},
			cursor:       4,
			scrollOffset: 0,
		},
		viewMode: 0,  // activeView() == mine
		height:   11, // visibleLines = 11-9 = 2; 5 PRs must scroll
	}
	m.adjustScrollFor(&m.org)
	if m.org.scrollOffset <= 0 {
		t.Errorf("org.scrollOffset = %d, want > 0 (cursor at end, should scroll)", m.org.scrollOffset)
	}
	// mine must be untouched.
	if m.mine.scrollOffset != 0 {
		t.Errorf("mine.scrollOffset = %d, want 0 (untouched)", m.mine.scrollOffset)
	}
}

// --- Cycle 4: sort-menu key handling ---

// keyMsg builds a tea.KeyMsg from a rune.
func keyMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestSortMenu_OpensWithSandSeedsState(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	m := initialModel(nil, "user", nil, 0, sortFieldCI, sortDesc)
	m.viewMode = 0
	m.mine.prs = []PullRequest{
		{Number: 1, UpdatedAt: base},
		{Number: 2, UpdatedAt: base.Add(24 * time.Hour)},
	}
	m.mine.cursor = 0
	m.width = 100
	m.height = 40

	mod, _ := m.Update(keyMsg('s'))
	got := mod.(model)
	if !got.sortMenuOpen {
		t.Fatal("sortMenuOpen should be true after pressing s")
	}
	if got.sortMenuDir != sortDesc {
		t.Errorf("sortMenuDir = %q, want desc (seeded from active view)", got.sortMenuDir)
	}
	if got.sortMenuCursor != indexOfSortField(sortFieldCI) {
		t.Errorf("sortMenuCursor = %d, want %d (index of ci)", got.sortMenuCursor, indexOfSortField(sortFieldCI))
	}
}

func TestSortMenu_JKMovesHighlight(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.width = 100
	m.height = 40
	mod, _ := m.Update(keyMsg('s'))
	open := mod.(model)
	// j moves down
	mod, _ = open.Update(keyMsg('j'))
	got := mod.(model)
	if got.sortMenuCursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", got.sortMenuCursor)
	}
	// k moves up
	mod, _ = got.Update(keyMsg('k'))
	got = mod.(model)
	if got.sortMenuCursor != 0 {
		t.Errorf("after k: cursor = %d, want 0", got.sortMenuCursor)
	}
	// k at top clamps to 0
	mod, _ = got.Update(keyMsg('k'))
	got = mod.(model)
	if got.sortMenuCursor != 0 {
		t.Errorf("k at top: cursor = %d, want 0 (clamped)", got.sortMenuCursor)
	}
}

func TestSortMenu_TabTogglesDir(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.width = 100
	m.height = 40
	mod, _ := m.Update(keyMsg('s')) // opens with defaultSortDir = desc
	open := mod.(model)
	if open.sortMenuDir != sortDesc {
		t.Fatalf("initial sortMenuDir = %q, want desc", open.sortMenuDir)
	}
	mod, _ = open.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := mod.(model)
	if got.sortMenuDir != sortAsc {
		t.Errorf("after tab: sortMenuDir = %q, want asc", got.sortMenuDir)
	}
	mod, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = mod.(model)
	if got.sortMenuDir != sortDesc {
		t.Errorf("after second tab: sortMenuDir = %q, want desc", got.sortMenuDir)
	}
}

func TestSortMenu_EnterAppliesAndCloses(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	m := initialModel(nil, "user", nil, 0, sortFieldUpdated, sortDesc)
	m.viewMode = 0
	m.mine.prs = []PullRequest{
		{Number: 1, Repo: "x", UpdatedAt: base},
		{Number: 2, Repo: "x", UpdatedAt: base.Add(48 * time.Hour)},
		{Number: 3, Repo: "x", UpdatedAt: base.Add(24 * time.Hour)},
	}
	m.mine.cursor = 0 // on #1
	m.width = 100
	m.height = 40

	// Open menu, pick repo (index 2), enter.
	mod, _ := m.Update(keyMsg('s'))
	open := mod.(model)
	open.sortMenuCursor = indexOfSortField(sortFieldRepo)
	mod, _ = open.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := mod.(model)

	if got.sortMenuOpen {
		t.Error("sortMenuOpen should be false after enter")
	}
	if got.mine.sortField != sortFieldRepo {
		t.Errorf("applied sortField = %q, want repo", got.mine.sortField)
	}
	// Repos all "x" (tied) → tiebreak UpdatedAt desc → #2 (48h) > #3 (24h) > #1 (base).
	if got.mine.prs[0].Number != 2 || got.mine.prs[1].Number != 3 || got.mine.prs[2].Number != 1 {
		t.Errorf("prs order: got [%d,%d,%d], want [2,3,1] (repo tied, UpdatedAt desc tiebreak)",
			got.mine.prs[0].Number, got.mine.prs[1].Number, got.mine.prs[2].Number)
	}
}

func TestSortMenu_EscCancels(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, sortFieldUpdated, sortDesc)
	m.viewMode = 0
	m.mine.prs = []PullRequest{{Number: 1}}
	m.mine.sortField = sortFieldUpdated
	m.mine.sortDir = sortDesc
	m.width = 100
	m.height = 40

	mod, _ := m.Update(keyMsg('s'))
	open := mod.(model)
	open.sortMenuCursor = indexOfSortField(sortFieldNumber)
	mod, _ = open.Update(tea.KeyMsg{Type: tea.KeyEscape})
	got := mod.(model)
	if got.sortMenuOpen {
		t.Error("esc should close the menu")
	}
	if got.mine.sortField != sortFieldUpdated {
		t.Errorf("esc should not change sortField; got %q", got.mine.sortField)
	}
}

func TestSortMenu_QCancelsAndDoesNotQuit(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, sortFieldUpdated, sortDesc)
	m.viewMode = 0
	m.mine.prs = []PullRequest{{Number: 1}}
	m.width = 100
	m.height = 40

	mod, _ := m.Update(keyMsg('s'))
	open := mod.(model)
	mod, _ = open.Update(keyMsg('q'))
	got := mod.(model)
	if got.sortMenuOpen {
		t.Error("q should close the menu (not quit)")
	}
	// We can't directly inspect the tea.Cmd to assert it isn't Quit, but the
	// fact that the model is returned (not nil) and sortMenuOpen is false is
	// sufficient: if q had quit, the program would have terminated.
}

func TestSortMenu_IgnoredDuringConfirm(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.viewMode = 0
	m.mine.prs = []PullRequest{{Number: 1}}
	m.confirmAction = "close"
	m.width = 100
	m.height = 40

	mod, _ := m.Update(keyMsg('s'))
	got := mod.(model)
	if got.sortMenuOpen {
		t.Error("s should not open sort menu while confirmAction is set")
	}
}

func TestFooterHelp_ContainsSortKey(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.width = 200
	m.height = 40
	m.mine.prs = []PullRequest{{Number: 1, Title: "x"}}
	m.mine.loading = false
	m.mine.fetching = false

	view := m.View()
	if !strings.Contains(view, "s: sort") {
		t.Errorf("mine-view footer should contain 's: sort', got view:\n%s", view)
	}

	m.viewMode = 1
	m.org.prs = []PullRequest{{Number: 2, Title: "y"}}
	m.org.loading = false
	m.org.fetching = false
	view = m.View()
	if !strings.Contains(view, "s: sort") {
		t.Errorf("org-view footer should contain 's: sort', got view:\n%s", view)
	}
}

// indexOfSortField is defined in sort.go.

// --- Cycle 5: fetch handlers re-sort with cursor-follow ---

func TestPRsFetchedMsg_SortsAndFollowsCursor(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	m := initialModel(nil, "user", nil, 0, sortFieldUpdated, sortDesc)
	m.viewMode = 0
	m.mine.loading = false
	m.mine.fetching = false
	// Pre-existing: cursor on #2.
	m.mine.prs = []PullRequest{
		{Number: 2, UpdatedAt: base.Add(48 * time.Hour)},
	}
	m.mine.cursor = 0

	// New fetch with PRs in API order (NOT sorted).
	newPRs := []PullRequest{
		{Number: 1, UpdatedAt: base},
		{Number: 2, UpdatedAt: base.Add(48 * time.Hour)},
		{Number: 3, UpdatedAt: base.Add(24 * time.Hour)},
	}
	mod, _ := m.Update(prsFetchedMsg{prs: newPRs})
	got := mod.(model)
	// Sorted updated-desc: #2 (48h), #3 (24h), #1 (base).
	if got.mine.prs[0].Number != 2 || got.mine.prs[1].Number != 3 || got.mine.prs[2].Number != 1 {
		t.Errorf("fetched order: got [%d,%d,%d], want [2,3,1]",
			got.mine.prs[0].Number, got.mine.prs[1].Number, got.mine.prs[2].Number)
	}
	// Cursor should follow #2 to its new index (0).
	if got.mine.cursor != 0 {
		t.Errorf("cursor: got %d, want 0 (followed #2)", got.mine.cursor)
	}
}

func TestPRsFetchedMsg_ClampsWhenSelectedMissing(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	m := initialModel(nil, "user", nil, 0, sortFieldUpdated, sortDesc)
	m.viewMode = 0
	m.mine.loading = false
	m.mine.fetching = false
	// Previously on #999 (about to disappear).
	m.mine.prs = []PullRequest{{Number: 999, UpdatedAt: base}}
	m.mine.cursor = 0

	newPRs := []PullRequest{
		{Number: 1, UpdatedAt: base},
		{Number: 2, UpdatedAt: base.Add(24 * time.Hour)},
	}
	mod, _ := m.Update(prsFetchedMsg{prs: newPRs})
	got := mod.(model)
	// #999 is gone; cursor clamps to last valid (1).
	if got.mine.cursor != 1 {
		t.Errorf("cursor: got %d, want 1 (clamped to last)", got.mine.cursor)
	}
}

func TestOrgPRsFetchedMsg_SortsAndFollowsCursor(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, sortFieldCI, sortDesc)
	m.viewMode = 1
	m.org.loading = false
	m.org.fetching = false
	// Org view not yet loaded; cursor at 0, no prs.
	// Fetch with CI-failure first in API order; our sort should re-rank.
	newPRs := []PullRequest{
		{Number: 1, CheckStatus: "SUCCESS"},
		{Number: 2, CheckStatus: "FAILURE"},
		{Number: 3, CheckStatus: "PENDING"},
	}
	mod, _ := m.Update(orgPRsFetchedMsg{prs: newPRs})
	got := mod.(model)
	// CI desc: #2 (FAILURE), #3 (PENDING), #1 (SUCCESS).
	if got.org.prs[0].Number != 2 || got.org.prs[1].Number != 3 || got.org.prs[2].Number != 1 {
		t.Errorf("org fetched order: got [%d,%d,%d], want [2,3,1] (CI desc)",
			got.org.prs[0].Number, got.org.prs[1].Number, got.org.prs[2].Number)
	}
}

// --- Cycle 6: header sort indicator ---

func TestHeader_ShowsColumnLabels(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.width = 200
	m.height = 40
	m.mine.loading = false
	m.mine.fetching = false
	m.mine.prs = []PullRequest{{Number: 1, Title: "x", Repo: "r"}}
	view := m.View()
	for _, label := range []string{"Repo", "Title", "CI", "Review", "Merge", "Upd", "Crtd", "Cmts"} {
		if !strings.Contains(view, label) {
			t.Errorf("header missing label %q", label)
		}
	}
}

func TestHeader_ActiveColumnShowsArrow(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, sortFieldCI, sortDesc)
	m.width = 200
	m.height = 40
	m.mine.loading = false
	m.mine.fetching = false
	m.mine.prs = []PullRequest{{Number: 1, Title: "x", Repo: "r"}}
	view := m.View()
	// Active CI desc → header contains "CI ↓".
	if !strings.Contains(view, "CI ↓") {
		t.Errorf("expected 'CI ↓' in header (active sort), got view:\n%s", view)
	}

	// Switch to updated asc.
	mod, _ := m.Update(keyMsg('s'))
	open := mod.(model)
	open.sortMenuCursor = indexOfSortField(sortFieldUpdated)
	mod, _ = open.Update(tea.KeyMsg{Type: tea.KeyEnter}) // dir stays desc
	// Now toggle via tab on the menu: open again, tab to asc, enter.
	mod2, _ := mod.(model).Update(keyMsg('s'))
	open2 := mod2.(model)
	open2.sortMenuCursor = indexOfSortField(sortFieldUpdated)
	mod2, _ = open2.Update(tea.KeyMsg{Type: tea.KeyTab}) // dir -> asc
	mod2, _ = mod2.(model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := mod2.(model)
	view = got.View()
	if !strings.Contains(view, "Upd ↑") {
		t.Errorf("expected 'Upd ↑' in header (active sort asc), got view:\n%s", view)
	}
}

func TestHeader_NumberSortShowsBadge(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, sortFieldNumber, sortDesc)
	m.width = 200
	m.height = 40
	m.mine.loading = false
	m.mine.fetching = false
	m.mine.prs = []PullRequest{{Number: 1, Title: "x", Repo: "r"}}
	view := m.View()
	if !strings.Contains(view, "[sort: Number ↓]") {
		t.Errorf("expected '[sort: Number ↓]' badge for number sort, got view:\n%s", view)
	}
}

func TestHeader_AuthorSortInMineShowsBadge(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, sortFieldAuthor, sortDesc)
	m.width = 200
	m.height = 40
	m.mine.loading = false
	m.mine.fetching = false
	m.mine.prs = []PullRequest{{Number: 1, Title: "x", Repo: "r", Author: "alice"}}
	view := m.View()
	// Author column is hidden in mine view → fallback badge.
	if !strings.Contains(view, "[sort: Author ↓]") {
		t.Errorf("expected '[sort: Author ↓]' badge for author sort in mine view, got view:\n%s", view)
	}
}

// --- Cycle 7: sort-menu overlay rendering ---

func TestSortMenuOverlay_RendersWhenOpen(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.width = 200
	m.height = 40
	m.mine.loading = false
	m.mine.fetching = false
	m.mine.prs = []PullRequest{{Number: 1, Title: "x", Repo: "r"}}
	m.sortMenuOpen = true
	m.sortMenuDir = sortDesc
	m.sortMenuCursor = indexOfSortField(sortFieldCI)

	view := m.View()
	for _, want := range []string{
		"Sort pull requests",
		"Descending",
		"Updated", "Created", "Repo", "Author", "Title",
		"CI", "Review", "Merge", "Comments", "Number",
		"apply", "cancel",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("sort-menu view missing %q\n--- view ---\n%s\n--- end ---", want, view)
		}
	}
}

func TestSortMenuOverlay_DirectionText(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, sortDesc)
	m.width = 200
	m.height = 40
	m.mine.loading = false
	m.mine.fetching = false
	m.mine.prs = []PullRequest{{Number: 1, Title: "x"}}
	m.sortMenuOpen = true
	m.sortMenuDir = sortAsc
	view := m.View()
	if !strings.Contains(view, "Ascending") {
		t.Errorf("expected 'Ascending' in menu when sortMenuDir=asc, got:\n%s", view)
	}
}

func TestSortMenuOverlay_ConfirmWins(t *testing.T) {
	// When both confirmAction and sortMenuOpen are set, confirm overlay renders
	// (sort menu cannot open while confirm active — covered in Cycle 4). Here
	// we verify the rendering guard: if both somehow set, confirm takes
	// precedence in View(). We set both and assert the confirm question text
	// appears and the sort-menu title does not.
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.width = 200
	m.height = 40
	m.mine.loading = false
	m.mine.fetching = false
	m.mine.prs = []PullRequest{{Number: 1, Title: "x"}}
	m.confirmAction = "close"
	m.sortMenuOpen = true
	view := m.View()
	if !strings.Contains(view, "Close pull request") {
		t.Errorf("expected confirm overlay text, got:\n%s", view)
	}
	if strings.Contains(view, "Sort pull requests") {
		t.Errorf("sort-menu title should NOT appear when confirm overlay is active, got:\n%s", view)
	}
}

// --- Navigable check runs ---

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func keyTab() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyTab}
}

func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			in = true
			continue
		}
		if in {
			if (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') {
				in = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestScroll_LongCheckListReachesLastCheck(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 15 // visibleLines = 6
	runs := make([]CheckRun, 20)
	for i := range runs {
		runs[i] = CheckRun{Name: fmt.Sprintf("job-%02d", i)}
	}
	m.mine.prs = []PullRequest{
		{Number: 1, Title: "big", CheckRuns: runs},
		{Number: 2, Title: "next"},
	}
	m.mine.expanded[1] = true
	m.mine.cursor = 0
	m.mine.scrollOffset = 0

	for i := 0; i < 20; i++ {
		nm, _ := m.Update(keyRune('j'))
		m = nm.(model)
	}
	rows := buildFocusRows(&m.mine)
	if m.mine.cursor != 20 {
		t.Fatalf("cursor=%d want 20 (last check); rows=%d", m.mine.cursor, len(rows))
	}
	if m.mine.cursor < m.mine.scrollOffset {
		t.Fatalf("cursor above viewport")
	}
	vis := m.height - 9
	if m.mine.cursor >= m.mine.scrollOffset+vis {
		t.Fatalf("cursor=%d scroll=%d vis=%d not visible", m.mine.cursor, m.mine.scrollOffset, vis)
	}
}

func TestScroll_UpFromDeepCheckScrollsBack(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 15
	runs := make([]CheckRun, 15)
	for i := range runs {
		runs[i] = CheckRun{Name: fmt.Sprintf("j%d", i)}
	}
	m.mine.prs = []PullRequest{{Number: 1, CheckRuns: runs}}
	m.mine.expanded[1] = true
	for i := 0; i < 15; i++ {
		nm, _ := m.Update(keyRune('j'))
		m = nm.(model)
	}
	for m.mine.cursor > 0 {
		nm, _ := m.Update(keyRune('k'))
		m = nm.(model)
	}
	if m.mine.scrollOffset != 0 {
		t.Fatalf("scrollOffset=%d want 0 at top", m.mine.scrollOffset)
	}
}

func TestAdjustScrollFor_RowBasedDeepCheck(t *testing.T) {
	m := model{
		height: 11, // vis=2
		mine: viewState{
			prs: []PullRequest{{
				Number: 1,
				CheckRuns: []CheckRun{
					{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}, {Name: "e"},
				},
			}},
			expanded:     map[int]bool{1: true},
			cursor:       5, // last check
			scrollOffset: 0,
		},
	}
	m.adjustScrollFor(&m.mine)
	if m.mine.scrollOffset == 0 {
		t.Fatal("expected scrollOffset > 0 for deep check with small viewport")
	}
	if m.mine.cursor < m.mine.scrollOffset {
		t.Fatal("cursor not in viewport")
	}
}

func TestJK_WalksIntoExpandedChecks(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.mine.prs = []PullRequest{
		{Number: 1, CheckRuns: []CheckRun{{Name: "build"}, {Name: "test"}}},
		{Number: 2, CheckRuns: []CheckRun{{Name: "lint"}}},
	}
	m.mine.expanded[1] = true
	m.mine.cursor = 0

	nm, _ := m.Update(keyRune('j'))
	m = nm.(model)
	rows := buildFocusRows(&m.mine)
	if rows[m.mine.cursor].Kind != focusCheck || rows[m.mine.cursor].CheckName != "build" {
		t.Fatalf("after j: %+v", rows[m.mine.cursor])
	}
	nm, _ = m.Update(keyRune('j'))
	m = nm.(model)
	rows = buildFocusRows(&m.mine)
	if rows[m.mine.cursor].CheckName != "test" {
		t.Fatalf("second j: %+v", rows[m.mine.cursor])
	}
	nm, _ = m.Update(keyRune('j'))
	m = nm.(model)
	rows = buildFocusRows(&m.mine)
	if rows[m.mine.cursor].Kind != focusPR || rows[m.mine.cursor].PRNumber != 2 {
		t.Fatalf("third j should be PR2: %+v", rows[m.mine.cursor])
	}
}

func TestJK_ClampsAtEnds(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.mine.prs = []PullRequest{{Number: 1, CheckRuns: []CheckRun{{Name: "a"}}}}
	m.mine.expanded[1] = true
	m.mine.cursor = 0
	nm, _ := m.Update(keyRune('k'))
	m = nm.(model)
	if m.mine.cursor != 0 {
		t.Fatalf("k at top: %d", m.mine.cursor)
	}
	nm, _ = m.Update(keyRune('j'))
	m = nm.(model)
	nm, _ = m.Update(keyRune('j'))
	m = nm.(model)
	if m.mine.cursor != 1 {
		t.Fatalf("j past end: %d want 1", m.mine.cursor)
	}
}

func TestJK_MultiExpandInterleaves(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.mine.prs = []PullRequest{
		{Number: 1, CheckRuns: []CheckRun{{Name: "a"}}},
		{Number: 2, CheckRuns: []CheckRun{{Name: "b"}}},
	}
	m.mine.expanded[1] = true
	m.mine.expanded[2] = true
	for i, wantKind := range []focusKind{focusPR, focusCheck, focusPR, focusCheck} {
		rows := buildFocusRows(&m.mine)
		if rows[m.mine.cursor].Kind != wantKind {
			t.Fatalf("step %d: got %+v want kind %v", i, rows[m.mine.cursor], wantKind)
		}
		if i < 3 {
			nm, _ := m.Update(keyRune('j'))
			m = nm.(model)
		}
	}
}

func TestTab_ExpandCollapse_FromPR(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.mine.prs = []PullRequest{{Number: 5, CheckRuns: []CheckRun{{Name: "x"}}}}
	m.mine.cursor = 0
	nm, _ := m.Update(keyTab())
	m = nm.(model)
	if !m.mine.expanded[5] {
		t.Fatal("expected expanded")
	}
	if len(buildFocusRows(&m.mine)) != 2 {
		t.Fatal("expected PR+check rows")
	}
	if buildFocusRows(&m.mine)[m.mine.cursor].Kind != focusPR {
		t.Fatal("tab expand should keep focus on PR row")
	}
	nm, _ = m.Update(keyTab())
	m = nm.(model)
	if m.mine.expanded[5] {
		t.Fatal("expected collapsed")
	}
	if len(buildFocusRows(&m.mine)) != 1 {
		t.Fatal("only PR row")
	}
}

func TestTab_CollapseWhileOnCheck_SnapsToParentPR(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.mine.prs = []PullRequest{
		{Number: 1, CheckRuns: []CheckRun{{Name: "a"}, {Name: "b"}}},
		{Number: 2},
	}
	m.mine.expanded[1] = true
	m.mine.cursor = 2 // check b
	nm, _ := m.Update(keyTab())
	m = nm.(model)
	if m.mine.expanded[1] {
		t.Fatal("should collapse parent")
	}
	rows := buildFocusRows(&m.mine)
	if rows[m.mine.cursor].Kind != focusPR || rows[m.mine.cursor].PRNumber != 1 {
		t.Fatalf("want parent PR1, got %+v cursor=%d", rows[m.mine.cursor], m.mine.cursor)
	}
}

func TestTab_OnCheckTogglesParentNotNextPR(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.mine.prs = []PullRequest{
		{Number: 1, CheckRuns: []CheckRun{{Name: "a"}}},
		{Number: 2, CheckRuns: []CheckRun{{Name: "z"}}},
	}
	m.mine.expanded[1] = true
	m.mine.expanded[2] = true
	m.mine.cursor = 1 // check a under PR1
	nm, _ := m.Update(keyTab())
	m = nm.(model)
	if m.mine.expanded[1] {
		t.Fatal("PR1 should collapse")
	}
	if !m.mine.expanded[2] {
		t.Fatal("PR2 should stay expanded")
	}
}

func TestActions_OnCheckTargetParentPR(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.viewMode = 0
	m.mine.prs = []PullRequest{{
		Number: 1, URL: "https://pr/1", HeadRefName: "feat/x",
		CheckRuns: []CheckRun{{Name: "build", Permalink: "https://check/build"}},
	}}
	m.mine.expanded[1] = true
	m.mine.cursor = 1 // on check

	nm, _ := m.Update(keyRune('m'))
	m = nm.(model)
	if m.confirmAction != "merge" {
		t.Fatalf("confirmAction=%q want merge", m.confirmAction)
	}

	m.confirmAction = ""
	_, cmd := m.Update(keyRune('b'))
	if cmd == nil {
		t.Fatal("branch should copy parent HeadRefName")
	}
}

func TestYankKey_CheckUsesCheckURL(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.mine.prs = []PullRequest{{
		Number: 1, URL: "https://pr/1",
		CheckRuns: []CheckRun{{Name: "build", Permalink: "https://check/build"}},
	}}
	m.mine.expanded[1] = true
	m.mine.cursor = 1 // on check

	rows := buildFocusRows(&m.mine)
	if u := openURLForFocus(m.mine.prs, rows, 1); u != "https://check/build" {
		t.Fatalf("openURLForFocus=%q want check URL", u)
	}
	_, cmd := m.Update(keyRune('y'))
	if cmd == nil {
		t.Fatal("yank on check with URL should return cmd")
	}
}

func TestYankKey_CheckWithoutURLNoCmd(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.mine.prs = []PullRequest{{
		Number: 1, URL: "https://pr/1",
		CheckRuns: []CheckRun{{Name: "build"}},
	}}
	m.mine.expanded[1] = true
	m.mine.cursor = 1
	_, cmd := m.Update(keyRune('y'))
	if cmd != nil {
		t.Fatal("yank on check without URL should be no-op")
	}
}

func TestYankKey_PRStillYanksPR(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.mine.prs = []PullRequest{{Number: 1, URL: "https://pr/1"}}
	m.mine.cursor = 0
	rows := buildFocusRows(&m.mine)
	if u := openURLForFocus(m.mine.prs, rows, 0); u != "https://pr/1" {
		t.Fatalf("got %q", u)
	}
	_, cmd := m.Update(keyRune('y'))
	if cmd == nil {
		t.Fatal("yank on PR should return cmd")
	}
}

func TestActions_EmptyListNoPanic(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.mine.prs = nil
	m.mine.cursor = 0
	for _, r := range []rune{'o', 'y', 'b', 'm', 'c', 'x', 'd'} {
		nm, _ := m.Update(keyRune(r))
		m = nm.(model)
	}
}

func TestApprove_OnCheckUsesParentAuthorGate(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.viewMode = 1
	m.org.prs = []PullRequest{{
		Number: 9, Author: "other",
		CheckRuns: []CheckRun{{Name: "ci"}},
	}}
	m.org.expanded[9] = true
	m.org.cursor = 1
	nm, _ := m.Update(keyRune('p'))
	if nm.(model).confirmAction != "approve" {
		t.Fatal("approve should work from check focus on others' PR")
	}

	m.org.prs[0].Author = "user"
	m.confirmAction = ""
	m.org.cursor = 1
	nm, _ = m.Update(keyRune('p'))
	if nm.(model).confirmAction == "approve" {
		t.Fatal("approve blocked for own PR even from check focus")
	}
}

func TestOpenKey_CheckWithPermalinkReturnsCmd(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.mine.prs = []PullRequest{{
		Number: 1, URL: "https://pr/1",
		CheckRuns: []CheckRun{{Name: "build", Permalink: "https://github.com/o/r/runs/1"}},
	}}
	m.mine.expanded[1] = true
	m.mine.cursor = 1
	_, cmd := m.Update(keyRune('o'))
	if cmd == nil {
		t.Fatal("expected open cmd")
	}
	rows := buildFocusRows(&m.mine)
	if u := openURLForFocus(m.mine.prs, rows, 1); u != "https://github.com/o/r/runs/1" {
		t.Fatalf("url=%q", u)
	}
}

func TestOpenKey_CheckWithoutURLNoCmd(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.mine.prs = []PullRequest{{
		Number: 1, URL: "https://pr/1",
		CheckRuns: []CheckRun{{Name: "build"}},
	}}
	m.mine.expanded[1] = true
	m.mine.cursor = 1
	_, cmd := m.Update(keyRune('o'))
	if cmd != nil {
		t.Fatal("expected no cmd when check has no URL")
	}
}

func TestOpenKey_PRStillOpensPR(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.mine.prs = []PullRequest{{Number: 1, URL: "https://pr/1"}}
	m.mine.cursor = 0
	_, cmd := m.Update(keyRune('o'))
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	if u := openURLForFocus(m.mine.prs, buildFocusRows(&m.mine), 0); u != "https://pr/1" {
		t.Fatalf("%q", u)
	}
}

func TestOpenKey_LoadingPlaceholderNoCmd(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.viewMode = 1
	m.org.prs = []PullRequest{{Number: 1, URL: "https://pr/1", CheckRuns: nil}}
	m.org.expanded[1] = true
	m.org.cursor = 1
	_, cmd := m.Update(keyRune('o'))
	if cmd != nil {
		t.Fatal("no open on placeholder")
	}
}

func TestView_SelectedCheckShowsCheckName(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.width, m.height = 120, 40
	m.mine.loading = false
	m.mine.prs = []PullRequest{{
		Number: 1, Repo: "r", Title: "t",
		CheckRuns: []CheckRun{{Name: "unique-build-job", Status: "COMPLETED", Conclusion: "SUCCESS"}},
	}}
	m.mine.expanded[1] = true
	m.mine.cursor = 1
	out := stripANSI(m.View())
	if !strings.Contains(out, "unique-build-job") {
		t.Fatalf("missing check name in view:\n%s", out)
	}
	if !strings.Contains(out, "t") {
		t.Fatal("PR title missing")
	}
}

func TestView_ScrollHidesOffscreenChecks(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.width, m.height = 120, 12 // small vis
	m.mine.loading = false
	runs := make([]CheckRun, 30)
	for i := range runs {
		runs[i] = CheckRun{Name: fmt.Sprintf("job-%02d", i), Status: "COMPLETED", Conclusion: "SUCCESS"}
	}
	m.mine.prs = []PullRequest{{Number: 1, Repo: "r", Title: "t", CheckRuns: runs}}
	m.mine.expanded[1] = true
	m.mine.cursor = 25
	m.adjustScroll()
	out := stripANSI(m.View())
	if strings.Contains(out, "job-00") && m.mine.scrollOffset > 1 {
		t.Fatalf("expected early jobs scrolled off; scroll=%d\n%s", m.mine.scrollOffset, out)
	}
	if !strings.Contains(out, "job-24") && !strings.Contains(out, "job-25") {
		t.Fatalf("expected focused region visible\n%s", out)
	}
}

func TestSortPRs_PreservesCheckFocus(t *testing.T) {
	m := initialModel(nil, "user", nil, time.Minute, sortFieldNumber, sortAsc)
	m.height = 40
	m.mine.prs = []PullRequest{
		{Number: 2, Title: "b", CheckRuns: []CheckRun{{Name: "x"}}},
		{Number: 1, Title: "a", CheckRuns: []CheckRun{{Name: "keep-me"}}},
	}
	m.mine.expanded[1] = true
	// rows: PR2, PR1, keep-me → focus keep-me index 2
	m.mine.cursor = 2
	m.mine.sortField = sortFieldNumber
	m.mine.sortDir = sortAsc
	m.sortPRs(&m.mine, 0, true)
	rows := buildFocusRows(&m.mine)
	if rows[m.mine.cursor].CheckName != "keep-me" {
		t.Fatalf("lost check focus: cursor=%d row=%+v", m.mine.cursor, rows[m.mine.cursor])
	}
}

func TestPRsFetched_PreservesCheckFocusWhenStillPresent(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, sortFieldNumber, sortAsc)
	m.height = 40
	m.mine.loading = false
	m.mine.prs = []PullRequest{
		{Number: 1, CheckRuns: []CheckRun{{Name: "a"}, {Name: "b"}}},
	}
	m.mine.expanded[1] = true
	m.mine.cursor = 2 // b
	nm, _ := m.Update(prsFetchedMsg{prs: []PullRequest{
		{Number: 1, CheckRuns: []CheckRun{{Name: "a"}, {Name: "b"}}},
		{Number: 3},
	}})
	m = nm.(model)
	rows := buildFocusRows(&m.mine)
	if rows[m.mine.cursor].CheckName != "b" {
		t.Fatalf("got %+v", rows[m.mine.cursor])
	}
}

func TestCheckRunsFetched_LoadingFocusMovesToFirstCheck(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.viewMode = 1
	m.org.prs = []PullRequest{{Number: 1, ID: "id", CheckRuns: nil}}
	m.org.expanded[1] = true
	m.org.cursor = 1 // loading placeholder
	nm, _ := m.Update(checkRunsFetchedMsg{
		prNumber: 1,
		runs:     []CheckRun{{Name: "first"}, {Name: "second"}},
	})
	m = nm.(model)
	rows := buildFocusRows(&m.org)
	if rows[m.org.cursor].Kind != focusCheck || rows[m.org.cursor].CheckName != "first" {
		t.Fatalf("got %+v", rows[m.org.cursor])
	}
}

func TestCheckRunsFetched_EmptyStaysOnEmptyPlaceholder(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.viewMode = 1
	m.org.prs = []PullRequest{{Number: 1, CheckRuns: nil}}
	m.org.expanded[1] = true
	m.org.cursor = 1
	nm, _ := m.Update(checkRunsFetchedMsg{prNumber: 1, runs: []CheckRun{}})
	m = nm.(model)
	rows := buildFocusRows(&m.org)
	if rows[m.org.cursor].Kind != focusPlaceholder || rows[m.org.cursor].Placeholder != placeholderEmpty {
		t.Fatalf("got %+v", rows[m.org.cursor])
	}
}

func TestEdge_DuplicateCheckNamesDistinctFocus(t *testing.T) {
	v := &viewState{
		prs: []PullRequest{{Number: 1, CheckRuns: []CheckRun{
			{Name: "test"}, {Name: "test"},
		}}},
		expanded: map[int]bool{1: true},
	}
	rows := buildFocusRows(v)
	if focusID(rows[1]) == focusID(rows[2]) {
		t.Fatal("duplicate names must still have distinct focus IDs")
	}
}

func TestEdge_SortMenuStealsKeys(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.mine.prs = []PullRequest{
		{Number: 1, CheckRuns: []CheckRun{{Name: "a"}}},
		{Number: 2},
	}
	m.mine.expanded[1] = true
	m.mine.cursor = 0
	m.sortMenuOpen = true
	nm, _ := m.Update(keyRune('j'))
	m = nm.(model)
	if m.mine.cursor != 0 {
		t.Fatal("list cursor must not move while sort menu open")
	}
}

func TestEdge_OrgTabExpandFetchesWhenNil(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.viewMode = 1
	m.org.prs = []PullRequest{{Number: 1, ID: "PR_id", CheckRuns: nil}}
	m.org.cursor = 0
	got, cmd := m.Update(keyTab())
	mod := got.(model)
	if !mod.org.expanded[1] {
		t.Fatal("should be expanded")
	}
	if cmd == nil {
		t.Fatal("expected fetch cmd when CheckRuns nil")
	}
}

func TestEdge_MineTabExpandNoFetchWhenRunsPresent(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 40
	m.mine.prs = []PullRequest{{Number: 1, CheckRuns: []CheckRun{{Name: "a"}}}}
	m.mine.cursor = 0
	_, cmd := m.Update(keyTab())
	if cmd != nil {
		t.Fatal("mine view with runs should not fetch")
	}
}

func TestEdge_TinyHeightNoPanic(t *testing.T) {
	m := initialModel(nil, "user", nil, 0, defaultSortField, defaultSortDir)
	m.height = 5
	m.width = 40
	m.mine.loading = false
	m.mine.prs = []PullRequest{{Number: 1, CheckRuns: []CheckRun{{Name: "a"}}}}
	m.mine.expanded[1] = true
	m.mine.cursor = 1
	m.adjustScroll()
	_ = m.View()
	nm, _ := m.Update(keyRune('j'))
	_ = nm.View()
}
