package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	PrimaryColor   = lipgloss.Color("#7C3AED")
	SecondaryColor = lipgloss.Color("#10B981")
	AccentColor    = lipgloss.Color("#F59E0B")
	ErrorColor     = lipgloss.Color("#EF4444")
	WarningColor   = lipgloss.Color("#F59E0B")
	MutedColor     = lipgloss.Color("#6B7280")
	TextColor      = lipgloss.Color("#F3F4F6")
	BgColor        = lipgloss.Color("#1F2937")
	HighlightBg    = lipgloss.Color("#374151")

	// Base styles
	BaseStyle = lipgloss.NewStyle().
			Foreground(TextColor)

	// Title style
	TitleStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			Padding(0, 1)

	// Header box style
	HeaderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(1, 2).
			MarginBottom(1)

	// Info style
	InfoStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true)

	// Warning style
	WarningStyle = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)

	// Label style
	LabelStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Bold(true)

	// Value style
	ValueStyle = lipgloss.NewStyle().
			Foreground(TextColor)

	// Input box style
	InputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

	// Focused input style
	FocusedInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(SecondaryColor).
				Padding(0, 1).
				MarginTop(1).
				MarginBottom(1)

	// List item style
	ListItemStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			PaddingLeft(2)

	// Selected list item style
	SelectedItemStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor).
				Bold(true).
				PaddingLeft(2)

	// Highlight match style
	MatchStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	// Error style
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	// Success style
	SuccessStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Bold(true)

	// Help style
	HelpStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			MarginTop(1)

	// Status bar style
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Background(HighlightBg).
			Padding(0, 1)

	// Command style
	CommandStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	// Cursor style
	CursorStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor)

	// Prompt style
	PromptStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true)
)

// RenderHeader creates a styled header with app info
func RenderHeader(kubeconfig, namespace, deployment string) string {
	title := TitleStyle.Render("🚀 khelper - Kubernetes Helper")

	// Kubeconfig info
	kcLabel := LabelStyle.Render("Kubeconfig: ")
	kcValue := ValueStyle.Render(kubeconfig)
	if kubeconfig == "" {
		kcValue = InfoStyle.Render("(default)")
	}

	nsLabel := LabelStyle.Render("Namespace: ")
	nsValue := ValueStyle.Render(namespace)
	if namespace == "" {
		nsValue = InfoStyle.Render("(not selected)")
	}

	depLabel := LabelStyle.Render("Deployment: ")
	depValue := ValueStyle.Render(deployment)
	if deployment == "" {
		depValue = InfoStyle.Render("(not selected)")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		kcLabel+kcValue,
		nsLabel+nsValue,
		depLabel+depValue,
	)

	return HeaderStyle.Render(content)
}

// RenderHelp creates a styled help text
func RenderHelp(items ...string) string {
	var result string
	for i, item := range items {
		if i > 0 {
			result += " • "
		}
		result += item
	}
	return HelpStyle.Render(result)
}

// RenderError creates a styled error message
func RenderError(msg string) string {
	return ErrorStyle.Render("✗ " + msg)
}

// RenderSuccess creates a styled success message
func RenderSuccess(msg string) string {
	return SuccessStyle.Render("✓ " + msg)
}

// RenderLoading creates a styled loading message
func RenderLoading(msg string) string {
	return InfoStyle.Render("⏳ " + msg)
}
