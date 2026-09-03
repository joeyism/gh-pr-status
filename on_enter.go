package main

import (
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func posixShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func expandOnEnter(template, url string) string {
	return strings.ReplaceAll(template, "${PR_URL}", posixShellQuote(url))
}

func onEnterExecCmd(script string) tea.Cmd {
	cmd := exec.Command("sh", "-c", script)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return onEnterFinishedMsg{err: err}
	})
}
