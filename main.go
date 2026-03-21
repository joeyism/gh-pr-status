package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	configPath := flag.String("config", "", "path to config file (default: ~/.config/gh-prs/config.yaml)")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		if *configPath == "" && errors.Is(err, os.ErrNotExist) {
			cfg = Config{}
		} else {
			log.Fatalf("Config error: %v\nCreate config at ~/.config/gh-prs/config.yaml (see config.example.yaml)", err)
		}
	}
	if len(cfg.Orgs) == 0 {
		fmt.Fprintf(os.Stderr, "No orgs configured. Add orgs to ~/.config/gh-prs/config.yaml\n")
		fmt.Fprintf(os.Stderr, "See config.example.yaml for the format.\n")
	}

	token, err := getGitHubToken()
	if err != nil {
		log.Fatalf("Auth error: %v", err)
	}

	client := newGitHubClient(token)

	ctx := context.Background()
	username, err := getViewerLogin(ctx, client)
	if err != nil {
		log.Fatalf("Failed to get GitHub username: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Authenticated as %s\n", username)

	m := initialModel(client, username, cfg.Orgs, cfg.PollDuration())
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithReportFocus())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
