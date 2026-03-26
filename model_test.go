package main

import (
	"fmt"
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
		{os: "freebsd", wantCmd: "", wantArgs: nil, wantErr: true},
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
			for i := range args {
				if args[i] != tc.wantArgs[i] {
					t.Fatalf("clipboardCommand(%q) args[%d] = %q, want %q", tc.os, i, args[i], tc.wantArgs[i])
				}
			}
		})
	}
}

func TestUpdateYankKey(t *testing.T) {
	prs := []PullRequest{
		{Number: 1, Title: "PR 1", URL: "https://github.com/org/repo/pull/1"},
	}

	t.Run("no overlay yank", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0)
		m.prs = prs
		m.confirmAction = ""

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
		newModel, cmd := m.Update(msg)

		mod := newModel.(model)
		if mod.confirmAction != "" {
			t.Fatalf("confirmAction should be empty, got %q", mod.confirmAction)
		}
		if cmd == nil {
			t.Fatal("expected command to be returned")
		}
		// We can't easily inspect the cmd content, but we know it's not nil.
	})

	t.Run("with overlay confirm instead of yank", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0)
		m.prs = prs
		m.confirmAction = "merge"

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
		newModel, cmd := m.Update(msg)

		mod := newModel.(model)
		if mod.confirmAction != "" {
			t.Fatalf("confirmAction should be cleared by 'y', got %q", mod.confirmAction)
		}
		if cmd == nil {
			t.Fatal("expected merge command to be returned")
		}

		// Ensure no clipboard flash message is set yet
		if strings.Contains(mod.flash, "URL copied") {
			t.Fatal("flash message should not indicate clipboard copy")
		}
	})

	t.Run("empty prs yank no-op", func(t *testing.T) {
		m := initialModel(nil, "user", nil, 0)
		m.prs = nil
		m.confirmAction = ""

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
		_, cmd := m.Update(msg)

		if cmd != nil {
			t.Fatal("expected nil command for empty PRs")
		}
	})
}

func TestUpdateClipboardMsg(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := model{}
		msg := clipboardMsg{err: nil}
		newModel, cmd := m.Update(msg)
		mod := newModel.(model)

		if !strings.Contains(mod.flash, "URL copied ✓") {
			t.Fatalf("expected success flash message, got %q", mod.flash)
		}
		if cmd == nil {
			t.Fatal("expected tick command to clear flash")
		}
	})

	t.Run("failure", func(t *testing.T) {
		m := model{}
		errMsg := "xclip: not found"
		msg := clipboardMsg{err: fmt.Errorf("%s", errMsg)}
		newModel, cmd := m.Update(msg)
		mod := newModel.(model)

		if !strings.Contains(mod.flash, "Copy failed") || !strings.Contains(mod.flash, errMsg) {
			t.Fatalf("expected failure flash message with error, got %q", mod.flash)
		}
		if cmd == nil {
			t.Fatal("expected tick command to clear flash")
		}
	})
}
