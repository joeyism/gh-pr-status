package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
	"github.com/shurcooL/githubv4"
)

type prsFetchedMsg struct {
	prs []PullRequest
	err error
}

type commentPostedMsg struct{ err error }
type prClosedMsg struct{ err error }
type prMergedMsg struct{ err error }
type clipboardMsg struct{ err error }

type clearFlashMsg struct{}

type tickMsg time.Time

type model struct {
	prs            []PullRequest
	cursor         int
	expanded       map[int]bool
	loading        bool
	fetching       bool
	err            error
	lastUpdated    time.Time
	pollInterval   time.Duration
	changedAt      map[int]time.Time
	previousStatus map[int]string

	focused       bool
	confirmAction string // "cursor" | "close" | "merge" | "" (none)
	flash         string
	width         int
	height        int

	client   *githubv4.Client
	username string
	orgs     []string
}

func initialModel(client *githubv4.Client, username string, orgs []string, pollInterval time.Duration) model {
	return model{
		focused:        true,
		loading:        true,
		fetching:       true,
		expanded:       make(map[int]bool),
		changedAt:      make(map[int]time.Time),
		previousStatus: make(map[int]string),
		pollInterval:   pollInterval,
		client:         client,
		username:       username,
		orgs:           orgs,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.fetchPRsCmd(), m.tickCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.FocusMsg:
		m.focused = true

	case tea.BlurMsg:
		m.focused = false

	case tea.KeyMsg:
		// When the confirmation overlay is open, intercept all keys.
		if m.confirmAction != "" {
			switch msg.String() {
			case "y", "enter":
				action := m.confirmAction
				m.confirmAction = ""
				pr := m.prs[m.cursor]
				switch action {
				case "cursor":
					return m, m.addCommentCmd(pr.ID, "@cursor review")
				case "close":
					return m, m.closePRCmd(pr.ID)
				case "merge":
					return m, m.mergePRCmd(pr.ID)
				}
			case "n", "esc":
				m.confirmAction = ""
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.prs)-1 {
				m.cursor++
			}
		case "tab":
			if len(m.prs) > 0 {
				pr := m.prs[m.cursor]
				m.expanded[pr.Number] = !m.expanded[pr.Number]
			}
		case "o":
			if len(m.prs) > 0 {
				return m, openBrowserCmd(m.prs[m.cursor].URL)
			}
		case "c":
			if len(m.prs) > 0 {
				m.confirmAction = "cursor"
			}
		case "x":
			if len(m.prs) > 0 {
				m.confirmAction = "close"
			}
		case "m":
			if len(m.prs) > 0 {
				m.confirmAction = "merge"
			}
		case "r":
			if !m.fetching {
				m.fetching = true
				return m, m.fetchPRsCmd()
			}
		case "y":
			if len(m.prs) > 0 {
				return m, copyToClipboardCmd(m.prs[m.cursor].URL)
			}
		}

	case commentPostedMsg:
		if msg.err != nil {
			m.flash = flashFailureMsg.Render(fmt.Sprintf("Error posting comment: %v", msg.err))
		} else {
			m.flash = flashSuccessMsg.Render("Comment posted ✓")
		}
		return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} })

	case prClosedMsg:
		if msg.err != nil {
			m.flash = flashFailureMsg.Render(fmt.Sprintf("Error closing PR: %v", msg.err))
			return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} })
		}
		m.flash = flashSuccessMsg.Render("PR closed ✓")
		m.fetching = true
		return m, tea.Batch(
			tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} }),
			m.fetchPRsCmd(),
		)

	case prMergedMsg:
		if msg.err != nil {
			m.flash = flashFailureMsg.Render(fmt.Sprintf("Error merging PR: %v", msg.err))
			return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} })
		}
		m.flash = flashSuccessMsg.Render("PR merged ✓")
		m.fetching = true
		return m, tea.Batch(
			tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} }),
			m.fetchPRsCmd(),
		)

	case clipboardMsg:
		if msg.err != nil {
			m.flash = flashFailureMsg.Render(fmt.Sprintf("Copy failed: %v", msg.err))
		} else {
			m.flash = flashSuccessMsg.Render("URL copied ✓")
		}
		return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} })

	case prsFetchedMsg:
		m.fetching = false
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		now := time.Now()
		m.lastUpdated = now
		statusChanged := false
		for _, pr := range msg.prs {
			// Note: TotalComments not included in key intentionally — a new comment
			// without resolving/creating a thread isn't actionable enough to flash.
			newKey := fmt.Sprintf("%s|%s|%d|%s", pr.CheckStatus, pr.ReviewDecision, pr.UnresolvedThreads, pr.Mergeable)
			if oldKey, ok := m.previousStatus[pr.Number]; ok && oldKey != newKey {
				m.changedAt[pr.Number] = now
				statusChanged = true
			}
			m.previousStatus[pr.Number] = newKey
		}

		m.prs = msg.prs
		if m.cursor >= len(m.prs) && len(m.prs) > 0 {
			m.cursor = len(m.prs) - 1
		}
		if statusChanged {
			return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} })
		}

	case tickMsg:
		cmds := []tea.Cmd{m.tickCmd()}
		if !m.fetching {
			m.fetching = true
			cmds = append(cmds, m.fetchPRsCmd())
		}
		return m, tea.Batch(cmds...)

	case clearFlashMsg:
		m.changedAt = make(map[int]time.Time)
		m.flash = ""
	}

	return m, nil
}

