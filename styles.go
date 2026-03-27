package main

import "github.com/charmbracelet/lipgloss"

var (
	// Status colors
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))
	failureStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1FA8C"))
	runningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Selection
	selectedStyle = lipgloss.NewStyle().Bold(true)
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6")).Bold(true)

	// Flash highlight styles
	flashSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Bold(true)
	flashFailureStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Bold(true)

	// Column styles
	repoColStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6")).Bold(true)
	titleColStyle  = lipgloss.NewStyle().Width(50)
	ciColStyle     = lipgloss.NewStyle().Width(9)
	reviewColStyle = lipgloss.NewStyle().Width(12)
	mergeColStyle  = lipgloss.NewStyle().Width(10)
	ageColStyle    = lipgloss.NewStyle().Width(6)

	// Header and footer
	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#BD93F9")).MarginBottom(1).MarginLeft(1)
	footerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).MarginTop(1).MarginLeft(1)
	columnHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(true)

	// Tree connectors for expanded check runs
	treeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Draft badge
	draftStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)

	// Confirmation overlay
	overlayBoxStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#BD93F9")).Padding(1, 4)
	overlayTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2"))
	overlayDimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	flashSuccessMsg  = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))
	flashFailureMsg  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
)

func statusStyle(status string) lipgloss.Style {
	switch status {
	case "SUCCESS":
		return successStyle
	case "FAILURE", "ERROR":
		return failureStyle
	case "PENDING":
		return pendingStyle
	default:
		return dimStyle
	}
}

func reviewStyleFn(decision string) lipgloss.Style {
	switch decision {
	case "APPROVED":
		return successStyle
	case "CHANGES_REQUESTED":
		return failureStyle
	case "REVIEW_REQUIRED":
		return pendingStyle
	default:
		return dimStyle
	}
}

func checkRunStyle(status, conclusion string) lipgloss.Style {
	if status != "COMPLETED" {
		return runningStyle
	}
	switch conclusion {
	case "SUCCESS":
		return successStyle
	case "FAILURE":
		return failureStyle
	case "SKIPPED", "NEUTRAL":
		return dimStyle
	default:
		return pendingStyle
	}
}
