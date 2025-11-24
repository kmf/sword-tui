package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sword-tui/internal/api"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewMode int

const (
	modeReader viewMode = iota
	modeSearch
	modeComparison
	modeTranslationSelect
	modeSidebar
)

type Model struct {
	client                 *api.Client
	viewport               viewport.Model
	textInput              textinput.Model
	translations           []api.Translation
	selectedTranslation    string
	currentBook            int
	currentChapter         int
	currentBookName        string
	books                  []api.Book
	content                string
	mode                   viewMode
	width                  int
	height                 int
	ready                  bool
	err                    error
	loading                bool
	comparisonTranslations []string
	sidebarSelected        int
	showSidebar            bool
	currentVerses          []api.Verse
	currentParallelVerses  map[string][]api.Verse
	highlightedVerse       int // Currently highlighted verse number
	// Miller columns state
	millerColumn           int // 0=books, 1=chapters, 2=verses
	millerBookIdx          int
	millerChapterIdx       int
	millerVerseIdx         int
	showMillerColumns      bool
}

type errMsg struct{ err error }
type translationsLoadedMsg struct{ translations []api.Translation }
type booksLoadedMsg struct{ books []api.Book }
type chapterLoadedMsg struct{ verses []api.Verse }
type parallelVersesLoadedMsg struct{ verses map[string][]api.Verse }

func (e errMsg) Error() string { return e.err.Error() }

func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "Enter verse reference (e.g., 1 1:1 or Gen 1:1)"
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 50

	return Model{
		client:              api.NewClient(),
		textInput:           ti,
		selectedTranslation: "NLT",
		currentBook:         1,
		currentChapter:      1,
		currentBookName:     "Genesis",
		mode:                modeReader,
		comparisonTranslations: []string{"NLT", "KJV", "WEB"},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadTranslations(m.client),
		loadBooks(m.client, m.selectedTranslation),
		loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter),
	)
}

func loadTranslations(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		translations, err := client.GetTranslations()
		if err != nil {
			return errMsg{err}
		}
		return translationsLoadedMsg{translations}
	}
}

func loadBooks(client *api.Client, translation string) tea.Cmd {
	return func() tea.Msg {
		books, err := client.GetBooks(translation)
		if err != nil {
			return errMsg{err}
		}
		return booksLoadedMsg{books}
	}
}

func loadChapter(client *api.Client, translation string, book, chapter int) tea.Cmd {
	return func() tea.Msg {
		verses, err := client.GetChapter(translation, book, chapter)
		if err != nil {
			return errMsg{err}
		}
		return chapterLoadedMsg{verses}
	}
}