func (m model) View() string {
	if m.loading {
		return headerStyle.Render("gh-pr-status") + "\n\nLoading PRs...\n"
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render("gh-pr-status"))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(failureStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	if len(m.prs) == 0 {
		b.WriteString("No open PRs found.\n")
	}

	now := time.Now()

	// Compute dynamic repo column width from actual data
	repoWidth := 8
	for _, pr := range m.prs {
		if len(pr.Repo) > repoWidth {
			repoWidth = len(pr.Repo)
		}
	}
	repoWidth++ // one space of padding
	dynRepoStyle := repoColStyle.Width(repoWidth)

	// Column header — gap = cursor(3) + repo(dynamic) + space(1)
	repoGap := strings.Repeat(" ", 3+repoWidth+1)
	b.WriteString(columnHeaderStyle.Render(fmt.Sprintf("%s%-50s %-9s %-12s %-10s",
		repoGap, "", "CI", "Review", "Merge")))
	b.WriteString("\n\n")
	for i, pr := range m.prs {
		cursor := "   "
		if i == m.cursor {
			cursor = cursorStyle.Render("┃") + "  "
		}

		title := pr.Title
		if pr.IsDraft {
			title = "DRAFT " + title
		}
		if len(title) > 48 {
			title = title[:47] + "…"
		}
		titleRendered := titleColStyle.Render(title)
		if pr.IsDraft {
			titleRendered = titleColStyle.Foreground(lipgloss.Color("240")).Italic(true).Render(title)
		}

		line := fmt.Sprintf("%s%s %s %s %s %s %s",
			cursor,
			dynRepoStyle.Render(pr.Repo),
			titleRendered,
			ciColStyle.Render(formatCIStatus(pr.CheckStatus)),
			reviewColStyle.Render(formatReviewStatus(pr.ReviewDecision)),
			mergeColStyle.Render(formatMergeable(pr.Mergeable)),
			formatComments(pr.TotalComments, pr.UnresolvedThreads, pr.TotalThreads),
		)

		if i == m.cursor {
			line = selectedStyle.Render(line)
		}

		if t, ok := m.changedAt[pr.Number]; ok && now.Sub(t) < 3*time.Second {
			if pr.CheckStatus == "SUCCESS" {
				line = flashSuccessStyle.Render(line)
			} else if pr.CheckStatus == "FAILURE" || pr.CheckStatus == "ERROR" {
				line = flashFailureStyle.Render(line)
			}
		}

		b.WriteString(line + "\n")

		if m.expanded[pr.Number] {
			treeIndent := "   " + strings.Repeat(" ", repoWidth) + " "
			if len(pr.CheckRuns) == 0 {
				b.WriteString(treeIndent + treeStyle.Render("└─") + " " + dimStyle.Render("no check runs") + "\n")
			} else {
				for j, cr := range pr.CheckRuns {
					branch := "├─"
					if j == len(pr.CheckRuns)-1 {
						branch = "└─"
					}
					b.WriteString(treeIndent + treeStyle.Render(branch) + " " + formatCheckRun(cr) + "\n")
				}
			}
		}
	}

	ago := ""
	if !m.lastUpdated.IsZero() {
		ago = fmt.Sprintf("Updated %s ago", time.Since(m.lastUpdated).Round(time.Second))
	}
	fetchIndicator := ""
	if m.fetching {
		fetchIndicator = " (refreshing...)"
	}
	flashLine := ""
	if m.flash != "" {
		flashLine = "\n" + m.flash
	}
	b.WriteString(footerStyle.Render(fmt.Sprintf(
		"\n%s%s%s                    j/k: nav • tab: expand • o: open • y: yank • r: refresh • c: cursor review • x: close • m: merge • q: quit",
		ago, fetchIndicator, flashLine,
	)))

	out := b.String()

	// Wrap everything in a focus-aware border.
	borderColor := lipgloss.Color("240")
	if m.focused {
		borderColor = lipgloss.Color("#BD93F9")
	}
	innerW := m.width - 2
	innerH := m.height - 2
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}
	out = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(innerW).
		Height(innerH).
		Render(out)

	if m.confirmAction != "" && len(m.prs) > 0 {
		pr := m.prs[m.cursor]
		title := pr.Title
		if len(title) > 40 {
			title = title[:39] + "…"
		}

		var questionText string
		var yesBtnColor lipgloss.Color
		switch m.confirmAction {
		case "close":
			questionText = "Close pull request"
			yesBtnColor = lipgloss.Color("#FF5555")
		case "merge":
			questionText = "Squash and merge"
			yesBtnColor = lipgloss.Color("#50FA7B")
		default: // "cursor"
			questionText = "Request @cursor review on"
			yesBtnColor = lipgloss.Color("#50FA7B")
		}

		question := overlayTextStyle.Render(questionText)
		prInfo := overlayTextStyle.Bold(true).Render(fmt.Sprintf("%q (#%d)?", title, pr.Number))
		yesBtn := overlayTextStyle.Bold(true).Foreground(yesBtnColor).Render("[y]es")
		noBtn := overlayTextStyle.Bold(true).Foreground(lipgloss.Color("#FF5555")).Render("[n]o")
		buttons := yesBtn + "    " + noBtn

		content := lipgloss.JoinVertical(lipgloss.Left, question, prInfo, "", buttons)
		box := overlayBoxStyle.Render(content)
		out = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("236")),
		)
	}

	return out
}

