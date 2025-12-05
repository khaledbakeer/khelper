package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
)

// FuzzyList is an interactive fuzzy-searchable list component
type FuzzyList struct {
	textInput       textinput.Model
	items           []string
	recentItems     []string
	filtered        []fuzzy.Match
	filteredRecent  []fuzzy.Match
	cursor          int
	maxVisible      int
	scrollOffset    int
	title           string
	loading         bool
	err             error
	inRecentSection bool
}

// NewFuzzyList creates a new fuzzy list component
func NewFuzzyList(title string) FuzzyList {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50
	ti.PromptStyle = PromptStyle
	ti.TextStyle = BaseStyle
	ti.Cursor.Style = CursorStyle

	return FuzzyList{
		textInput:       ti,
		items:           []string{},
		recentItems:     []string{},
		filtered:        []fuzzy.Match{},
		filteredRecent:  []fuzzy.Match{},
		cursor:          0,
		maxVisible:      10,
		title:           title,
		loading:         true,
		inRecentSection: true,
	}
}

// SetItems sets the list items
func (f *FuzzyList) SetItems(items []string) {
	f.items = items
	f.loading = false
	f.filterItems()
}

// SetRecentItems sets the recent items list
func (f *FuzzyList) SetRecentItems(items []string) {
	f.recentItems = items
	f.filterItems()
}

// SetError sets an error message
func (f *FuzzyList) SetError(err error) {
	f.err = err
	f.loading = false
}

// SetLoading sets the loading state
func (f *FuzzyList) SetLoading(loading bool) {
	f.loading = loading
}

// GetSelected returns the currently selected item
func (f *FuzzyList) GetSelected() string {
	if f.inRecentSection && len(f.filteredRecent) > 0 {
		if f.cursor < len(f.filteredRecent) {
			return f.filteredRecent[f.cursor].Str
		}
	}

	// Adjust cursor for main list
	mainCursor := f.cursor
	if len(f.filteredRecent) > 0 {
		mainCursor = f.cursor - len(f.filteredRecent)
	}

	if mainCursor >= 0 && mainCursor < len(f.filtered) {
		return f.filtered[mainCursor].Str
	}

	return ""
}

// GetInput returns the current input value
func (f *FuzzyList) GetInput() string {
	return f.textInput.Value()
}

// Reset clears the input and resets the list
func (f *FuzzyList) Reset() {
	f.textInput.SetValue("")
	f.cursor = 0
	f.scrollOffset = 0
	f.inRecentSection = true
	f.filterItems()
}

// Focus focuses the text input
func (f *FuzzyList) Focus() {
	f.textInput.Focus()
}

// Blur blurs the text input
func (f *FuzzyList) Blur() {
	f.textInput.Blur()
}

// totalItems returns the total number of visible items
func (f *FuzzyList) totalItems() int {
	return len(f.filteredRecent) + len(f.filtered)
}

func (f *FuzzyList) filterItems() {
	query := f.textInput.Value()

	// Filter recent items
	if len(f.recentItems) > 0 {
		if query == "" {
			f.filteredRecent = make([]fuzzy.Match, len(f.recentItems))
			for i, item := range f.recentItems {
				f.filteredRecent[i] = fuzzy.Match{
					Str:   item,
					Index: i,
				}
			}
		} else {
			f.filteredRecent = fuzzy.Find(query, f.recentItems)
		}
	} else {
		f.filteredRecent = []fuzzy.Match{}
	}

	// Filter main items (excluding recent items from main list)
	itemsWithoutRecent := make([]string, 0, len(f.items))
	recentSet := make(map[string]bool)
	for _, r := range f.recentItems {
		recentSet[r] = true
	}
	for _, item := range f.items {
		if !recentSet[item] {
			itemsWithoutRecent = append(itemsWithoutRecent, item)
		}
	}

	if query == "" {
		f.filtered = make([]fuzzy.Match, len(itemsWithoutRecent))
		for i, item := range itemsWithoutRecent {
			f.filtered[i] = fuzzy.Match{
				Str:   item,
				Index: i,
			}
		}
	} else {
		f.filtered = fuzzy.Find(query, itemsWithoutRecent)
	}

	// Reset cursor if out of bounds
	total := f.totalItems()
	if f.cursor >= total {
		f.cursor = 0
	}

	// Update section tracking
	f.inRecentSection = f.cursor < len(f.filteredRecent)
	f.scrollOffset = 0
}

