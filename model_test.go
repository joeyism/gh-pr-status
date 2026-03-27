package main

import (
	"strings"
	"testing"

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
	m := initialModel(nil, "user", nil, 0)
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
		m := initialModel(nil, "user", nil, 0)
		m.mine.prs = prs
		m.viewMode = 0

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
		_, cmd := m.Update(msg)
		if cmd == nil {
			t.Fatal("expected command to be returned")
		}
	})

	t.Run("with overlay confirm instead of yank", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0)
		m.mine.prs = prs
		m.confirmAction = "approve"

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
		newModel, _ := m.Update(msg)
		mod := newModel.(model)
		if mod.confirmAction != "" {
			t.Fatal("confirmAction should be cleared")
		}
		if strings.Contains(mod.flash, "URL copied") {
			t.Fatal("flash should not indicate yank")
		}
	})
}

func TestUpdateToggleView(t *testing.T) {
	m := initialModel(nil, "user", nil, 0)
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
	m := initialModel(nil, "user", nil, 0)
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
	m := initialModel(nil, "user", nil, 0)
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
	m := initialModel(nil, "user", nil, 0)

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
