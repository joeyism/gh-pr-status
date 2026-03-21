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
| `j` / `k` or arrow keys | Move cursor up/down |
| `tab` | Expand/collapse check runs for selected PR |
| `o` | Open selected PR in browser |
| `r` | Force refresh |
| `c` | Post `@cursor review` comment on selected PR |
| `q` / `ctrl+c` | Quit |

## Configuration

Create `~/.config/gh-prs/config.yaml`:

```yaml
orgs:
  - mycompany
  - other-org
poll_interval: "30s"
```

| Field | Description | Default |
|---|---|---|
| `orgs` | GitHub organizations to include in the PR search | none |
| `poll_interval` | How often to refresh (minimum `5s`) | `30s` |

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
