# PR Status TUI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go TUI that shows all the user's open GitHub PRs across configured orgs with CI status, review status, auto-polling, expand/collapse for check details, and open-in-browser.

**Architecture:** Bubbletea TUI with a flat list view. GitHub GraphQL API (via shurcooL/githubv4) fetches all open PRs for the authenticated user, filtered to configured orgs. Polling via tea.Tick auto-refreshes data. Config lives in ~/.config/gh-prs/config.yaml.

**Tech Stack:** Go, Bubbletea, Lipgloss, shurcooL/githubv4, gopkg.in/yaml.v3

---

## Project Structure

```
github-pr-status/
├── main.go              # entrypoint, wires config + client + TUI
├── config.go            # config loading from YAML
├── github.go            # GraphQL queries, data types
├── model.go             # Bubbletea model, Update, View
├── styles.go            # lipgloss style definitions
├── config.example.yaml  # example config
├── go.mod
└── go.sum
```

---

### Task 1: Project Scaffolding

**Files:**
- Create: go.mod
- Create: main.go

**Step 1:** Initialize Go module
Run: go mod init github.com/joey/gh-pr-status

**Step 2:** Create minimal main.go that prints "gh-pr-status"

**Step 3:** Verify: go build ./... (clean build)

**Step 4:** Commit: git init && git add -A && git commit -m "chore: scaffold go project"

---

### Task 2: Config Loading

**Files:**
- Create: config.go
- Create: config.example.yaml

**Step 1:** go get gopkg.in/yaml.v3

**Step 2:** Create config.go with:
- Config struct: Orgs []string, PollInterval string
- PollDuration() method that parses duration with 30s default, 5s minimum
- defaultConfigPath() returning ~/.config/gh-prs/config.yaml
- LoadConfig(path) that reads YAML, applies defaults

**Step 3:** Create config.example.yaml:
```yaml
orgs:
  - mycompany
  - other-org
poll_interval: "30s"
```

**Step 4:** Wire into main.go with --config flag support

**Step 5:** Verify: go build ./...

**Step 6:** Commit: "feat: add config loading from YAML"

---

### Task 3: GitHub Auth + Client + PR Fetching

**Files:**
- Create: github.go
- Modify: main.go

**Step 1:** go get github.com/shurcooL/githubv4 golang.org/x/oauth2

**Step 2:** Create github.go with:

Auth:
- getGitHubToken() checks GITHUB_TOKEN, GH_TOKEN env vars, then shells out to gh auth token
- newGitHubClient(token) wraps with oauth2

Data types:
- CheckRun: Name, Status (QUEUED/IN_PROGRESS/COMPLETED), Conclusion (SUCCESS/FAILURE/etc)
- PullRequest: Number, Title, URL, Repo, Org, CheckStatus (rollup), CheckRuns (detail), ReviewDecision, IsDraft

Queries:
- getViewerLogin() - simple viewer { login } query
- fetchPRs() - search query: "is:pr is:open archived:false author:{user} org:{org1} org:{org2}"
  Returns PRs with: title, url, repo, org, reviewDecision, isDraft
  Plus commits(last:1) -> statusCheckRollup { state, contexts(first:50) { ...on CheckRun { name, status, conclusion } } }

**Step 3:** Temporary main.go that fetches and prints PRs to stdout (will be replaced by TUI)

**Step 4:** go mod tidy && go build ./...

**Step 5:** Commit: "feat: add GitHub GraphQL client with PR fetching"

---

### Task 4: Lipgloss Styles

**Files:**
- Create: styles.go

**Step 1:** go get github.com/charmbracelet/lipgloss

