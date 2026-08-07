# gh-pr-status

A `gh` extension that shows a live terminal dashboard of your open pull requests across GitHub organizations.

Displays CI status, review decisions, merge readiness, and comment/thread counts in a single view. Polls GitHub automatically and highlights PRs that change state.

## Installation

```bash
gh extension install joeyism/gh-pr-status
```

## Usage

```bash
gh pr-status
```

By default it shows all open PRs authored by you. To scope results to specific organizations, create a config file (see Configuration).

### Flags

| Flag | Description |
|---|---|
| `--config <path>` | Path to config file (default: `~/.config/gh-prs/config.yaml`) |

### Keybindings

| Key | Action |
|---|---|
| `j` / `k` or arrow keys | Move cursor up/down across PRs and expanded check runs |
| `tab` | Expand/collapse checks for the PR under the cursor (or parent PR if on a check) |
| `s` | Open the sort menu (changes the sort for the current session only) |
| `o` | Open selected PR in browser, or the focused check run’s GitHub page |
| `y` | Copy selected PR URL, or the focused check run’s URL, to the clipboard |
| `r` | Force refresh |
| `c` | Post `@cursor review` comment on selected PR |
| `q` / `ctrl+c` | Quit |

Inside the sort menu: `j`/`k` (or arrow keys) to move, `tab` to toggle
ascending/descending, `enter` to apply, `esc` to cancel.

## Configuration

Create `~/.config/gh-prs/config.yaml`:

```yaml
orgs:
  - mycompany
  - other-org
poll_interval: "30s"
sort_by: "updated"
sort_dir: "desc"
```

| Field | Description | Default |
|---|---|---|
| `orgs` | GitHub organizations to include in the PR search | none |
| `poll_interval` | How often to refresh (minimum `5s`) | `30s` |
| `sort_by` | Initial sort field: `updated`, `created`, `repo`, `author`, `title`, `ci`, `review`, `merge`, `comments`, or `number` | `updated` |
| `sort_dir` | Initial sort direction: `asc` or `desc` | `desc` |

The `sort_by` and `sort_dir` values only set the startup default. The in-app
sort menu changes the sort for the current session; it does not write back to
the config file. Invalid values are ignored and fall back to the defaults
with a warning printed to stderr.

## Authentication

The extension uses your existing `gh` credentials. Run `gh auth login` if you have not already authenticated. You can also set `GITHUB_TOKEN` or `GH_TOKEN` as an environment variable to override.

## Requirements

- `gh` CLI installed and authenticated
- Go 1.21 or later (only needed if building from source)

## Building from source

```bash
git clone https://github.com/joeyism/gh-pr-status
cd gh-pr-status
go build -o gh-pr-status .
```