// Update handles messages
func (f *FuzzyList) Update(msg tea.Msg) (FuzzyList, tea.Cmd) {
	var cmd tea.Cmd
	total := f.totalItems()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "ctrl+p":
			if f.cursor > 0 {
				f.cursor--
				f.inRecentSection = f.cursor < len(f.filteredRecent)
				if f.cursor < f.scrollOffset {
					f.scrollOffset = f.cursor
				}
			}
			return *f, nil

		case "down", "ctrl+n":
			if f.cursor < total-1 {
				f.cursor++
				f.inRecentSection = f.cursor < len(f.filteredRecent)
				if f.cursor >= f.scrollOffset+f.maxVisible {
					f.scrollOffset = f.cursor - f.maxVisible + 1
				}
			}
			return *f, nil

		case "pgup":
			f.cursor -= f.maxVisible
			if f.cursor < 0 {
				f.cursor = 0
			}
			f.inRecentSection = f.cursor < len(f.filteredRecent)
			f.scrollOffset = f.cursor
			return *f, nil

		case "pgdown":
			f.cursor += f.maxVisible
			if f.cursor >= total {
				f.cursor = total - 1
			}
			if f.cursor < 0 {
				f.cursor = 0
			}
			f.inRecentSection = f.cursor < len(f.filteredRecent)
			if f.cursor >= f.scrollOffset+f.maxVisible {
				f.scrollOffset = f.cursor - f.maxVisible + 1
			}
			return *f, nil
		}
	}

	// Update text input
	prevValue := f.textInput.Value()
	f.textInput, cmd = f.textInput.Update(msg)

	// If input changed, re-filter
	if f.textInput.Value() != prevValue {
		f.filterItems()
	}

	return *f, cmd
}

// View renders the fuzzy list
func (f *FuzzyList) View() string {
	var b strings.Builder

	// Title
	b.WriteString(LabelStyle.Render(f.title))
	b.WriteString("\n")

	// Text input
	inputStyle := InputBoxStyle
	if f.textInput.Focused() {
		inputStyle = FocusedInputStyle
	}
	b.WriteString(inputStyle.Render(f.textInput.View()))
	b.WriteString("\n")

	// Loading state
	if f.loading {
		b.WriteString(RenderLoading("Loading..."))
		return b.String()
	}

	// Error state
	if f.err != nil {
		b.WriteString(RenderError(f.err.Error()))
		return b.String()
	}

	total := f.totalItems()

	// No results
	if total == 0 {
		if len(f.items) == 0 && len(f.recentItems) == 0 {
			b.WriteString(InfoStyle.Render("  No items available"))
		} else {
			b.WriteString(InfoStyle.Render("  No matches found"))
		}
		return b.String()
	}

	// Build combined list for rendering
	type listItem struct {
		match    fuzzy.Match
		isRecent bool
		index    int // index in combined list
	}

	allItems := make([]listItem, 0, total)
	for i, match := range f.filteredRecent {
		allItems = append(allItems, listItem{match: match, isRecent: true, index: i})
	}
	for i, match := range f.filtered {
		allItems = append(allItems, listItem{match: match, isRecent: false, index: len(f.filteredRecent) + i})
	}

	// Render visible items
	end := f.scrollOffset + f.maxVisible
	if end > len(allItems) {
		end = len(allItems)
	}

	// Track if we need section headers
	showRecentHeader := len(f.filteredRecent) > 0 && f.scrollOffset < len(f.filteredRecent)
	showAllHeader := len(f.filtered) > 0

	inRecentSection := true
	for i := f.scrollOffset; i < end; i++ {
		item := allItems[i]

		// Section headers
		if showRecentHeader && i == f.scrollOffset && item.isRecent {
			b.WriteString(InfoStyle.Render("  ⏱ Recent"))
			b.WriteString("\n")
		}
		if showAllHeader && !item.isRecent && inRecentSection {
			inRecentSection = false
			b.WriteString(InfoStyle.Render("  📋 All"))
			b.WriteString("\n")
		}

		isSelected := i == f.cursor

		// Build the display string with highlighted matches
		var display string
		if len(item.match.MatchedIndexes) > 0 && f.textInput.Value() != "" {
			display = f.highlightMatches(item.match.Str, item.match.MatchedIndexes)
		} else {
			display = item.match.Str
		}

		if isSelected {
			b.WriteString(SelectedItemStyle.Render("  ▸ " + display))
		} else {
			b.WriteString(ListItemStyle.Render("    " + display))
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if total > f.maxVisible {
		current := f.cursor + 1
		b.WriteString(InfoStyle.Render("  [" + itoa(current) + "/" + itoa(total) + "]"))
	}

	return b.String()
}

// itoa is a simple int to string conversion
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var result strings.Builder
	for n > 0 {
		result.WriteString(string(rune('0' + n%10)))
		n /= 10
	}
	// Reverse
	s := result.String()
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func (f *FuzzyList) highlightMatches(str string, indexes []int) string {
	if len(indexes) == 0 {
		return str
	}

	// Create a map of highlighted indices
	highlighted := make(map[int]bool)
	for _, idx := range indexes {
		highlighted[idx] = true
	}

	var result strings.Builder
	for i, char := range str {
		if highlighted[i] {
			result.WriteString(MatchStyle.Render(string(char)))
		} else {
			result.WriteRune(char)
		}
	}

	return result.String()
}
