package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LogViewer is an interactive log viewer with search and selection capability
type LogViewer struct {
	viewport       viewport.Model
	detailViewport viewport.Model
	searchInput    textinput.Model
	allLines       []string
	filteredLines  []string
	recentSearches []string
	searchQuery    string
	selectedIndex  int
	showSearch     bool
	ready          bool
	width          int
	height         int
	streaming      bool
	autoScroll     bool
}

// NewLogViewer creates a new log viewer component
func NewLogViewer() LogViewer {
	ti := textinput.New()
	ti.Placeholder = "Type to search..."
	ti.Prompt = "> "
	ti.CharLimit = 200
	ti.Width = 60
	ti.PromptStyle = PromptStyle
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))

	return LogViewer{
		searchInput:    ti,
		allLines:       []string{},
		filteredLines:  []string{},
		recentSearches: []string{},
		showSearch:     true,
		selectedIndex:  0,
		autoScroll:     true,
	}
}

// SetSize sets the viewport size
func (l *LogViewer) SetSize(width, height int) {
	l.width = width
	l.height = height

	// Split: list takes 60%, detail takes 40% (minus headers)
	listHeight := (height - 10) * 6 / 10
	detailHeight := (height - 10) - listHeight

	if listHeight < 5 {
		listHeight = 5
	}
	if detailHeight < 3 {
		detailHeight = 3
	}

	if !l.ready {
		l.viewport = viewport.New(width-4, listHeight)
		l.viewport.Style = BaseStyle
		l.detailViewport = viewport.New(width-4, detailHeight)
		l.detailViewport.Style = BaseStyle
		l.ready = true
	} else {
		l.viewport.Width = width - 4
		l.viewport.Height = listHeight
		l.detailViewport.Width = width - 4
		l.detailViewport.Height = detailHeight
	}

	l.searchInput.Width = width - 20
	l.updateContent()
}

// SetLogs sets the log content
func (l *LogViewer) SetLogs(logs string) {
	if logs == "" {
		l.allLines = []string{}
	} else {
		l.allLines = strings.Split(logs, "\n")
	}
	l.filterLogs()
}

// AppendLog appends a log line
func (l *LogViewer) AppendLog(line string) {
	l.allLines = append(l.allLines, line)
	l.filterLogs()

	// Auto-scroll to bottom if enabled and at/near bottom
	if l.autoScroll && l.streaming {
		if len(l.filteredLines) > 0 {
			l.selectedIndex = len(l.filteredLines) - 1
		}
	}
}

// SetStreaming sets streaming mode
func (l *LogViewer) SetStreaming(streaming bool) {
	l.streaming = streaming
	l.autoScroll = streaming
}

// IsStreaming returns whether in streaming mode
func (l *LogViewer) IsStreaming() bool {
	return l.streaming
}

// SetRecentSearches sets the recent search terms
func (l *LogViewer) SetRecentSearches(searches []string) {
	l.recentSearches = searches
}

// GetSearchQuery returns the current search query
func (l *LogViewer) GetSearchQuery() string {
	return l.searchQuery
}

func (l *LogViewer) filterLogs() {
	query := strings.ToLower(l.searchInput.Value())
	l.searchQuery = l.searchInput.Value()

	if query == "" {
		l.filteredLines = l.allLines
	} else {
		l.filteredLines = make([]string, 0)
		for _, line := range l.allLines {
			if strings.Contains(strings.ToLower(line), query) {
				l.filteredLines = append(l.filteredLines, line)
			}
		}
	}

	// Reset selection if out of bounds
	if l.selectedIndex >= len(l.filteredLines) {
		l.selectedIndex = 0
	}

	l.updateContent()
}

func (l *LogViewer) updateContent() {
	if !l.ready {
		return
	}

	var content strings.Builder
	query := strings.ToLower(l.searchInput.Value())

	for i, line := range l.filteredLines {
		// Truncate long lines for the list view
		displayLine := line
		maxLen := l.width - 10
		if maxLen > 0 && len(displayLine) > maxLen {
			displayLine = displayLine[:maxLen] + "..."
		}

		// Apply selection style
		if i == l.selectedIndex {
			// Selected line - highlight background
			if query != "" {
				highlighted := l.highlightMatches(displayLine, query)
				content.WriteString(SelectedItemStyle.Render("▶ " + highlighted))
			} else {
				content.WriteString(SelectedItemStyle.Render("▶ " + displayLine))
			}
		} else {
			// Normal line
			if query != "" {
				highlighted := l.highlightMatches(displayLine, query)
				content.WriteString("  " + highlighted)
			} else {
				content.WriteString("  " + displayLine)
			}
		}
		content.WriteString("\n")
	}

	l.viewport.SetContent(content.String())

	// Update detail viewport with full selected line
	l.updateDetailView()

	// Ensure selected line is visible
	l.ensureSelectedVisible()
}

func (l *LogViewer) updateDetailView() {
	if !l.ready || len(l.filteredLines) == 0 {
		l.detailViewport.SetContent(InfoStyle.Render("No log entry selected"))
		return
	}

	if l.selectedIndex < len(l.filteredLines) {
		fullLine := l.filteredLines[l.selectedIndex]
		query := strings.ToLower(l.searchInput.Value())

		// Word wrap the full line
		wrapped := l.wordWrap(fullLine, l.width-6)

		if query != "" {
			wrapped = l.highlightMatches(wrapped, query)
		}

		l.detailViewport.SetContent(wrapped)
	}
}