func loadParallelVerses(client *api.Client, translations []string, book, chapter int, verses []int) tea.Cmd {
	return func() tea.Msg {
		req := api.ParallelVerseRequest{
			Translations: translations,
			Verses:       verses,
			Chapter:      chapter,
			Book:         book,
		}
		result, err := client.GetParallelVerses(req)
		if err != nil {
			return errMsg{err}
		}
		return parallelVersesLoadedMsg{result}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "[":
			if m.mode == modeReader {
				m.showSidebar = !m.showSidebar
				if m.showSidebar && m.books != nil {
					// Find the index of the current book in the books array
					for i, book := range m.books {
						if book.BookID == m.currentBook {
							m.sidebarSelected = i
							break
						}
					}
				}
				return m, nil
			}
		case "v":
			if m.mode == modeReader && !m.showSidebar {
				m.showMillerColumns = !m.showMillerColumns
				if m.showMillerColumns && m.books != nil {
					// Initialize Miller columns with current position
					for i, book := range m.books {
						if book.BookID == m.currentBook {
							m.millerBookIdx = i
							break
						}
					}
					m.millerChapterIdx = m.currentChapter - 1
					m.millerVerseIdx = 0
					m.millerColumn = 0
				}
				return m, nil
			}
		case "/":
			if m.mode == modeReader && !m.showSidebar {
				m.mode = modeSearch
				m.textInput.Focus()
				return m, nil
			}
		case "up", "k":
			if m.showMillerColumns {
				switch m.millerColumn {
				case 0: // Books column
					if m.millerBookIdx > 0 {
						m.millerBookIdx--
						m.millerChapterIdx = 0
						m.millerVerseIdx = 0
					}
				case 1: // Chapters column
					if m.millerChapterIdx > 0 {
						m.millerChapterIdx--
						m.millerVerseIdx = 0
					}
				case 2: // Verses column
					if m.millerVerseIdx > 0 {
						m.millerVerseIdx--
					}
				}
				return m, nil
			} else if m.showSidebar && m.sidebarSelected > 0 {
				m.sidebarSelected--
				return m, nil
			}
		case "down", "j":
			if m.showMillerColumns && m.books != nil {
				switch m.millerColumn {
				case 0: // Books column
					if m.millerBookIdx < len(m.books)-1 {
						m.millerBookIdx++
						m.millerChapterIdx = 0
						m.millerVerseIdx = 0
					}
				case 1: // Chapters column
					selectedBook := m.books[m.millerBookIdx]
					if m.millerChapterIdx < selectedBook.Chapters-1 {
						m.millerChapterIdx++
						m.millerVerseIdx = 0
					}
				case 2: // Verses column
					if m.currentVerses != nil && m.millerVerseIdx < len(m.currentVerses)-1 {
						m.millerVerseIdx++
					}
				}
				return m, nil
			} else if m.showSidebar && m.books != nil && m.sidebarSelected < len(m.books)-1 {
				m.sidebarSelected++
				return m, nil
			}
		case "left", "h":
			if m.showMillerColumns && m.millerColumn > 0 {
				m.millerColumn--
				return m, nil
			}
		case "right", "l":
			if m.showMillerColumns {
				if m.millerColumn < 2 {
					m.millerColumn++
					// When moving to verses column, load the chapter if not already loaded
					if m.millerColumn == 2 {
						selectedBook := m.books[m.millerBookIdx]
						selectedChapter := m.millerChapterIdx + 1
						// Only load if different from current
						if selectedBook.BookID != m.currentBook || selectedChapter != m.currentChapter {
							return m, loadChapter(m.client, m.selectedTranslation, selectedBook.BookID, selectedChapter)
						}
					}
				}
				return m, nil
			}
		case "c":
			if m.mode == modeReader && !m.showSidebar {
				m.mode = modeComparison
				verses := []int{}
				for i := 1; i <= 31; i++ {
					verses = append(verses, i)
				}
				return m, loadParallelVerses(m.client, m.comparisonTranslations, m.currentBook, m.currentChapter, verses)
			}
		case "r":
			if m.mode != modeReader {
				m.mode = modeReader
				return m, nil
			}
		case "t":
			if m.mode == modeReader && !m.showSidebar {
				m.mode = modeTranslationSelect
				return m, nil
			}
		case "n":
			if m.mode == modeReader && m.books != nil {
				for _, book := range m.books {
					if book.BookID == m.currentBook {
						if m.currentChapter < book.Chapters {
							m.currentChapter++
							m.loading = true
							return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
						}
					}
				}
			}
		case "p":
			if m.mode == modeReader && m.currentChapter > 1 {
				m.currentChapter--
				m.loading = true
				return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
			}
		case "enter":
			if m.showMillerColumns && m.books != nil && m.currentVerses != nil {
				// Navigate to the selected verse
				selectedBook := m.books[m.millerBookIdx]
				selectedChapter := m.millerChapterIdx + 1
				m.currentBook = selectedBook.BookID
				m.currentBookName = selectedBook.Name
				m.currentChapter = selectedChapter
				m.showMillerColumns = false
				m.loading = true
				// Scroll viewport to the selected verse
				return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
			} else if m.showSidebar && m.books != nil {
				// Select book from sidebar
				if m.sidebarSelected < len(m.books) {
					m.currentBook = m.books[m.sidebarSelected].BookID
					m.currentBookName = m.books[m.sidebarSelected].Name
					m.currentChapter = 1
					m.showSidebar = false
					m.loading = true
					return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
				}
			} else if m.mode == modeSearch {
				input := m.textInput.Value()
				book, chapter, verse, err := parseReference(input)
				if err == nil {
					m.currentBook = book
					m.currentChapter = chapter
					m.mode = modeReader
					m.loading = true
					m.textInput.SetValue("")
					if verse > 0 {
						// Highlight specific verse
						return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
					}
					return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
				}
			} else if m.mode == modeTranslationSelect {
				// Simple translation selection (cycle through common ones)
				translations := []string{"NLT", "KJV", "ASV", "WEB", "YLT", "DARBY"}
				for i, t := range translations {
					if t == m.selectedTranslation {
						m.selectedTranslation = translations[(i+1)%len(translations)]
						break
					}
				}
				m.mode = modeReader
				m.loading = true
				return m, tea.Batch(
					loadBooks(m.client, m.selectedTranslation),
					loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter),
				)
			}
		case "esc":
			if m.showMillerColumns {
				m.showMillerColumns = false
				return m, nil
			}
			if m.showSidebar {
				m.showSidebar = false
				return m, nil
			}
			if m.mode == modeSearch || m.mode == modeTranslationSelect {
				m.mode = modeReader
				return m, nil
			}
		}

	case tea.MouseMsg:
		if m.showSidebar {
			if msg.Type == tea.MouseLeft {
				// Handle mouse clicks in sidebar
				// Sidebar is 30 chars wide + 4 for border/padding
				if msg.X < 34 {
					// Calculate which book was clicked
					// Account for header, padding, and section titles
					clickY := msg.Y - 5 // Adjust for header and padding

					if clickY >= 0 && m.books != nil {
						bookIndex := 0
						currentY := 0

						// Skip "OLD TESTAMENT" header (2 lines)
						if clickY < 2 {
							return m, nil
						}
						currentY = 2

						// Old Testament books
						for i, book := range m.books {
							if book.BookID > 39 {
								break
							}
							if clickY == currentY {
								bookIndex = i
								m.sidebarSelected = bookIndex
								m.currentBook = m.books[bookIndex].BookID
								m.currentBookName = m.books[bookIndex].Name
								m.currentChapter = 1
								m.showSidebar = false
								m.loading = true
								return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
							}
							currentY++
						}

						// Skip "NEW TESTAMENT" header (2 lines)
						currentY += 2

						// New Testament books
						for i, book := range m.books {
							if book.BookID < 40 {
								continue
							}
							if clickY == currentY {
								bookIndex = i
								m.sidebarSelected = bookIndex
								m.currentBook = m.books[bookIndex].BookID
								m.currentBookName = m.books[bookIndex].Name
								m.currentChapter = 1
								m.showSidebar = false
								m.loading = true
								return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
							}
							currentY++
						}
					}
				}
			}
		} else {
			// Pass mouse events to viewport for scrolling
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-6)
			m.viewport.YPosition = 4
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 6
		}

		// Reformat content with new width
		if m.currentVerses != nil {
			m.content = formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerse)
		} else if m.currentParallelVerses != nil {
			m.content = formatParallelVerses(m.currentParallelVerses, m.comparisonTranslations, m.currentBookName, m.currentChapter, m.width)
		}
		m.viewport.SetContent(m.content)

	case translationsLoadedMsg:
		m.translations = msg.translations

	case booksLoadedMsg:
		m.books = msg.books
		for _, book := range m.books {
			if book.BookID == m.currentBook {
				m.currentBookName = book.Name
				break
			}
		}

	case chapterLoadedMsg:
		m.loading = false
		m.currentVerses = msg.verses
		m.currentParallelVerses = nil
		// Initialize highlighted verse to first verse
		if len(msg.verses) > 0 {
			m.highlightedVerse = msg.verses[0].Verse
		} else {
			m.highlightedVerse = 1
		}
		m.content = formatChapter(msg.verses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerse)
		m.viewport.SetContent(m.content)
		m.viewport.GotoTop()

	case parallelVersesLoadedMsg:
		m.loading = false
		m.currentParallelVerses = msg.verses
		m.currentVerses = nil
		m.content = formatParallelVerses(msg.verses, m.comparisonTranslations, m.currentBookName, m.currentChapter, m.width)
		m.viewport.SetContent(m.content)
		m.viewport.GotoTop()

	case errMsg:
		m.err = msg.err
		m.loading = false
	}

	if m.mode == modeSearch {
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		oldYOffset := m.viewport.YOffset
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

		// Update highlighted verse based on viewport position
		if m.currentVerses != nil && oldYOffset != m.viewport.YOffset {
			newHighlightedVerse := m.calculateHighlightedVerse()
			if newHighlightedVerse != m.highlightedVerse {
				m.highlightedVerse = newHighlightedVerse
				// Reformat content with new highlighted verse
				m.content = formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerse)
				m.viewport.SetContent(m.content)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("240"))

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	var header string
	if m.mode == modeSearch {
		header = headerStyle.Render("Search - Enter verse reference") + "\n" + m.textInput.View()
	} else if m.mode == modeTranslationSelect {
		header = headerStyle.Render("Select Translation - Press Enter to cycle: " + m.selectedTranslation)
	} else if m.mode == modeComparison {
		header = headerStyle.Render(fmt.Sprintf("Comparison View - %s %d", m.currentBookName, m.currentChapter))
	} else {
		title := titleStyle.Render(fmt.Sprintf("%s %s %d", m.selectedTranslation, m.currentBookName, m.currentChapter))
		header = headerStyle.Render(title)
	}

	var help string
	if m.loading {
		help = helpStyle.Render("Loading...")
	} else if m.showMillerColumns {
		help = helpStyle.Render("↑/↓ or j/k: navigate | ←/→ or h/l: switch column | enter: select | v/esc: close")
	} else if m.showSidebar {
		help = helpStyle.Render("↑/↓ or j/k: navigate | enter: select | [/esc: close")
	} else {
		help = helpStyle.Render("[: books | v: verse picker | /: search | c: compare | t: translation | n: next | p: prev | q: quit")
	}

	var errorMsg string
	if m.err != nil {
		errorMsg = "\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	mainContent := fmt.Sprintf("%s\n%s\n%s%s", header, m.viewport.View(), help, errorMsg)

	if m.showMillerColumns {
		millerColumns := m.renderMillerColumns()
		// Overlay Miller columns on top of the main content
		return overlayContent(mainContent, millerColumns, m.width, m.height)
	}

	if m.showSidebar {
		sidebar := m.renderSidebar()
		// Overlay the sidebar on top of the main content
		return overlayContent(mainContent, sidebar, m.width, m.height)
	}

	return mainContent
}

func (m Model) calculateHighlightedVerse() int {
	if m.currentVerses == nil || len(m.currentVerses) == 0 {
		return 1
	}

	// Each verse takes approximately 3-4 lines (verse number + text + blank line)
	// We'll calculate which verse is at the top of the viewport
	yOffset := m.viewport.YOffset

	// Count lines to find which verse we're at
	currentLine := 0
	for _, verse := range m.currentVerses {
		text := stripHTMLTags(verse.Text)
		verseNumStr := fmt.Sprintf("%d", verse.Verse)
		indent := len(verseNumStr) + 2

		// Calculate available width
		textWidth := m.width - 10
		if textWidth < 40 {
			textWidth = 40
		}

		// Calculate how many lines this verse takes
		wrappedText := wrapTextWithIndent(text, textWidth, indent)
		linesInVerse := strings.Count(wrappedText, "\n") + 1

		// Add verse number line + wrapped text lines + blank line
		verseTotalLines := linesInVerse + 2

		// If the current line + verse lines exceeds yOffset, this is our verse
		if currentLine+verseTotalLines > yOffset {
			return verse.Verse
		}

		currentLine += verseTotalLines
	}

	// If we've scrolled past all verses, return the last one
	if len(m.currentVerses) > 0 {
		return m.currentVerses[len(m.currentVerses)-1].Verse
	}

	return 1
}

func overlayContent(base, overlay string, width, height int) string {
	// Split both strings into lines
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Ensure we have enough base lines
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}

	// Overlay the sidebar lines onto the base lines
	for i := 0; i < len(overlayLines) && i < len(baseLines); i++ {
		overlayLine := overlayLines[i]
		baseLine := baseLines[i]

		// Get the visual width of the overlay line (accounting for ANSI codes)
		overlayWidth := lipgloss.Width(overlayLine)

		// Pad base line to full width if needed
		baseWidth := lipgloss.Width(baseLine)
		if baseWidth < width {
			baseLine += strings.Repeat(" ", width-baseWidth)
		}

		// Replace the beginning of the base line with the overlay line
		if overlayWidth > 0 {
			// For lines with ANSI codes, we need to be careful
			// Just replace the visual portion
			if len(overlayLine) > 0 {
				// Simple approach: trim base line and prepend overlay
				baseRunes := []rune(baseLine)
				visualPos := 0
				runePos := 0
				inAnsi := false

				// Count runes until we reach the visual width
				for runePos < len(baseRunes) && visualPos < overlayWidth {
					if baseRunes[runePos] == '\x1b' {
						inAnsi = true
					}
					if !inAnsi {
						visualPos++
					}
					if inAnsi && baseRunes[runePos] == 'm' {
						inAnsi = false
					}
					runePos++
				}

				// Combine overlay with remaining base content
				if runePos < len(baseRunes) {
					baseLines[i] = overlayLine + string(baseRunes[runePos:])
				} else {
					baseLines[i] = overlayLine
				}
			}
		}
	}

	return strings.Join(baseLines, "\n")
}

