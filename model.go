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

type orgPRsFetchedMsg struct {
	prs []PullRequest
	err error
}

type checkRunsFetchedMsg struct {
	prNumber int
	runs     []CheckRun
	err      error
}

type commentPostedMsg struct{ err error }
type prClosedMsg struct{ err error }
type prMergedMsg struct{ err error }
type prApprovedMsg struct{ err error }
type clipboardMsg struct{ err error }
type draftToggledMsg struct {
	err     error
	isDraft bool // the new state after toggle
}

type clearFlashMsg struct{}

type tickMsg time.Time

type viewState struct {
	prs            []PullRequest
	cursor         int
	scrollOffset   int
	expanded       map[int]bool
	loading        bool
	fetching       bool
	err            error
	lastUpdated    time.Time
	changedAt      map[int]time.Time
	previousStatus map[int]string
}

type model struct {
	mine     viewState
	org      viewState
	viewMode int // 0 = mine, 1 = org

	focused       bool
	confirmAction string // "cursor" | "close" | "merge" | "approve" | "" (none)
	flash         string
	width         int
	height        int

	client       *githubv4.Client
	username     string
	orgs         []string
	pollInterval time.Duration
}

func (m *model) activeView() *viewState {
	if m.viewMode == 1 {
		return &m.org
	}
	return &m.mine
}