func formatCIStatus(status string) string {
	switch status {
	case "SUCCESS":
		return successStyle.Render("✓ pass")
	case "FAILURE":
		return failureStyle.Render("✗ fail")
	case "PENDING":
		return runningStyle.Render("● run")
	case "ERROR":
		return failureStyle.Render("✗ err")
	default:
		return dimStyle.Render("○ --")
	}
}

func formatReviewStatus(decision string) string {
	switch decision {
	case "APPROVED":
		return successStyle.Render("✓ approved")
	case "CHANGES_REQUESTED":
		return failureStyle.Render("✗ changes")
	case "REVIEW_REQUIRED":
		return pendingStyle.Render("~ pending")
	default:
		return dimStyle.Render("- none")
	}
}

func formatMergeable(status string) string {
	switch status {
	case "MERGEABLE":
		return successStyle.Render("+ ready")
	case "CONFLICTING":
		return failureStyle.Render("! conflict")
	default:
		return dimStyle.Render("? unknown")
	}
}

func formatComments(total, unresolved, totalThreads int) string {
	if total == 0 && totalThreads == 0 {
		return dimStyle.Render("💬 0")
	}
	if totalThreads == 0 && total > 0 {
		return dimStyle.Render(fmt.Sprintf("💬 %d", total))
	}
	if unresolved == -1 {
		return fmt.Sprintf("%s %s",
			dimStyle.Render(fmt.Sprintf("💬 %d", total)),
			pendingStyle.Render("(? unresolved)"),
		)
	}
	if unresolved == 0 {
		return fmt.Sprintf("%s %s",
			dimStyle.Render(fmt.Sprintf("💬 %d", total)),
			successStyle.Render("(0 unresolved)"),
		)
	}
	return fmt.Sprintf("%s %s",
		dimStyle.Render(fmt.Sprintf("💬 %d", total)),
		pendingStyle.Render(fmt.Sprintf("(%d unresolved)", unresolved)),
	)
}

func formatCheckRun(cr CheckRun) string {
	s := checkRunStyle(cr.Status, cr.Conclusion)
	symbol := "●"
	label := strings.ToLower(cr.Status)
	if cr.Status == "COMPLETED" {
		switch cr.Conclusion {
		case "SUCCESS":
			symbol, label = "✓", "passed"
		case "FAILURE":
			symbol, label = "✗", "failed"
		case "SKIPPED":
			symbol, label = "→", "skipped"
		case "NEUTRAL":
			symbol, label = "–", "neutral"
		case "CANCELLED":
			symbol, label = "✕", "cancelled"
		default:
			label = cr.Conclusion
		}
	}
	return s.Render(fmt.Sprintf("%s %s — %s", symbol, cr.Name, label))
}

func (m model) addCommentCmd(subjectID, body string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := addPRComment(context.Background(), client, subjectID, body)
		return commentPostedMsg{err: err}
	}
}

func (m model) closePRCmd(prID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := closePR(context.Background(), client, prID)
		return prClosedMsg{err: err}
	}
}

func (m model) mergePRCmd(prID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := mergePR(context.Background(), client, prID)
		return prMergedMsg{err: err}
	}
}

func (m model) fetchPRsCmd() tea.Cmd {
	client, username, orgs := m.client, m.username, m.orgs
	return func() tea.Msg {
		prs, err := fetchPRs(context.Background(), client, username, orgs)
		return prsFetchedMsg{prs: prs, err: err}
	}
}

func (m model) tickCmd() tea.Cmd {
	interval := m.pollInterval
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		openBrowser(url)
		return nil
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		cmd = exec.Command("open", url)
	}
	_ = cmd.Start()
}

func clipboardCommand(goos string) (string, []string, error) {
	switch goos {
	case "darwin":
		return "pbcopy", nil, nil
	case "linux":
		return "xclip", []string{"-selection", "clipboard"}, nil
	default:
		return "", nil, fmt.Errorf("unsupported OS: %s", goos)
	}
}

func copyToClipboardCmd(url string) tea.Cmd {
	return func() tea.Msg {
		name, args, err := clipboardCommand(runtime.GOOS)
		if err != nil {
			return clipboardMsg{err: err}
		}
		cmd := exec.Command(name, args...)
		cmd.Stdin = strings.NewReader(url)
		return clipboardMsg{err: cmd.Run()}
	}
}