**Step 2:** Create styles.go with:
- Status colors: successStyle (#50FA7B), failureStyle (#FF5555), pendingStyle (#F1FA8C), runningStyle (#8BE9FD), dimStyle (240)
- Row styles: selectedStyle (bold), normalStyle
- Flash styles: flashSuccessStyle, flashFailureStyle (bold + color)
- checkIndentStyle (PaddingLeft 4)
- headerStyle (bold purple), footerStyle (dim)
- repoStyle (pink bold), draftStyle (dim italic)
- Helper functions: statusStyle(status), reviewStyle(decision), checkRunStyle(status, conclusion)

**Step 3:** Verify: go build ./...

**Step 4:** Commit: "feat: add lipgloss style definitions"

---

### Task 5: Bubbletea TUI Model

**Files:**
- Create: model.go
- Modify: main.go

This is the core task. Handles: list navigation, expand/collapse, polling, open-in-browser, status change flash.

**Step 1:** go get github.com/charmbracelet/bubbletea

**Step 2:** Create model.go with:

Messages:
- prsFetchedMsg { prs, err }
- tickMsg

Model struct:
- prs []PullRequest, cursor int
- expanded map[int]bool (PR number -> expanded)
- loading, fetching bool
- err error, lastUpdated time.Time
- pollInterval time.Duration
- changedAt map[int]time.Time (for flash highlight)
- previousStatus map[int]string (for change detection: "checkStatus|reviewDecision")
- client, username, orgs (dependencies)

Init():
- tea.Batch(fetchPRsCmd(), tickCmd())

Update():
- KeyMsg handlers:
  - q/ctrl+c: quit
  - j/k/up/down: navigate cursor
  - tab: toggle expanded[pr.Number]
  - o: openBrowser(pr.URL)
  - r: manual refresh (guard with fetching bool)
- prsFetchedMsg: update prs, detect status changes by diffing previousStatus map, set changedAt timestamp for changed PRs, clamp cursor
- tickMsg: reschedule next tick + fetch if not already fetching (prevents request stacking)

View():
- Header: "gh-pr-status"
- Error display if present
- For each PR:
  - Cursor indicator ("> " or "  ")
  - repoStyle(repo) - title - ciText - reviewText
  - Draft prefix [DRAFT] if isDraft
  - Flash highlight if changedAt[pr.Number] within 3 seconds
  - If expanded[pr.Number]: indented check runs with symbol + name + status
- Footer: "Updated Xs ago (refreshing...) | j/k: navigate | tab: expand | o: open | r: refresh | q: quit"

Format helpers:
- formatCIStatus(status): "CI passed" / "CI failed" / "CI running" / "CI error" / "no CI"
- formatReviewStatus(decision): "approved" / "changes requested" / "pending review" / "no reviews"
- formatCheckRun(cr): symbol + name + label, colored by status/conclusion

Command helpers:
- fetchPRsCmd(): tea.Cmd wrapping fetchPRs()
- tickCmd(): tea.Tick with pollInterval
- openBrowser(url): exec "open" on darwin, "xdg-open" on linux

**Step 3:** Replace main.go with TUI entrypoint:
- Load config (with --config flag), get token, create client, get viewer login
- Print auth info to stderr before TUI starts
- Create model via initialModel(), run tea.NewProgram with WithAltScreen

**Step 4:** go mod tidy && go build ./...

**Step 5:** Commit: "feat: add bubbletea TUI with polling, expand, open-in-browser"

---

### Task 6: Manual Integration Test

**No code changes — verification only.**

**Step 1:** Create ~/.config/gh-prs/config.yaml with real orgs

**Step 2:** Run: go run .

**Step 3:** Verify checklist:
- [ ] PRs from configured orgs displayed
- [ ] j/k and arrows navigate
- [ ] tab expands/collapses check runs
- [ ] o opens PR in browser
- [ ] r manual refresh works
- [ ] Auto-poll refreshes (watch "Updated Xs ago")
- [ ] q quits cleanly
- [ ] CI shows: passed/failed/running/no CI with correct colors
- [ ] Review shows: approved/changes requested/pending review/no reviews
- [ ] Draft PRs show [DRAFT]
- [ ] Status changes flash briefly

**Step 4:** Fix issues. Common gotchas:
- shurcooL/githubv4 union types for contexts may need struct tag adjustment
- Nil StatusCheckRollup for repos without CI
- __typename field handling for inline fragments

**Step 5:** Commit fixes

---

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Auth | gh auth token + env fallback | Zero config for gh CLI users |
| API | GraphQL via shurcooL/githubv4 | Single query for everything |
| Config | ~/.config/gh-prs/config.yaml | XDG convention |
| Polling | tea.Tick self-rescheduling | No stacking, easy cancel |
| Flash | 3s bold color on change | Free fade from existing tick |
| Check details | Fetched upfront, shown on tab | Simple, negligible cost |

## Known Risks

1. shurcooL/githubv4 inline fragment handling for CheckRun|StatusContext union can be fragile
2. Nil StatusCheckRollup on repos with no CI — must nil-check
3. Rate limits: 30s poll ~600 points/hr of 5000 — comfortable

## Future Enhancements (not in scope)

- Sort by repo/age/status
- Filter draft/non-draft
- Show PR age
- Desktop notifications on status change
- Show PRs assigned for review