func initialModel(client *githubv4.Client, username string, orgs []string, pollInterval time.Duration) model {
	return model{
		mine: viewState{
			loading:        true,
			fetching:       true,
			expanded:       make(map[int]bool),
			changedAt:      make(map[int]time.Time),
			previousStatus: make(map[int]string),
		},
		org: viewState{
			expanded:       make(map[int]bool),
			changedAt:      make(map[int]time.Time),
			previousStatus: make(map[int]string),
		},
		focused:      true,
		pollInterval: pollInterval,
		client:       client,
		username:     username,
		orgs:         orgs,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.fetchPRsCmd(), m.tickCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	v := m.activeView()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.FocusMsg:
		m.focused = true

	case tea.BlurMsg:
		m.focused = false

	case tea.KeyMsg:
		if m.confirmAction != "" {
			switch msg.String() {
			case "y", "enter":
				action := m.confirmAction
				m.confirmAction = ""
				pr := v.prs[v.cursor]
				switch action {
				case "cursor":
					return m, m.addCommentCmd(pr.ID, "@cursor review")
				case "close":
					return m, m.closePRCmd(pr.ID)
				case "merge":
					return m, m.mergePRCmd(pr.ID)
				case "approve":
					return m, m.approvePRCmd(pr.ID)
				case "draft":
					if pr.IsDraft {
						return m, m.markReadyCmd(pr.ID)
					}
					return m, m.convertToDraftCmd(pr.ID)
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
			if v.cursor > 0 {
				v.cursor--
				if v.cursor < v.scrollOffset {
					v.scrollOffset = v.cursor
				}
			}
		case "down", "j":
			if v.cursor < len(v.prs)-1 {
				v.cursor++
				m.adjustScroll()
			}
		case "tab":
			if len(v.prs) > 0 {
				pr := v.prs[v.cursor]
				v.expanded[pr.Number] = !v.expanded[pr.Number]
				if v.expanded[pr.Number] && m.viewMode == 1 && pr.CheckRuns == nil {
					return m, m.fetchCheckRunsCmd(pr.ID, pr.Number)
				}
				m.adjustScroll()
			}
		case "o":
			if len(v.prs) > 0 {
				return m, openBrowserCmd(v.prs[v.cursor].URL)
			}
		case "c":
			if m.viewMode == 0 && len(v.prs) > 0 {
				m.confirmAction = "cursor"
			}
		case "x":
			if m.viewMode == 0 && len(v.prs) > 0 {
				m.confirmAction = "close"
			}
		case "m":
			if m.viewMode == 0 && len(v.prs) > 0 {
				m.confirmAction = "merge"
			}
		case "p":
			if m.viewMode == 1 && len(v.prs) > 0 {
				pr := v.prs[v.cursor]
				if pr.Author != m.username {
					m.confirmAction = "approve"
				}
			}
		case "d":
			if m.viewMode == 0 && len(v.prs) > 0 {
				m.confirmAction = "draft"
			}
		case "r":
			if !v.fetching {
				v.fetching = true
				if m.viewMode == 1 {
					return m, m.fetchOrgPRsCmd()
				}
				return m, m.fetchPRsCmd()
			}
		case "y":
			if len(v.prs) > 0 {
				return m, copyToClipboardCmd(v.prs[v.cursor].URL)
			}
		case "b":
			if len(v.prs) > 0 && v.prs[v.cursor].HeadRefName != "" {
				return m, copyToClipboardCmd(v.prs[v.cursor].HeadRefName)
			}
		case "a":
			m.viewMode = 1 - m.viewMode
			v = m.activeView()
			if !v.fetching && (len(v.prs) == 0 || time.Since(v.lastUpdated) > time.Minute) {
				v.loading = len(v.prs) == 0
				v.fetching = true
				if m.viewMode == 1 {
					return m, m.fetchOrgPRsCmd()
				}
				return m, m.fetchPRsCmd()
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
		v.fetching = true
		if m.viewMode == 1 {
			return m, tea.Batch(tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} }), m.fetchOrgPRsCmd())
		}
		return m, tea.Batch(tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} }), m.fetchPRsCmd())

	case prMergedMsg:
		if msg.err != nil {
			m.flash = flashFailureMsg.Render(fmt.Sprintf("Error merging PR: %v", msg.err))
			return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} })
		}
		m.flash = flashSuccessMsg.Render("PR merged ✓")
		v.fetching = true
		if m.viewMode == 1 {
			return m, tea.Batch(tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} }), m.fetchOrgPRsCmd())
		}
		return m, tea.Batch(tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} }), m.fetchPRsCmd())

	case prApprovedMsg:
		if msg.err != nil {
			m.flash = flashFailureMsg.Render(fmt.Sprintf("Approve failed: %v", msg.err))
		} else {
			m.flash = flashSuccessMsg.Render("PR approved ✓")
		}
		v.fetching = true
		if m.viewMode == 1 {
			return m, tea.Batch(tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} }), m.fetchOrgPRsCmd())
		}
		return m, tea.Batch(tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} }), m.fetchPRsCmd())

	case draftToggledMsg:
		if msg.err != nil {
			m.flash = flashFailureMsg.Render(fmt.Sprintf("Error toggling draft: %v", msg.err))
			return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} })
		}
		if msg.isDraft {
			m.flash = flashSuccessMsg.Render("Converted to draft ✓")
		} else {
			m.flash = flashSuccessMsg.Render("Marked as ready for review ✓")
		}
		v.fetching = true
		if m.viewMode == 1 {
			return m, tea.Batch(tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} }), m.fetchOrgPRsCmd())
		}
		return m, tea.Batch(tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} }), m.fetchPRsCmd())

	case clipboardMsg:
		if msg.err != nil {
			m.flash = flashFailureMsg.Render(fmt.Sprintf("Copy failed: %v", msg.err))
		} else {
			m.flash = flashSuccessMsg.Render("Copied to clipboard ✓")
		}
		return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} })

	case prsFetchedMsg:
		m.mine.fetching = false
		m.mine.loading = false
		if msg.err != nil {
			m.mine.err = msg.err
			return m, nil
		}
		m.mine.err = nil
		now := time.Now()
		m.mine.lastUpdated = now
		statusChanged := false
		for _, pr := range msg.prs {
			newKey := fmt.Sprintf("%s|%s|%d|%s", pr.CheckStatus, pr.ReviewDecision, pr.UnresolvedThreads, pr.Mergeable)
			if oldKey, ok := m.mine.previousStatus[pr.Number]; ok && oldKey != newKey {
				m.mine.changedAt[pr.Number] = now
				statusChanged = true
			}
			m.mine.previousStatus[pr.Number] = newKey
		}
		m.mine.prs = msg.prs
		if m.mine.cursor >= len(m.mine.prs) && len(m.mine.prs) > 0 {
			m.mine.cursor = len(m.mine.prs) - 1
		}
		m.adjustScroll()
		if statusChanged {
			return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} })
		}

	case orgPRsFetchedMsg:
		m.org.fetching = false
		m.org.loading = false
		if msg.err != nil {
			m.org.err = msg.err
			return m, nil
		}
		m.org.err = nil
		now := time.Now()
		m.org.lastUpdated = now
		m.org.prs = msg.prs
		if m.org.cursor >= len(m.org.prs) && len(m.org.prs) > 0 {
			m.org.cursor = len(m.org.prs) - 1
		}
		m.adjustScroll()

	case checkRunsFetchedMsg:
		for i, pr := range m.org.prs {
			if pr.Number == msg.prNumber {
				m.org.prs[i].CheckRuns = msg.runs
				break
			}
		}
		m.adjustScroll()

	case tickMsg:
		cmds := []tea.Cmd{m.tickCmd()}
		if !v.fetching {
			v.fetching = true
			if m.viewMode == 1 {
				cmds = append(cmds, m.fetchOrgPRsCmd())
			} else {
				cmds = append(cmds, m.fetchPRsCmd())
			}
		}
		return m, tea.Batch(cmds...)

	case clearFlashMsg:
		m.mine.changedAt = make(map[int]time.Time)
		m.org.changedAt = make(map[int]time.Time)
		m.flash = ""
	}

	return m, nil
}