func (l *LogViewer) wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	for len(text) > width {
		// Find a good break point
		breakAt := width
		for i := width; i > width/2; i-- {
			if text[i] == ' ' || text[i] == ',' || text[i] == ';' || text[i] == ':' {
				breakAt = i + 1
				break
			}
		}
		result.WriteString(text[:breakAt])
		result.WriteString("\n")
		text = text[breakAt:]
	}
	result.WriteString(text)
	return result.String()
}

func (l *LogViewer) ensureSelectedVisible() {
	if len(l.filteredLines) == 0 {
		return
	}

	// Each line is approximately 1 row
	visibleStart := l.viewport.YOffset
	visibleEnd := visibleStart + l.viewport.Height

	if l.selectedIndex < visibleStart {
		l.viewport.SetYOffset(l.selectedIndex)
	} else if l.selectedIndex >= visibleEnd {
		l.viewport.SetYOffset(l.selectedIndex - l.viewport.Height + 1)
	}
}

func (l *LogViewer) highlightMatches(line, query string) string {
	lower := strings.ToLower(line)
	var result strings.Builder
	lastEnd := 0

	for {
		idx := strings.Index(lower[lastEnd:], query)
		if idx == -1 {
			result.WriteString(line[lastEnd:])
			break
		}

		matchStart := lastEnd + idx
		matchEnd := matchStart + len(query)

		result.WriteString(line[lastEnd:matchStart])
		result.WriteString(MatchStyle.Render(line[matchStart:matchEnd]))

		lastEnd = matchEnd
	}

	return result.String()
}

// Update handles messages
func (l *LogViewer) Update(msg tea.Msg) (LogViewer, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		// Navigation - works even when search is focused
		case "up", "k":
			if l.selectedIndex > 0 {
				l.selectedIndex--
				l.updateContent()
			}
			return *l, nil
		case "down", "j":
			if l.selectedIndex < len(l.filteredLines)-1 {
				l.selectedIndex++
				l.updateContent()
			}
			return *l, nil
		case "pgup", "ctrl+u":
			// Move selection up by half page
			l.selectedIndex -= l.viewport.Height / 2
			if l.selectedIndex < 0 {
				l.selectedIndex = 0
			}
			l.updateContent()
			return *l, nil
		case "pgdown", "ctrl+d":
			// Move selection down by half page
			l.selectedIndex += l.viewport.Height / 2
			if l.selectedIndex >= len(l.filteredLines) {
				l.selectedIndex = len(l.filteredLines) - 1
			}
			if l.selectedIndex < 0 {
				l.selectedIndex = 0
			}
			l.updateContent()
			return *l, nil
		case "home", "g":
			if !l.searchInput.Focused() {
				l.selectedIndex = 0
				l.updateContent()
				return *l, nil
			}
		case "end", "G":
			if !l.searchInput.Focused() {
				if len(l.filteredLines) > 0 {
					l.selectedIndex = len(l.filteredLines) - 1
				}
				l.updateContent()
				return *l, nil
			}
		case "/":
			// Focus search if not already focused
			if !l.searchInput.Focused() {
				l.searchInput.Focus()
				return *l, nil
			}
		case "enter":
			if l.searchInput.Focused() {
				l.searchInput.Blur()
				return *l, nil
			}
		case "tab":
			// Toggle focus between search and viewport
			if l.searchInput.Focused() {
				l.searchInput.Blur()
			} else {
				l.searchInput.Focus()
			}
			return *l, nil
		case "ctrl+l":
			// Clear search
			l.searchInput.SetValue("")
			l.filterLogs()
			return *l, nil
		}
	}

	// Update search input if focused
	if l.searchInput.Focused() {
		prevValue := l.searchInput.Value()
		l.searchInput, cmd = l.searchInput.Update(msg)
		cmds = append(cmds, cmd)

		if l.searchInput.Value() != prevValue {
			l.filterLogs()
		}
	}

	return *l, tea.Batch(cmds...)
}

// View renders the log viewer
func (l *LogViewer) View() string {
	var b strings.Builder

	// Streaming indicator
	if l.streaming {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true).Render("● LIVE "))
	}

	// Search box label
	if l.searchInput.Focused() {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true).Render("🔍 Search: "))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("🔍 Search: "))
	}
	b.WriteString(l.searchInput.View())

	// Stats
	stats := "  " + InfoStyle.Render(itoa(len(l.filteredLines))+"/"+itoa(len(l.allLines))+" lines")
	if l.selectedIndex < len(l.filteredLines) {
		stats += InfoStyle.Render(" • Selected: " + itoa(l.selectedIndex+1))
	}
	b.WriteString(stats)
	b.WriteString("\n")

	// Log list header
	b.WriteString(LabelStyle.Render("─── Matching Logs ───"))
	b.WriteString("\n")

	// Log list viewport
	if l.ready {
		b.WriteString(l.viewport.View())
	}

	// Detail header
	b.WriteString("\n")
	b.WriteString(LabelStyle.Render("─── Full Log Entry ───"))
	b.WriteString("\n")

	// Detail viewport with border
	if l.ready {
		detailStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7C3AED")).
			Padding(0, 1)
		b.WriteString(detailStyle.Render(l.detailViewport.View()))
	}

	return b.String()
}

// Focus focuses the search input
func (l *LogViewer) Focus() {
	l.searchInput.Focus()
}

// Blur blurs the search input
func (l *LogViewer) Blur() {
	l.searchInput.Blur()
}

// IsFocused returns whether the search input is focused
func (l *LogViewer) IsFocused() bool {
	return l.searchInput.Focused()
}