func (m Model) renderMillerColumns() string {
	columnWidth := 30

	columnStyle := lipgloss.NewStyle().
		Width(columnWidth).
		Height(m.height - 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1)

	activeColumnStyle := lipgloss.NewStyle().
		Width(columnWidth).
		Height(m.height - 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("86")).
		Padding(1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	// Column 1: Books
	var booksContent strings.Builder
	booksContent.WriteString(headerStyle.Render("BOOKS") + "\n\n")

	if m.books != nil {
		visibleItems := m.height - 8
		if visibleItems < 5 {
			visibleItems = 5
		}

		startIdx := m.millerBookIdx - visibleItems/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := startIdx + visibleItems
		if endIdx > len(m.books) {
			endIdx = len(m.books)
			startIdx = endIdx - visibleItems
			if startIdx < 0 {
				startIdx = 0
			}
		}

		if startIdx > 0 {
			booksContent.WriteString(normalStyle.Render(fmt.Sprintf("... (%d)\n", startIdx)))
		}

		for i := startIdx; i < endIdx && i < len(m.books); i++ {
			book := m.books[i]
			name := book.Name
			if len(name) > 26 {
				name = name[:23] + "..."
			}

			if i == m.millerBookIdx {
				booksContent.WriteString(selectedStyle.Render("> "+name) + "\n")
			} else {
				booksContent.WriteString(normalStyle.Render("  "+name) + "\n")
			}
		}

		if endIdx < len(m.books) {
			booksContent.WriteString(normalStyle.Render(fmt.Sprintf("... (%d)\n", len(m.books)-endIdx)))
		}
	}

	var booksColumn string
	if m.millerColumn == 0 {
		booksColumn = activeColumnStyle.Render(booksContent.String())
	} else {
		booksColumn = columnStyle.Render(booksContent.String())
	}

	// Column 2: Chapters
	var chaptersContent strings.Builder
	chaptersContent.WriteString(headerStyle.Render("CHAPTERS") + "\n\n")

	if m.books != nil && m.millerBookIdx < len(m.books) {
		selectedBook := m.books[m.millerBookIdx]
		for i := 0; i < selectedBook.Chapters; i++ {
			chapterNum := fmt.Sprintf("Chapter %d", i+1)
			if i == m.millerChapterIdx {
				chaptersContent.WriteString(selectedStyle.Render("> "+chapterNum) + "\n")
			} else {
				chaptersContent.WriteString(normalStyle.Render("  "+chapterNum) + "\n")
			}
		}
	}

	var chaptersColumn string
	if m.millerColumn == 1 {
		chaptersColumn = activeColumnStyle.Render(chaptersContent.String())
	} else {
		chaptersColumn = columnStyle.Render(chaptersContent.String())
	}

	// Column 3: Verses
	var versesContent strings.Builder
	versesContent.WriteString(headerStyle.Render("VERSES") + "\n\n")

	if m.currentVerses != nil {
		visibleItems := m.height - 8
		if visibleItems < 5 {
			visibleItems = 5
		}

		startIdx := m.millerVerseIdx - visibleItems/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := startIdx + visibleItems
		if endIdx > len(m.currentVerses) {
			endIdx = len(m.currentVerses)
			startIdx = endIdx - visibleItems
			if startIdx < 0 {
				startIdx = 0
			}
		}

		if startIdx > 0 {
			versesContent.WriteString(normalStyle.Render(fmt.Sprintf("... (%d)\n", startIdx)))
		}

		for i := startIdx; i < endIdx && i < len(m.currentVerses); i++ {
			verse := m.currentVerses[i]
			text := stripHTMLTags(verse.Text)
			if len(text) > 23 {
				text = text[:20] + "..."
			}
			verseLabel := fmt.Sprintf("%d. %s", verse.Verse, text)

			if i == m.millerVerseIdx {
				versesContent.WriteString(selectedStyle.Render("> "+verseLabel) + "\n")
			} else {
				versesContent.WriteString(normalStyle.Render("  "+verseLabel) + "\n")
			}
		}

		if endIdx < len(m.currentVerses) {
			versesContent.WriteString(normalStyle.Render(fmt.Sprintf("... (%d)\n", len(m.currentVerses)-endIdx)))
		}
	} else {
		versesContent.WriteString(normalStyle.Render("  Loading..."))
	}

	var versesColumn string
	if m.millerColumn == 2 {
		versesColumn = activeColumnStyle.Render(versesContent.String())
	} else {
		versesColumn = columnStyle.Render(versesContent.String())
	}

	// Join the three columns horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, booksColumn, chaptersColumn, versesColumn)
}

func (m Model) renderSidebar() string {
	sidebarStyle := lipgloss.NewStyle().
		Width(30).
		Height(m.height - 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	oldTestamentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	var sb strings.Builder
	sb.WriteString(oldTestamentStyle.Render("OLD TESTAMENT") + "\n\n")

	if m.books != nil {
		// Old Testament (books 1-39)
		for i, book := range m.books {
			if book.BookID > 39 {
				break
			}
			if i == m.sidebarSelected {
				sb.WriteString(selectedStyle.Render("> "+book.Name) + "\n")
			} else {
				sb.WriteString(normalStyle.Render("  "+book.Name) + "\n")
			}
		}

		sb.WriteString("\n" + oldTestamentStyle.Render("NEW TESTAMENT") + "\n\n")

		// New Testament (books 40-66)
		for i, book := range m.books {
			if book.BookID < 40 {
				continue
			}
			if i == m.sidebarSelected {
				sb.WriteString(selectedStyle.Render("> "+book.Name) + "\n")
			} else {
				sb.WriteString(normalStyle.Render("  "+book.Name) + "\n")
			}
		}
	}

	return sidebarStyle.Render(sb.String())
}

func formatChapter(verses []api.Verse, bookName string, chapter int, width int, highlightedVerse int) string {
	verseStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("147"))

	highlightedVerseStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	textStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	highlightedTextStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Background(lipgloss.Color("235"))

	var sb strings.Builder

	// Calculate available width for text (accounting for verse number and spacing)
	textWidth := width - 10 // Reserve space for verse number and padding
	if textWidth < 40 {
		textWidth = 40 // Minimum width
	}

	for _, v := range verses {
		// Remove HTML tags
		text := stripHTMLTags(v.Text)
		verseNumStr := fmt.Sprintf("%d", v.Verse)

		// Use highlighted style if this is the current verse
		var verseNum string
		var verseText string
		if v.Verse == highlightedVerse {
			verseNum = highlightedVerseStyle.Render(verseNumStr)

			// Calculate indent for wrapped lines (verse number length + 2 spaces)
			indent := len(verseNumStr) + 2
			wrappedText := wrapTextWithIndent(text, textWidth, indent)
			verseText = highlightedTextStyle.Render(wrappedText)
		} else {
			verseNum = verseStyle.Render(verseNumStr)

			// Calculate indent for wrapped lines (verse number length + 2 spaces)
			indent := len(verseNumStr) + 2
			wrappedText := wrapTextWithIndent(text, textWidth, indent)
			verseText = textStyle.Render(wrappedText)
		}

		sb.WriteString(fmt.Sprintf("%s  %s\n\n", verseNum, verseText))
	}

	return sb.String()
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	var currentLine strings.Builder
	currentLength := 0

	words := strings.Fields(text)
	for i, word := range words {
		wordLen := len(word)

		// If adding this word would exceed width, start a new line
		if currentLength > 0 && currentLength+1+wordLen > width {
			result.WriteString(currentLine.String())
			result.WriteString("\n")
			currentLine.Reset()
			currentLength = 0
		}

		// Add space before word (except at start of line)
		if currentLength > 0 {
			currentLine.WriteString(" ")
			currentLength++
		}

		currentLine.WriteString(word)
		currentLength += wordLen

		// If this is the last word, write it out
		if i == len(words)-1 {
			result.WriteString(currentLine.String())
		}
	}

	return result.String()
}

func wrapTextWithIndent(text string, width int, indent int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	var currentLine strings.Builder
	currentLength := 0
	isFirstLine := true

	words := strings.Fields(text)
	for i, word := range words {
		wordLen := len(word)

		// If adding this word would exceed width, start a new line
		if currentLength > 0 && currentLength+1+wordLen > width {
			result.WriteString(currentLine.String())
			result.WriteString("\n")
			currentLine.Reset()
			currentLength = 0
			isFirstLine = false
		}

		// Add indent for continuation lines
		if currentLength == 0 && !isFirstLine {
			currentLine.WriteString(strings.Repeat(" ", indent))
			currentLength = indent
		}

		// Add space before word (except at start of line)
		if currentLength > 0 && currentLength != indent {
			currentLine.WriteString(" ")
			currentLength++
		}

		currentLine.WriteString(word)
		currentLength += wordLen

		// If this is the last word, write it out
		if i == len(words)-1 {
			result.WriteString(currentLine.String())
		}
	}

	return result.String()
}

func formatParallelVerses(versesMap map[string][]api.Verse, translations []string, bookName string, chapter int, width int) string {
	translationStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	verseNumStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("147")).
		Bold(true)

	textStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	var sb strings.Builder

	// Calculate available width for text (accounting for translation label)
	textWidth := width - 15 // Reserve space for [TRANS] label and padding
	if textWidth < 40 {
		textWidth = 40 // Minimum width
	}

	// Get max verses from any translation
	maxVerses := 0
	for _, verses := range versesMap {
		if len(verses) > maxVerses {
			maxVerses = len(verses)
		}
	}

	// Display verse by verse across translations
	for i := 1; i <= maxVerses; i++ {
		sb.WriteString(verseNumStyle.Render(fmt.Sprintf("Verse %d", i)) + "\n")
		separatorWidth := width
		if separatorWidth > 80 {
			separatorWidth = 80
		}
		sb.WriteString(strings.Repeat("─", separatorWidth) + "\n")

		for _, trans := range translations {
			verses, ok := versesMap[trans]
			if !ok {
				continue
			}

			for _, v := range verses {
				if v.Verse == i {
					text := stripHTMLTags(v.Text)
					transLabelStr := fmt.Sprintf("[%s]", trans)
					// Calculate indent for wrapped lines (translation label length + 1 space)
					indent := len(transLabelStr) + 1
					wrappedText := wrapTextWithIndent(text, textWidth, indent)
					transLabel := translationStyle.Render(transLabelStr)
					verseText := textStyle.Render(wrappedText)
					sb.WriteString(fmt.Sprintf("%s %s\n\n", transLabel, verseText))
					break
				}
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func stripHTMLTags(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}

func parseReference(ref string) (book, chapter, verse int, err error) {
	// Handle formats like "1 1:1" or "Gen 1:1" or "1:1" or just "1"
	ref = strings.TrimSpace(ref)

	// Simple numeric parser: "book chapter:verse"
	parts := strings.Fields(ref)
	if len(parts) == 0 {
		return 0, 0, 0, fmt.Errorf("empty reference")
	}

	// Try to parse first part as book number
	book, err = strconv.Atoi(parts[0])
	if err != nil {
		// Could be book name, default to Genesis (1) for now
		book = 1
	}

	if len(parts) >= 2 {
		// Parse chapter:verse
		chapterVerse := parts[1]
		cvParts := strings.Split(chapterVerse, ":")
		chapter, err = strconv.Atoi(cvParts[0])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid chapter")
		}
		if len(cvParts) > 1 {
			verse, _ = strconv.Atoi(cvParts[1])
		}
	} else {
		// Only book provided
		chapter = 1
	}

	return book, chapter, verse, nil
}