func (m *model) adjustScroll() {
	v := m.activeView()
	if len(v.prs) == 0 {
		return
	}
	visibleLines := m.height - 9 // Budget for header, colheader, footer, border
	if visibleLines <= 0 {
		return
	}

	if v.cursor < v.scrollOffset {
		v.scrollOffset = v.cursor
		return
	}

	for {
		linesConsumed := 0
		found := false
		for i := v.scrollOffset; i < len(v.prs); i++ {
			itemHeight := 1
			if v.expanded[v.prs[i].Number] {
				itemHeight += len(v.prs[i].CheckRuns)
				if len(v.prs[i].CheckRuns) == 0 {
					itemHeight++
				}
			}
			if i == v.cursor {
				if linesConsumed+itemHeight <= visibleLines {
					found = true
				}
				break
			}
			linesConsumed += itemHeight
			if linesConsumed >= visibleLines {
				break
			}
		}

		if found || v.scrollOffset >= v.cursor {
			break
		}
		v.scrollOffset++
	}
}

func (m model) View() string {
	v := m.activeView()
	if v.loading {
		title := "My PRs"
		if m.viewMode == 1 {
			title = "Org PRs"
		}
		return headerStyle.Render("gh-pr-status ["+title+"]") + "\n\nLoading PRs...\n"
	}

	var b strings.Builder
	title := "My PRs"
	if m.viewMode == 1 {
		title = "Org PRs"
	}
	b.WriteString(headerStyle.Render("gh-pr-status [" + title + "]"))
	b.WriteString("\n")

	if v.err != nil {
		b.WriteString(failureStyle.Render(fmt.Sprintf("Error: %v", v.err)))
		b.WriteString("\n\n")
	}

	if len(v.prs) == 0 {
		b.WriteString("No open PRs found.\n")
	} else {
		now := time.Now()
		repoWidth := 8
		for _, pr := range v.prs {
			if len(pr.Repo) > repoWidth {
				repoWidth = len(pr.Repo)
			}
		}
		repoWidth++
		dynRepoStyle := repoColStyle.Width(repoWidth)

		authorWidth := 8
		if m.viewMode == 1 {
			for _, pr := range v.prs {
				if len(pr.Author) > authorWidth {
					authorWidth = len(pr.Author)
				}
			}
		}
		authorWidth++
		dynAuthorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("246")).Width(authorWidth)

		repoGap := strings.Repeat(" ", 3+repoWidth+1)
		if m.viewMode == 1 {
			repoGap += strings.Repeat(" ", authorWidth+1)
		}
		b.WriteString(columnHeaderStyle.Render(fmt.Sprintf("%s%-50s %-9s %-12s %-10s %-6s %-6s",
			repoGap, "", "CI", "Review", "Merge", "Upd", "Crtd")))
		b.WriteString("\n\n")

		visibleLines := m.height - 9
		linesConsumed := 0

		for i := v.scrollOffset; i < len(v.prs); i++ {
			pr := v.prs[i]
			cursor := "   "
			if i == v.cursor {
				cursor = cursorStyle.Render("┃") + "  "
			}

			prTitle := pr.Title
			if pr.IsDraft {
				prTitle = "DRAFT " + prTitle
			}
			if len(prTitle) > 48 {
				prTitle = prTitle[:47] + "…"
			}
			titleRendered := titleColStyle.Render(prTitle)
			if pr.IsDraft {
				titleRendered = titleColStyle.Foreground(lipgloss.Color("240")).Italic(true).Render(prTitle)
			}

			authorPart := ""
			if m.viewMode == 1 {
				authorPart = dynAuthorStyle.Render(pr.Author) + " "
			}

			datePart := " " + ageColStyle.Render(formatAge(pr.UpdatedAt)) + " " + ageColStyle.Render(formatAge(pr.CreatedAt))
			line := fmt.Sprintf("%s%s %s%s %s %s %s%s %s",
				cursor,
				dynRepoStyle.Render(pr.Repo),
				authorPart,
				titleRendered,
				ciColStyle.Render(formatCIStatus(pr.CheckStatus)),
				reviewColStyle.Render(formatReviewStatus(pr.ReviewDecision)),
				mergeColStyle.Render(formatMergeable(pr.Mergeable)),
				datePart,
				formatComments(pr.TotalComments, pr.UnresolvedThreads, pr.TotalThreads),
			)

			if i == v.cursor {
				line = selectedStyle.Render(line)
			}

			if t, ok := v.changedAt[pr.Number]; ok && now.Sub(t) < 3*time.Second {
				if pr.CheckStatus == "SUCCESS" {
					line = flashSuccessStyle.Render(line)
				} else if pr.CheckStatus == "FAILURE" || pr.CheckStatus == "ERROR" {
					line = flashFailureStyle.Render(line)
				}
			}

			b.WriteString(line + "\n")
			linesConsumed++

			if v.expanded[pr.Number] {
				treeIndent := "   " + strings.Repeat(" ", repoWidth) + " "
				if m.viewMode == 1 {
					treeIndent += strings.Repeat(" ", authorWidth) + " "
				}
				if pr.CheckRuns == nil && m.viewMode == 1 {
					b.WriteString(treeIndent + treeStyle.Render("└─") + " " + dimStyle.Render("loading check runs...") + "\n")
					linesConsumed++
				} else if len(pr.CheckRuns) == 0 {
					b.WriteString(treeIndent + treeStyle.Render("└─") + " " + dimStyle.Render("no check runs") + "\n")
					linesConsumed++
				} else {
					for j, cr := range pr.CheckRuns {
						branch := "├─"
						if j == len(pr.CheckRuns)-1 {
							branch = "└─"
						}
						b.WriteString(treeIndent + treeStyle.Render(branch) + " " + formatCheckRun(cr) + "\n")
						linesConsumed++
					}
				}
			}

			if linesConsumed >= visibleLines {
				break
			}
		}
	}

	ago := ""
	if !v.lastUpdated.IsZero() {
		ago = fmt.Sprintf("Updated %s ago", time.Since(v.lastUpdated).Round(time.Second))
	}
	fetchIndicator := ""
	if v.fetching {
		fetchIndicator = " (refreshing...)"
	}
	flashLine := ""
	if m.flash != "" {
		flashLine = "\n" + m.flash
	}

	help := "j/k: nav • tab: expand • o: open • y: yank • b: branch • r: refresh • c: review • d: draft • x: close • m: merge • a: org view • q: quit"
	if m.viewMode == 1 {
		help = "j/k: nav • tab: expand • o: open • y: yank • b: branch • r: refresh • p: approve • a: my prs • q: quit"
	}
	b.WriteString(footerStyle.Render(fmt.Sprintf("\n%s%s%s                    %s", ago, fetchIndicator, flashLine, help)))

	out := b.String()
	borderColor := lipgloss.Color("240")
	if m.focused {
		borderColor = lipgloss.Color("#BD93F9")
	}
	innerW, innerH := m.width-2, m.height-2
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}
	out = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(borderColor).Width(innerW).Height(innerH).Render(out)

	if m.confirmAction != "" && len(v.prs) > 0 {
		pr := v.prs[v.cursor]
		title := pr.Title
		if len(title) > 40 {
			title = title[:39] + "…"
		}
		var questionText string
		var yesBtnColor lipgloss.Color
		switch m.confirmAction {
		case "close":
			questionText, yesBtnColor = "Close pull request", lipgloss.Color("#FF5555")
		case "merge":
			questionText, yesBtnColor = "Squash and merge", lipgloss.Color("#50FA7B")
		case "approve":
			questionText, yesBtnColor = "Approve pull request", lipgloss.Color("#50FA7B")
		case "draft":
			if pr.IsDraft {
				questionText, yesBtnColor = "Mark as ready for review", lipgloss.Color("#50FA7B")
			} else {
				questionText, yesBtnColor = "Convert to draft (dismisses reviews)", lipgloss.Color("#FF5555")
			}
		default:
			questionText, yesBtnColor = "Request @cursor review on", lipgloss.Color("#50FA7B")
		}
		question := overlayTextStyle.Render(questionText)
		prInfo := overlayTextStyle.Bold(true).Render(fmt.Sprintf("%q (#%d)?", title, pr.Number))
		yesBtn := overlayTextStyle.Bold(true).Foreground(yesBtnColor).Render("[y]es")
		noBtn := overlayTextStyle.Bold(true).Foreground(lipgloss.Color("#FF5555")).Render("[n]o")
		content := lipgloss.JoinVertical(lipgloss.Left, question, prInfo, "", yesBtn+"    "+noBtn)
		box := overlayBoxStyle.Render(content)
		out = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box, lipgloss.WithWhitespaceChars(" "), lipgloss.WithWhitespaceForeground(lipgloss.Color("236")))
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
		return fmt.Sprintf("%s %s", dimStyle.Render(fmt.Sprintf("💬 %d", total)), pendingStyle.Render("(? unresolved)"))
	}
	if unresolved == 0 {
		return fmt.Sprintf("%s %s", dimStyle.Render(fmt.Sprintf("💬 %d", total)), successStyle.Render("(0 unresolved)"))
	}
	return fmt.Sprintf("%s %s", dimStyle.Render(fmt.Sprintf("💬 %d", total)), pendingStyle.Render(fmt.Sprintf("(%d unresolved)", unresolved)))
}

