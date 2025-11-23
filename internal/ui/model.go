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
)

type Model struct {
	client              *api.Client
	viewport            viewport.Model
	textInput           textinput.Model
	translations        []api.Translation
	selectedTranslation string
	currentBook         int
	currentChapter      int
	currentBookName     string
	books               []api.Book
	content             string
	mode                viewMode
	width               int
	height              int
	ready               bool
	err                 error
	loading             bool
	comparisonTranslations []string
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
		case "/":
			if m.mode == modeReader {
				m.mode = modeSearch
				m.textInput.Focus()
				return m, nil
			}
		case "c":
			if m.mode == modeReader {
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
			if m.mode == modeReader {
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
			if m.mode == modeSearch {
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
			if m.mode == modeSearch || m.mode == modeTranslationSelect {
				m.mode = modeReader
				return m, nil
			}
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
		m.content = formatChapter(msg.verses, m.currentBookName, m.currentChapter)
		m.viewport.SetContent(m.content)
		m.viewport.GotoTop()

	case parallelVersesLoadedMsg:
		m.loading = false
		m.content = formatParallelVerses(msg.verses, m.comparisonTranslations, m.currentBookName, m.currentChapter)
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
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
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
	} else {
		help = helpStyle.Render("/: search | c: compare | t: translation | n: next | p: prev | q: quit")
	}

	var errorMsg string
	if m.err != nil {
		errorMsg = "\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	return fmt.Sprintf("%s\n%s\n%s%s", header, m.viewport.View(), help, errorMsg)
}

func formatChapter(verses []api.Verse, bookName string, chapter int) string {
	verseStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("147"))

	textStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Width(80)

	var sb strings.Builder

	for _, v := range verses {
		// Remove HTML tags
		text := stripHTMLTags(v.Text)
		verseNum := verseStyle.Render(fmt.Sprintf("%d", v.Verse))
		verseText := textStyle.Render(text)
		sb.WriteString(fmt.Sprintf("%s  %s\n\n", verseNum, verseText))
	}

	return sb.String()
}

func formatParallelVerses(versesMap map[string][]api.Verse, translations []string, bookName string, chapter int) string {
	translationStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	verseNumStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("147")).
		Bold(true)

	textStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Width(80)

	var sb strings.Builder

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
		sb.WriteString(strings.Repeat("─", 80) + "\n")

		for _, trans := range translations {
			verses, ok := versesMap[trans]
			if !ok {
				continue
			}

			for _, v := range verses {
				if v.Verse == i {
					text := stripHTMLTags(v.Text)
					transLabel := translationStyle.Render(fmt.Sprintf("[%s]", trans))
					verseText := textStyle.Render(text)
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