func formatCheckRun(cr CheckRun) string {
	s := checkRunStyle(cr.Status, cr.Conclusion)
	symbol, label := "●", strings.ToLower(cr.Status)
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

func formatAge(t time.Time) string {
	if t.IsZero() {
		return dimStyle.Render("--")
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return dimStyle.Render(fmt.Sprintf("%dm", int(d.Minutes())))
	case d < 24*time.Hour:
		return dimStyle.Render(fmt.Sprintf("%dh", int(d.Hours())))
	case d < 30*24*time.Hour:
		return dimStyle.Render(fmt.Sprintf("%dd", int(d.Hours()/24)))
	case d < 52*7*24*time.Hour:
		return dimStyle.Render(fmt.Sprintf("%dw", int(d.Hours()/(7*24))))
	default:
		return dimStyle.Render(fmt.Sprintf("%dmo", int(d.Hours()/(30*24))))
	}
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

func (m model) approvePRCmd(prID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := approvePR(context.Background(), client, prID)
		return prApprovedMsg{err: err}
	}
}

func (m model) convertToDraftCmd(prID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := convertToDraft(context.Background(), client, prID)
		return draftToggledMsg{err: err, isDraft: true}
	}
}

func (m model) markReadyCmd(prID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := markReadyForReview(context.Background(), client, prID)
		return draftToggledMsg{err: err, isDraft: false}
	}
}

func (m model) fetchPRsCmd() tea.Cmd {
	client, username, orgs := m.client, m.username, m.orgs
	return func() tea.Msg {
		prs, err := fetchPRs(context.Background(), client, username, orgs)
		return prsFetchedMsg{prs: prs, err: err}
	}
}

func (m model) fetchOrgPRsCmd() tea.Cmd {
	client, orgs := m.client, m.orgs
	return func() tea.Msg {
		prs, err := fetchOrgPRs(context.Background(), client, orgs)
		return orgPRsFetchedMsg{prs: prs, err: err}
	}
}

func (m model) fetchCheckRunsCmd(prID string, prNumber int) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		runs, err := fetchCheckRuns(context.Background(), client, prID)
		return checkRunsFetchedMsg{prNumber: prNumber, runs: runs, err: err}
	}
}

func (m model) tickCmd() tea.Cmd {
	return tea.Tick(m.pollInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
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
