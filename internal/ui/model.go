package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sword-tui/internal/api"
	"sword-tui/internal/theme"
	"sword-tui/internal/version"

	"github.com/atotto/clipboard"
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
	modeCacheManager
	modeThemeSelect
	modeAbout
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
	highlightedVerseStart  int // Start of highlighted verse range
	highlightedVerseEnd    int // End of highlighted verse range
	// Miller columns state
	millerColumn           int // 0=books, 1=chapters, 2=verses
	millerBookIdx          int
	millerChapterIdx       int
	millerVerseIdx         int
	showMillerColumns      bool
	millerFilterInput      textinput.Model
	millerFilter           string
	millerFilteredBooks    []api.Book
	millerFilteredVerses   []api.Verse
	millerFilterMode       bool // When true, all keys go to filter input
	// Cache management state
	cache                  CacheInterface
	cachedTranslations     []string
	cacheSelected          int
	downloadingTranslation string
	// Translation selection state
	translationSelected    int
	// Theme state
	currentTheme           theme.Theme
	themeSelected          int
}

type CacheInterface interface {
	IsCached(translation string) bool
	GetChapter(translation string, book, chapter int) ([]api.Verse, error)
	GetVerse(translation string, book, chapter, verse int) (*api.Verse, error)
	DownloadTranslation(translation string) error
	ListCached() ([]string, error)
	GetCacheSize() (int64, error)
	RemoveTranslation(translation string) error
	ClearCache() error
}

type errMsg struct{ err error }
type translationsLoadedMsg struct{ translations []api.Translation }
type booksLoadedMsg struct{ books []api.Book }
type chapterLoadedMsg struct{ verses []api.Verse }
type parallelVersesLoadedMsg struct{ verses map[string][]api.Verse }
type cacheListLoadedMsg struct{ translations []string }
type downloadCompleteMsg struct{ translation string }
type downloadErrorMsg struct{ translation string; err error }

func (e errMsg) Error() string { return e.err.Error() }

func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "Enter verse reference (e.g., 1 1:1 or Gen 1:1)"
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 50

	millerFilter := textinput.New()
	millerFilter.Placeholder = "Type to filter..."
	millerFilter.CharLimit = 50
	millerFilter.Width = 25

	return Model{
		client:              api.NewClient(),
		textInput:           ti,
		millerFilterInput:   millerFilter,
		selectedTranslation: "NLT",
		currentBook:         1,
		currentChapter:      1,
		currentBookName:     "Genesis",
		mode:                modeReader,
		comparisonTranslations: []string{"NLT", "KJV", "WEB"},
		currentTheme:        theme.CatppuccinMocha,
		themeSelected:       0,
	}
}

func (m *Model) SetCache(cache CacheInterface) {
	m.cache = cache
	if cache != nil {
		// Set cache on API client too
		m.client.SetCache(cache)
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

func loadCachedList(cache CacheInterface) tea.Cmd {
	return func() tea.Msg {
		translations, err := cache.ListCached()
		if err != nil {
			return errMsg{err}
		}
		return cacheListLoadedMsg{translations}
	}
}

func downloadTranslation(cache CacheInterface, translation string) tea.Cmd {
	return func() tea.Msg {
		err := cache.DownloadTranslation(translation)
		if err != nil {
			return downloadErrorMsg{translation, err}
		}
		return downloadCompleteMsg{translation}
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
					// Reset filter
					m.millerFilterInput.SetValue("")
					m.millerFilter = ""
					m.millerFilteredBooks = nil
					m.millerFilteredVerses = nil
					m.millerFilterMode = false
				}
				return m, nil
			}
		case "/":
			if m.showMillerColumns {
				// Toggle filter mode in Miller columns
				m.millerFilterMode = !m.millerFilterMode
				if m.millerFilterMode {
					m.millerFilterInput.Focus()
				} else {
					m.millerFilterInput.Blur()
				}
				return m, nil
			} else if m.mode == modeReader {
				// Close sidebar if open when entering search mode
				m.showSidebar = false
				m.mode = modeSearch
				m.textInput.Focus()
				return m, nil
			}
		case "up", "k":
			if m.mode == modeTranslationSelect && m.translations != nil && m.translationSelected > 0 {
				m.translationSelected--
				return m, nil
			} else if m.mode == modeThemeSelect && m.themeSelected > 0 {
				m.themeSelected--
				return m, nil
			} else if m.mode == modeCacheManager && m.translations != nil && m.cacheSelected > 0 {
				m.cacheSelected--
				return m, nil
			} else if m.showMillerColumns && !m.millerFilterMode {
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
			} else if m.mode == modeReader && m.currentVerses != nil {
				// Navigate to previous verse
				currentIdx := -1
				for i, v := range m.currentVerses {
					if v.Verse == m.highlightedVerseStart {
						currentIdx = i
						break
					}
				}
				if currentIdx > 0 {
					m.highlightedVerseStart = m.currentVerses[currentIdx-1].Verse
					m.highlightedVerseEnd = m.highlightedVerseStart
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
					m.scrollToHighlightedVerse()
				}
				return m, nil
			}
		case "down", "j":
			if m.mode == modeTranslationSelect && m.translations != nil && m.translationSelected < len(m.translations)-1 {
				m.translationSelected++
				return m, nil
			} else if m.mode == modeThemeSelect && m.themeSelected < len(theme.AllThemes())-1 {
				m.themeSelected++
				return m, nil
			} else if m.mode == modeCacheManager && m.translations != nil && m.cacheSelected < len(m.translations)-1 {
				m.cacheSelected++
				return m, nil
			} else if m.showMillerColumns && !m.millerFilterMode && m.books != nil {
				switch m.millerColumn {
				case 0: // Books column
					booksToUse := m.books
					if m.millerFilter != "" && m.millerFilteredBooks != nil {
						booksToUse = m.millerFilteredBooks
					}
					if m.millerBookIdx < len(booksToUse)-1 {
						m.millerBookIdx++
						m.millerChapterIdx = 0
						m.millerVerseIdx = 0
					}
				case 1: // Chapters column
					booksToUse := m.books
					if m.millerFilter != "" && m.millerFilteredBooks != nil {
						booksToUse = m.millerFilteredBooks
					}
					if m.millerBookIdx < len(booksToUse) {
						selectedBook := booksToUse[m.millerBookIdx]
						if m.millerChapterIdx < selectedBook.Chapters-1 {
							m.millerChapterIdx++
							m.millerVerseIdx = 0
						}
					}
				case 2: // Verses column
					versesToUse := m.currentVerses
					if m.millerFilter != "" && m.millerFilteredVerses != nil {
						versesToUse = m.millerFilteredVerses
					}
					if versesToUse != nil && m.millerVerseIdx < len(versesToUse)-1 {
						m.millerVerseIdx++
					}
				}
				return m, nil
			} else if m.showSidebar && m.books != nil && m.sidebarSelected < len(m.books)-1 {
				m.sidebarSelected++
				return m, nil
			} else if m.mode == modeReader && m.currentVerses != nil {
				// Navigate to next verse
				currentIdx := -1
				for i, v := range m.currentVerses {
					if v.Verse == m.highlightedVerseStart {
						currentIdx = i
						break
					}
				}
				if currentIdx >= 0 && currentIdx < len(m.currentVerses)-1 {
					m.highlightedVerseStart = m.currentVerses[currentIdx+1].Verse
					m.highlightedVerseEnd = m.highlightedVerseStart
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
					m.scrollToHighlightedVerse()
				}
				return m, nil
			}
		case "left", "h":
			if m.showMillerColumns && !m.millerFilterMode && m.millerColumn > 0 {
				m.millerColumn--
				return m, nil
			}
		case "right", "l":
			if m.showMillerColumns && !m.millerFilterMode {
				if m.millerColumn < 2 {
					m.millerColumn++
					// When moving to verses column, load the chapter if not already loaded
					if m.millerColumn == 2 {
						booksToUse := m.books
						if m.millerFilter != "" && m.millerFilteredBooks != nil {
							booksToUse = m.millerFilteredBooks
						}
						if m.millerBookIdx < len(booksToUse) {
							selectedBook := booksToUse[m.millerBookIdx]
							selectedChapter := m.millerChapterIdx + 1
							// Only load if different from current
							if selectedBook.BookID != m.currentBook || selectedChapter != m.currentChapter {
								return m, loadChapter(m.client, m.selectedTranslation, selectedBook.BookID, selectedChapter)
							}
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
				m.translationSelected = 0
				// Find current translation in list
				if m.translations != nil {
					for i, trans := range m.translations {
						if trans.ShortName == m.selectedTranslation {
							m.translationSelected = i
							break
						}
					}
				}
				return m, nil
			}
		case "T":
			if m.mode == modeReader && !m.showSidebar {
				m.mode = modeThemeSelect
				m.themeSelected = 0
				// Find current theme in list
				themes := theme.AllThemes()
				for i, thm := range themes {
					if thm.Name == m.currentTheme.Name {
						m.themeSelected = i
						break
					}
				}
				return m, nil
			}
		case "d":
			if m.mode == modeReader && !m.showSidebar {
				m.mode = modeCacheManager
				m.cacheSelected = 0
				if m.cache != nil {
					return m, loadCachedList(m.cache)
				}
				return m, nil
			}
		case "?":
			if m.mode == modeReader && !m.showSidebar {
				m.mode = modeAbout
				return m, nil
			}
		case "n":
			if m.mode == modeReader && m.books != nil {
				for _, book := range m.books {
					if book.BookID == m.currentBook {
						if m.currentChapter < book.Chapters {
							m.currentChapter++
							m.loading = true
							m.highlightedVerseStart = 0
							m.highlightedVerseEnd = 0
							return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
						}
					}
				}
			}
		case "p":
			if m.mode == modeReader && m.currentChapter > 1 {
				m.currentChapter--
				m.loading = true
				m.highlightedVerseStart = 0
				m.highlightedVerseEnd = 0
				return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
			}
		case "y":
			// Yank (copy) highlighted verse(s) or current chapter to clipboard
			if m.mode == modeReader && m.currentVerses != nil {
				var textToCopy strings.Builder

				// If verses are highlighted, only copy those
				if m.highlightedVerseStart > 0 {
					if m.highlightedVerseStart == m.highlightedVerseEnd {
						textToCopy.WriteString(fmt.Sprintf("%s %s %d:%d\n\n", m.selectedTranslation, m.currentBookName, m.currentChapter, m.highlightedVerseStart))
					} else {
						textToCopy.WriteString(fmt.Sprintf("%s %s %d:%d-%d\n\n", m.selectedTranslation, m.currentBookName, m.currentChapter, m.highlightedVerseStart, m.highlightedVerseEnd))
					}

					for _, v := range m.currentVerses {
						if v.Verse >= m.highlightedVerseStart && v.Verse <= m.highlightedVerseEnd {
							text := stripHTMLTags(v.Text)
							textToCopy.WriteString(fmt.Sprintf("%d. %s\n\n", v.Verse, text))
						}
					}
				} else {
					// Copy entire chapter
					textToCopy.WriteString(fmt.Sprintf("%s %s %d\n\n", m.selectedTranslation, m.currentBookName, m.currentChapter))

					for _, v := range m.currentVerses {
						text := stripHTMLTags(v.Text)
						textToCopy.WriteString(fmt.Sprintf("%d. %s\n\n", v.Verse, text))
					}
				}

				clipboard.WriteAll(textToCopy.String())
			}
		case "pgdown":
			// Page down = next chapter
			if m.mode == modeReader && m.books != nil {
				for _, book := range m.books {
					if book.BookID == m.currentBook {
						if m.currentChapter < book.Chapters {
							m.currentChapter++
							m.loading = true
							m.highlightedVerseStart = 0
							m.highlightedVerseEnd = 0
							return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
						}
						break
					}
				}
			}
		case "pgup":
			// Page up = previous chapter
			if m.mode == modeReader && m.currentChapter > 1 {
				m.currentChapter--
				m.loading = true
				m.highlightedVerseStart = 0
				m.highlightedVerseEnd = 0
				return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
			}
		case "enter":
			if m.mode == modeTranslationSelect && m.translations != nil && m.translationSelected < len(m.translations) {
				// Select translation and reload chapter
				m.selectedTranslation = m.translations[m.translationSelected].ShortName
				m.mode = modeReader
				m.loading = true
				return m, tea.Batch(
					loadBooks(m.client, m.selectedTranslation),
					loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter),
				)
			} else if m.mode == modeThemeSelect && m.themeSelected < len(theme.AllThemes()) {
				// Select theme and update all colors
				themes := theme.AllThemes()
				m.currentTheme = themes[m.themeSelected]
				m.mode = modeReader
				return m, nil
			} else if m.mode == modeCacheManager && m.translations != nil && m.cacheSelected < len(m.translations) {
				// Download selected translation
				translation := m.translations[m.cacheSelected].ShortName
				if m.cache != nil && !m.cache.IsCached(translation) {
					m.downloadingTranslation = translation
					return m, downloadTranslation(m.cache, translation)
				}
				return m, nil
			} else if m.showMillerColumns && m.millerFilterMode {
				// Exit filter mode on enter
				m.millerFilterMode = false
				m.millerFilterInput.Blur()
				return m, nil
			} else if m.showMillerColumns && m.books != nil && m.currentVerses != nil {
				// Navigate to the selected verse
				booksToUse := m.books
				if m.millerFilter != "" && m.millerFilteredBooks != nil {
					booksToUse = m.millerFilteredBooks
				}
				if m.millerBookIdx < len(booksToUse) {
					selectedBook := booksToUse[m.millerBookIdx]
					selectedChapter := m.millerChapterIdx + 1
					m.currentBook = selectedBook.BookID
					m.currentBookName = selectedBook.Name
					m.currentChapter = selectedChapter
					m.showMillerColumns = false
					m.loading = true
					m.highlightedVerseStart = 0
					m.highlightedVerseEnd = 0
					// Scroll viewport to the selected verse
					return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
				}
			} else if m.showSidebar && m.books != nil {
				// Select book from sidebar
				if m.sidebarSelected < len(m.books) {
					m.currentBook = m.books[m.sidebarSelected].BookID
					m.currentBookName = m.books[m.sidebarSelected].Name
					m.currentChapter = 1
					m.showSidebar = false
					m.loading = true
					m.highlightedVerseStart = 0
					m.highlightedVerseEnd = 0
					return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
				}
			} else if m.mode == modeSearch {
				input := m.textInput.Value()
				book, chapter, verseStart, verseEnd, err := parseReference(input, m.books)
				if err == nil {
					m.currentBook = book
					m.currentChapter = chapter
					m.highlightedVerseStart = verseStart
					m.highlightedVerseEnd = verseEnd

					// Look up the book name from the book ID
					for _, b := range m.books {
						if b.BookID == book {
							m.currentBookName = b.Name
							break
						}
					}

					m.mode = modeReader
					m.loading = true
					m.textInput.SetValue("")
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
		case "x":
			// Delete cached translation
			if m.mode == modeCacheManager && m.translations != nil && m.cacheSelected < len(m.translations) {
				translation := m.translations[m.cacheSelected].ShortName
				if m.cache != nil && m.cache.IsCached(translation) {
					if err := m.cache.RemoveTranslation(translation); err == nil {
						return m, loadCachedList(m.cache)
					}
				}
				return m, nil
			}
		case "esc":
			if m.mode == modeCacheManager {
				m.mode = modeReader
				return m, nil
			}
			if m.showMillerColumns && m.millerFilterMode {
				// Exit filter mode on esc
				m.millerFilterMode = false
				m.millerFilterInput.Blur()
				return m, nil
			} else if m.showMillerColumns {
				m.showMillerColumns = false
				return m, nil
			}
			if m.showSidebar {
				m.showSidebar = false
				return m, nil
			}
			if m.mode == modeSearch || m.mode == modeTranslationSelect || m.mode == modeThemeSelect || m.mode == modeAbout || m.mode == modeComparison {
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
			m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
		} else if m.currentParallelVerses != nil {
			m.content = m.formatParallelVerses(m.currentParallelVerses, m.comparisonTranslations, m.currentBookName, m.currentChapter, m.width)
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
		// Track if we came from a search (highlighted verse was set)
		cameFromSearch := m.highlightedVerseStart > 1
		// Initialize highlighted verse to first verse or use the range from search
		if m.highlightedVerseStart == 0 {
			if len(msg.verses) > 0 {
				m.highlightedVerseStart = msg.verses[0].Verse
				m.highlightedVerseEnd = m.highlightedVerseStart
			} else {
				m.highlightedVerseStart = 1
				m.highlightedVerseEnd = 1
			}
		}
		m.content = m.formatChapter(msg.verses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
		m.viewport.SetContent(m.content)

		// If we came from a search, scroll to the highlighted verse
		if cameFromSearch {
			m.scrollToHighlightedVerse()
		} else {
			m.viewport.GotoTop()
		}

	case parallelVersesLoadedMsg:
		m.loading = false
		m.currentParallelVerses = msg.verses
		m.currentVerses = nil
		m.content = m.formatParallelVerses(msg.verses, m.comparisonTranslations, m.currentBookName, m.currentChapter, m.width)
		m.viewport.SetContent(m.content)
		m.viewport.GotoTop()

	case cacheListLoadedMsg:
		m.cachedTranslations = msg.translations

	case downloadCompleteMsg:
		m.downloadingTranslation = ""
		if m.cache != nil {
			return m, loadCachedList(m.cache)
		}

	case downloadErrorMsg:
		m.downloadingTranslation = ""
		m.err = msg.err

	case errMsg:
		m.err = msg.err
		m.loading = false
	}

	if m.mode == modeSearch {
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.showMillerColumns && m.millerFilterMode {
		// Update Miller filter input when in filter mode
		m.millerFilterInput, cmd = m.millerFilterInput.Update(msg)
		cmds = append(cmds, cmd)

		// Apply filter when input changes
		newFilter := m.millerFilterInput.Value()
		if newFilter != m.millerFilter {
			m.millerFilter = newFilter
			m.applyMillerFilter()
			m.millerBookIdx = 0
			m.millerVerseIdx = 0
		}
	} else {
		oldYOffset := m.viewport.YOffset
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

		// Update highlighted verse based on viewport position
		if m.currentVerses != nil && oldYOffset != m.viewport.YOffset {
			newHighlightedVerse := m.calculateHighlightedVerse()
			if newHighlightedVerse != m.highlightedVerseStart {
				m.highlightedVerseStart = newHighlightedVerse
				m.highlightedVerseEnd = newHighlightedVerse
				// Reformat content with new highlighted verse
				m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
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
		Foreground(m.currentTheme.Accent).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(m.currentTheme.Border)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.currentTheme.Success)

	helpStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Muted)

	errorStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Error).
		Bold(true)

	// Logo using Unicode character U+100C9
	logo := "\U000100C9"
	logoStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Bold(true)

	var header string
	if m.mode == modeSearch {
		header = headerStyle.Render(logoStyle.Render(logo)+" Search - Enter verse reference") + "\n" + m.textInput.View()
	} else if m.mode == modeTranslationSelect {
		header = headerStyle.Render(logoStyle.Render(logo) + " Select Translation")
	} else if m.mode == modeThemeSelect {
		header = headerStyle.Render(logoStyle.Render(logo) + " Select Theme")
	} else if m.mode == modeComparison {
		header = headerStyle.Render(logoStyle.Render(logo) + " " + fmt.Sprintf("Comparison View - %s %d", m.currentBookName, m.currentChapter))
	} else if m.mode == modeCacheManager {
		header = headerStyle.Render(logoStyle.Render(logo) + " Download Translations")
	} else if m.mode == modeAbout {
		header = headerStyle.Render(logoStyle.Render(logo) + " About")
	} else {
		// Check if current translation is cached
		offlineIndicator := ""
		if m.cache != nil && m.cache.IsCached(m.selectedTranslation) {
			offlineStyle := lipgloss.NewStyle().
				Foreground(m.currentTheme.Success)
			offlineIndicator = " " + offlineStyle.Render("[Offline]")
		}

		// Build title with verse reference if verses are highlighted from search
		var titleText string
		if m.highlightedVerseStart > 0 {
			if m.highlightedVerseStart == m.highlightedVerseEnd {
				// Single verse
				titleText = fmt.Sprintf("Search: %s %s %d:%d", m.selectedTranslation, m.currentBookName, m.currentChapter, m.highlightedVerseStart)
			} else {
				// Verse range
				titleText = fmt.Sprintf("Search: %s %s %d:%d-%d", m.selectedTranslation, m.currentBookName, m.currentChapter, m.highlightedVerseStart, m.highlightedVerseEnd)
			}
		} else {
			titleText = fmt.Sprintf("%s %s %d", m.selectedTranslation, m.currentBookName, m.currentChapter)
		}

		title := logoStyle.Render(logo) + " " + titleStyle.Render(titleText) + offlineIndicator
		header = headerStyle.Render(title)
	}

	// Create status bar with help and version
	statusBarStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Muted).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(m.currentTheme.Border)

	versionStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Bold(true)

	// Format version string with build number
	versionString := fmt.Sprintf("%s (build %s)", version.Version, version.BuildNumber)

	var helpText string
	if m.loading {
		helpText = "Loading..."
	} else if m.mode == modeTranslationSelect {
		helpText = "↑/↓ or j/k: navigate | enter: select | esc: close"
	} else if m.mode == modeThemeSelect {
		helpText = "↑/↓ or j/k: navigate | enter: select | esc: close"
	} else if m.mode == modeCacheManager {
		helpText = "↑/↓ or j/k: navigate | enter: download | x: delete | esc: close"
	} else if m.mode == modeAbout {
		helpText = "esc: close"
	} else if m.mode == modeComparison {
		helpText = "↑/↓ or j/k: scroll | r/esc: return to reader"
	} else if m.showMillerColumns && m.millerFilterMode {
		helpText = "Type to filter | enter/esc: exit filter mode"
	} else if m.showMillerColumns {
		helpText = "↑/↓ or j/k: navigate | ←/→ or h/l: switch column | /: filter | enter: select | v/esc: close"
	} else if m.showSidebar {
		helpText = "↑/↓ or j/k: navigate | enter: select | [/esc: close"
	} else {
		helpText = "[: books | v: verse picker | /: search | c: compare | t: translation | T: theme | d: download | y: yank | n/pgdn: next | p/pgup: prev | ?: about | q: quit"
	}

	// Calculate padding to right-align version
	helpLen := len(helpText)
	versionLen := len(versionString)
	totalLen := helpLen + versionLen + 3 // 3 for spacing
	padding := ""
	if m.width > totalLen {
		padding = strings.Repeat(" ", m.width-totalLen)
	}

	help := statusBarStyle.Render(helpStyle.Render(helpText) + padding + versionStyle.Render(versionString))

	var errorMsg string
	if m.err != nil {
		errorMsg = "\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	mainContent := fmt.Sprintf("%s\n%s\n%s%s", header, m.viewport.View(), help, errorMsg)

	if m.mode == modeTranslationSelect {
		return m.renderTranslationSelect(header, help, errorMsg)
	}

	if m.mode == modeThemeSelect {
		return m.renderThemeSelect(header, help, errorMsg)
	}

	if m.mode == modeCacheManager {
		return m.renderCacheManager(header, help, errorMsg)
	}

	if m.mode == modeAbout {
		return m.renderAbout(header, help, errorMsg)
	}

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
		if textWidth < 20 {
			textWidth = 20
		}
		if textWidth > m.width-2 {
			textWidth = m.width - 2
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

func (m *Model) scrollToHighlightedVerse() {
	if m.currentVerses == nil || len(m.currentVerses) == 0 {
		return
	}

	// Calculate the line position of the highlighted verse
	currentLine := 0
	for _, verse := range m.currentVerses {
		if verse.Verse == m.highlightedVerseStart {
			// Found the verse, scroll to it
			// Keep it near the top of the viewport (with some padding)
			targetOffset := currentLine
			if targetOffset < 0 {
				targetOffset = 0
			}

			// Get the total number of lines in content
			totalLines := strings.Count(m.content, "\n") + 1
			maxOffset := totalLines - m.viewport.Height
			if maxOffset < 0 {
				maxOffset = 0
			}

			if targetOffset > maxOffset {
				targetOffset = maxOffset
			}

			m.viewport.YOffset = targetOffset
			return
		}

		// Calculate lines for this verse
		text := stripHTMLTags(verse.Text)
		verseNumStr := fmt.Sprintf("%d", verse.Verse)
		indent := len(verseNumStr) + 2

		textWidth := m.width - 10
		if textWidth < 20 {
			textWidth = 20
		}
		if textWidth > m.width-2 {
			textWidth = m.width - 2
		}

		wrappedText := wrapTextWithIndent(text, textWidth, indent)
		linesInVerse := strings.Count(wrappedText, "\n") + 1
		verseTotalLines := linesInVerse + 2

		currentLine += verseTotalLines
	}
}

func (m *Model) applyMillerFilter() {
	if m.millerFilter == "" {
		// No filter, clear filtered lists
		m.millerFilteredBooks = nil
		m.millerFilteredVerses = nil
		return
	}

	filterLower := strings.ToLower(m.millerFilter)

	// Filter books based on current column
	if m.millerColumn == 0 && m.books != nil {
		m.millerFilteredBooks = []api.Book{}
		for _, book := range m.books {
			if strings.Contains(strings.ToLower(book.Name), filterLower) {
				m.millerFilteredBooks = append(m.millerFilteredBooks, book)
			}
		}
	}

	// Filter verses based on current column
	if m.millerColumn == 2 && m.currentVerses != nil {
		m.millerFilteredVerses = []api.Verse{}
		for _, verse := range m.currentVerses {
			verseText := stripHTMLTags(verse.Text)
			verseNumStr := fmt.Sprintf("%d", verse.Verse)
			if strings.Contains(strings.ToLower(verseText), filterLower) || strings.Contains(verseNumStr, m.millerFilter) {
				m.millerFilteredVerses = append(m.millerFilteredVerses, verse)
			}
		}
	}
}

// dimContent applies a dimming effect to content by reducing color intensity
// and adding a lighter overlay for a subtle fog/shadow effect
func dimContent(content string) string {
	lines := strings.Split(content, "\n")
	dimmedLines := make([]string, len(lines))

	// Create a dim style with lighter gray to keep text readable
	// Using Faint() for dimming while maintaining visibility
	dimStyle := lipgloss.NewStyle().
		Faint(true).
		Foreground(lipgloss.Color("#888888")) // Lighter gray for better visibility

	for i, line := range lines {
		// Apply faint style to dim the line
		dimmedLines[i] = dimStyle.Render(line)
	}

	return strings.Join(dimmedLines, "\n")
}

func overlayContent(base, overlay string, width, height int) string {
	// Dim the base content to create a focus effect
	base = dimContent(base)

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
		BorderForeground(m.currentTheme.Border).
		Padding(1)

	activeColumnStyle := lipgloss.NewStyle().
		Width(columnWidth).
		Height(m.height - 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		Background(m.currentTheme.Background).
		Padding(1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(m.currentTheme.Background).
		Bold(true).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Background(m.currentTheme.Background).
		Bold(true).
		Padding(0, 1).
		Width(columnWidth-2)

	// Column 1: Books
	var booksContent strings.Builder
	booksContent.WriteString(headerStyle.Render("BOOKS") + "\n")

	// Show filter input if in books column
	if m.millerColumn == 0 && m.millerFilterMode {
		booksContent.WriteString(m.millerFilterInput.View() + "\n")
	} else if m.millerColumn == 0 && m.millerFilter != "" {
		filterStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Warning)
		booksContent.WriteString(filterStyle.Render("Filter: "+m.millerFilter) + "\n\n")
	} else {
		booksContent.WriteString("\n")
	}

	// Use filtered books if filter is active, otherwise use all books
	booksToDisplay := m.books
	if m.millerColumn == 0 && m.millerFilter != "" && m.millerFilteredBooks != nil {
		booksToDisplay = m.millerFilteredBooks
	}

	if booksToDisplay != nil {
		visibleItems := m.height - 8
		if visibleItems < 5 {
			visibleItems = 5
		}

		startIdx := m.millerBookIdx - visibleItems/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := startIdx + visibleItems
		if endIdx > len(booksToDisplay) {
			endIdx = len(booksToDisplay)
			startIdx = endIdx - visibleItems
			if startIdx < 0 {
				startIdx = 0
			}
		}

		if startIdx > 0 {
			booksContent.WriteString(normalStyle.Render(fmt.Sprintf("... (%d)\n", startIdx)))
		}

		for i := startIdx; i < endIdx && i < len(booksToDisplay); i++ {
			book := booksToDisplay[i]
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

		if endIdx < len(booksToDisplay) {
			booksContent.WriteString(normalStyle.Render(fmt.Sprintf("... (%d)\n", len(booksToDisplay)-endIdx)))
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
	versesContent.WriteString(headerStyle.Render("VERSES") + "\n")

	// Show filter input if in verses column
	if m.millerColumn == 2 && m.millerFilterMode {
		versesContent.WriteString(m.millerFilterInput.View() + "\n")
	} else if m.millerColumn == 2 && m.millerFilter != "" {
		filterStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Warning)
		versesContent.WriteString(filterStyle.Render("Filter: "+m.millerFilter) + "\n\n")
	} else {
		versesContent.WriteString("\n")
	}

	// Use filtered verses if filter is active, otherwise use all verses
	versesToDisplay := m.currentVerses
	if m.millerColumn == 2 && m.millerFilter != "" && m.millerFilteredVerses != nil {
		versesToDisplay = m.millerFilteredVerses
	}

	if versesToDisplay != nil {
		visibleItems := m.height - 8
		if visibleItems < 5 {
			visibleItems = 5
		}

		startIdx := m.millerVerseIdx - visibleItems/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := startIdx + visibleItems
		if endIdx > len(versesToDisplay) {
			endIdx = len(versesToDisplay)
			startIdx = endIdx - visibleItems
			if startIdx < 0 {
				startIdx = 0
			}
		}

		if startIdx > 0 {
			versesContent.WriteString(normalStyle.Render(fmt.Sprintf("... (%d)\n", startIdx)))
		}

		for i := startIdx; i < endIdx && i < len(versesToDisplay); i++ {
			verse := versesToDisplay[i]
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

		if endIdx < len(versesToDisplay) {
			versesContent.WriteString(normalStyle.Render(fmt.Sprintf("... (%d)\n", len(versesToDisplay)-endIdx)))
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
	columnsView := lipgloss.JoinHorizontal(lipgloss.Top, booksColumn, chaptersColumn, versesColumn)

	// Add shadow effect to the right of the columns with gradient
	shadow1Style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#333333")).
		Background(lipgloss.Color("#333333"))
	shadow2Style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#222222")).
		Background(lipgloss.Color("#222222"))
	shadow3Style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#111111")).
		Background(lipgloss.Color("#111111"))

	columnsLines := strings.Split(columnsView, "\n")
	shadowLines := make([]string, len(columnsLines))
	for i := range columnsLines {
		shadowLines[i] = shadow1Style.Render("▌") + shadow2Style.Render("▌") + shadow3Style.Render("▌")
	}

	// Combine columns with shadow
	var columnsWithShadow strings.Builder
	for i := 0; i < len(columnsLines); i++ {
		columnsWithShadow.WriteString(columnsLines[i])
		if i < len(shadowLines) {
			columnsWithShadow.WriteString(shadowLines[i])
		}
		if i < len(columnsLines)-1 {
			columnsWithShadow.WriteString("\n")
		}
	}

	// Add status bar at the bottom
	statusBarStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Muted).
		Background(m.currentTheme.Background).
		Width(columnWidth * 3 + 6). // 3 columns + borders
		Align(lipgloss.Center).
		Padding(0, 1)

	statusText := "Press / to filter"
	if m.millerFilterMode {
		statusText = "Filtering... (press enter or esc to exit)"
	} else if m.millerFilter != "" {
		statusText = fmt.Sprintf("Filter active: \"%s\" (press / to edit)", m.millerFilter)
	}

	statusBar := statusBarStyle.Render(statusText)

	// Join columns and status bar vertically
	return lipgloss.JoinVertical(lipgloss.Left, columnsWithShadow.String(), statusBar)
}

func (m Model) renderSidebar() string {
	sidebarStyle := lipgloss.NewStyle().
		Width(30).
		Height(m.height - 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		Background(m.currentTheme.Background).
		Padding(1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(m.currentTheme.Background).
		Bold(true).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Padding(0, 1)

	sectionHeaderStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Background(m.currentTheme.Background).
		Bold(true).
		Padding(0, 1).
		Width(28)

	var sb strings.Builder
	sb.WriteString(sectionHeaderStyle.Render("OLD TESTAMENT") + "\n\n")

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

		sb.WriteString("\n" + sectionHeaderStyle.Render("NEW TESTAMENT") + "\n\n")

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

	sidebar := sidebarStyle.Render(sb.String())

	// Add shadow effect to the right of the sidebar with gradient
	shadow1Style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#333333")).
		Background(lipgloss.Color("#333333"))
	shadow2Style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#222222")).
		Background(lipgloss.Color("#222222"))
	shadow3Style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#111111")).
		Background(lipgloss.Color("#111111"))

	sidebarLines := strings.Split(sidebar, "\n")
	shadowLines := make([]string, len(sidebarLines))
	for i := range sidebarLines {
		shadowLines[i] = shadow1Style.Render("▌") + shadow2Style.Render("▌") + shadow3Style.Render("▌")
	}

	// Combine sidebar with shadow
	var result strings.Builder
	for i := 0; i < len(sidebarLines); i++ {
		result.WriteString(sidebarLines[i])
		if i < len(shadowLines) {
			result.WriteString(shadowLines[i])
		}
		if i < len(sidebarLines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func (m Model) renderTranslationSelect(header, help, errorMsg string) string {
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		Padding(1, 2)

	selectedStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(m.currentTheme.Highlight).
		Bold(true).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Padding(0, 1)

	currentStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Padding(0, 1)

	var content strings.Builder

	if m.translations != nil {
		for i, trans := range m.translations {
			prefix := "  "
			style := normalStyle
			suffix := ""

			// Check if this is the currently selected translation
			isCurrent := trans.ShortName == m.selectedTranslation

			if i == m.translationSelected {
				prefix = "> "
				style = selectedStyle
			} else if isCurrent {
				style = currentStyle
			}

			name := fmt.Sprintf("%-6s - %s", trans.ShortName, trans.FullName)

			if isCurrent && i != m.translationSelected {
				suffix = " [Current]"
			}

			content.WriteString(style.Render(prefix+name+suffix) + "\n")
		}
	} else {
		content.WriteString(normalStyle.Render("  Loading translations..."))
	}

	listContent := containerStyle.Render(content.String())
	return fmt.Sprintf("%s\n%s\n%s%s", header, listContent, help, errorMsg)
}

func (m Model) renderCacheManager(header, help, errorMsg string) string {
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		Padding(1, 2)

	selectedStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(m.currentTheme.Highlight).
		Bold(true).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Padding(0, 1)

	cachedStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Padding(0, 1)

	downloadingStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Warning).
		Padding(0, 1)

	var content strings.Builder

	if m.translations != nil {
		for i, trans := range m.translations {
			prefix := "  "
			style := normalStyle
			suffix := ""

			// Check if cached
			isCached := false
			if m.cache != nil {
				isCached = m.cache.IsCached(trans.ShortName)
			}

			// Check if downloading
			isDownloading := m.downloadingTranslation == trans.ShortName

			if i == m.cacheSelected {
				prefix = "> "
				style = selectedStyle
			}

			name := fmt.Sprintf("%-6s - %s", trans.ShortName, trans.FullName)

			if isDownloading {
				suffix = " [Downloading...]"
				style = downloadingStyle
			} else if isCached {
				suffix = " [✓]"
				if i == m.cacheSelected {
					style = selectedStyle
				} else {
					style = cachedStyle
				}
			}

			content.WriteString(style.Render(prefix+name+suffix) + "\n")
		}
	} else {
		content.WriteString(normalStyle.Render("  Loading translations..."))
	}

	// Show cache size if available
	if m.cache != nil {
		size, err := m.cache.GetCacheSize()
		if err == nil && size > 0 {
			sizeStr := fmt.Sprintf("\n\nCache Size: %.2f MB", float64(size)/(1024*1024))
			content.WriteString("\n" + normalStyle.Render(sizeStr))
		}
	}

	listContent := containerStyle.Render(content.String())
	return fmt.Sprintf("%s\n%s\n%s%s", header, listContent, help, errorMsg)
}

func (m Model) renderThemeSelect(header, help, errorMsg string) string {
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		Padding(1, 2)

	selectedStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(m.currentTheme.Highlight).
		Bold(true).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Padding(0, 1)

	currentStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Padding(0, 1)

	var content strings.Builder

	themes := theme.AllThemes()
	for i, thm := range themes {
		prefix := "  "
		style := normalStyle
		suffix := ""

		// Check if this is the currently active theme
		isCurrent := thm.Name == m.currentTheme.Name

		if i == m.themeSelected {
			prefix = "> "
			style = selectedStyle
		} else if isCurrent {
			style = currentStyle
		}

		if isCurrent && i != m.themeSelected {
			suffix = " [Current]"
		}

		content.WriteString(style.Render(prefix+thm.Name+suffix) + "\n")
	}

	listContent := containerStyle.Render(content.String())
	return fmt.Sprintf("%s\n%s\n%s%s", header, listContent, help, errorMsg)
}

func (m Model) formatChapter(verses []api.Verse, bookName string, chapter int, width int, highlightedVerseStart, highlightedVerseEnd int) string {
	verseStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Warning).
		Bold(true).
		Width(4).
		Align(lipgloss.Right)

	highlightedVerseStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Bold(true).
		Width(4).
		Align(lipgloss.Right)

	textStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary)

	highlightedTextStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Bold(true)

	highlightedContainerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		Padding(0, 1)

	var sb strings.Builder

	// Calculate available width for text (accounting for verse number and spacing)
	// Verse number is right-aligned in 4 chars + 2 spaces = 6 chars total
	textWidth := width - 6
	if textWidth < 20 {
		textWidth = 20 // Minimum width for readability
	}
	// Ensure we don't exceed actual terminal width
	if textWidth > width-2 {
		textWidth = width - 2
	}

	// Track if we're currently in a highlighted range
	inHighlightedRange := false
	var highlightedContent strings.Builder

	for i, v := range verses {
		// Remove HTML tags
		text := stripHTMLTags(v.Text)
		verseNumStr := fmt.Sprintf("%d", v.Verse)

		// Check if this verse is in the highlighted range
		isHighlighted := highlightedVerseStart > 0 && v.Verse >= highlightedVerseStart && v.Verse <= highlightedVerseEnd

		// Check if next verse is also highlighted
		nextIsHighlighted := false
		if i+1 < len(verses) {
			nextVerse := verses[i+1]
			nextIsHighlighted = highlightedVerseStart > 0 && nextVerse.Verse >= highlightedVerseStart && nextVerse.Verse <= highlightedVerseEnd
		}

		if isHighlighted {
			if !inHighlightedRange {
				// Start of highlighted range
				inHighlightedRange = true
				highlightedContent.Reset()
			}

			verseNum := highlightedVerseStyle.Render(verseNumStr)

			// Calculate indent for wrapped lines (verse number width + 2 spaces)
			indent := 6
			// Account for border padding (2 chars on each side)
			wrappedText := wrapTextWithIndent(text, textWidth-4, indent)
			// Apply color with width set to prevent terminal wrapping
			verseText := highlightedTextStyle.Width(textWidth - 4).Render(wrappedText)

			highlightedContent.WriteString(fmt.Sprintf("%s  %s", verseNum, verseText))

			// If next verse is also highlighted, add spacing within the border
			if nextIsHighlighted {
				highlightedContent.WriteString("\n\n")
			} else {
				// End of highlighted range - render the border
				borderedVerse := highlightedContainerStyle.Render(highlightedContent.String())
				sb.WriteString(borderedVerse + "\n\n")
				inHighlightedRange = false
			}
		} else {
			verseNum := verseStyle.Render(verseNumStr)

			// Calculate indent for wrapped lines (verse number width + 2 spaces)
			indent := 6
			wrappedText := wrapTextWithIndent(text, textWidth, indent)
			// Apply color with width set to prevent terminal wrapping
			verseText := textStyle.Width(textWidth).Render(wrappedText)

			sb.WriteString(fmt.Sprintf("%s  %s\n\n", verseNum, verseText))
		}
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

		// Add space before word (except at the very start of a line where currentLength is 0)
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

func (m Model) formatParallelVerses(versesMap map[string][]api.Verse, translations []string, bookName string, chapter int, width int) string {
	translationStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.currentTheme.Accent)

	verseNumStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Warning).
		Background(m.currentTheme.Background).
		Bold(true).
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.currentTheme.Border).
		Padding(0, 1)

	textStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.Border).
		Padding(0, 1)

	separatorStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Border)

	var sb strings.Builder

	// Calculate available width for text (accounting for translation label)
	textWidth := width - 15 // Reserve space for [TRANS] label and padding
	if textWidth < 20 {
		textWidth = 20 // Minimum width for readability
	}
	// Ensure we don't exceed actual terminal width
	if textWidth > width-2 {
		textWidth = width - 2
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
		sb.WriteString(separatorStyle.Render(strings.Repeat("─", separatorWidth)) + "\n")

		for _, trans := range translations {
			verses, ok := versesMap[trans]
			if !ok {
				continue
			}

			for _, v := range verses {
				if v.Verse == i {
					text := stripHTMLTags(v.Text)
					transLabelStr := fmt.Sprintf("[%s]", trans)
					transLabel := translationStyle.Render(transLabelStr)

					// Wrap text without indent since it's in a box
					wrappedText := wrapText(text, textWidth-6) // Account for border and padding
					verseText := textStyle.Render(transLabel + " " + wrappedText)
					sb.WriteString(verseText + "\n\n")
					break
				}
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func stripHTMLTags(s string) string {
	// First replace HTML tags with spaces to preserve word boundaries
	re := regexp.MustCompile(`<[^>]*>`)
	s = re.ReplaceAllString(s, " ")

	// Decode common HTML entities
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&ldquo;", "\u201C") // Left double quote
	s = strings.ReplaceAll(s, "&rdquo;", "\u201D") // Right double quote
	s = strings.ReplaceAll(s, "&lsquo;", "\u2018") // Left single quote
	s = strings.ReplaceAll(s, "&rsquo;", "\u2019") // Right single quote
	s = strings.ReplaceAll(s, "&mdash;", "\u2014") // Em dash
	s = strings.ReplaceAll(s, "&ndash;", "\u2013") // En dash
	s = strings.ReplaceAll(s, "&hellip;", "\u2026") // Ellipsis

	// Decode numeric HTML entities (e.g., &#8220; for left double quote)
	reNumeric := regexp.MustCompile(`&#(\d+);`)
	s = reNumeric.ReplaceAllStringFunc(s, func(match string) string {
		// Extract the numeric code
		numStr := match[2 : len(match)-1]
		if num, err := strconv.Atoi(numStr); err == nil && num < 0x110000 {
			return string(rune(num))
		}
		return match
	})

	// Decode hex HTML entities (e.g., &#x201C; for left double quote)
	reHex := regexp.MustCompile(`&#[xX]([0-9a-fA-F]+);`)
	s = reHex.ReplaceAllStringFunc(s, func(match string) string {
		// Extract the hex code
		hexStr := match[3 : len(match)-1]
		if num, err := strconv.ParseInt(hexStr, 16, 32); err == nil && num < 0x110000 {
			return string(rune(num))
		}
		return match
	})

	// Clean up multiple consecutive spaces
	reSpaces := regexp.MustCompile(`\s+`)
	s = reSpaces.ReplaceAllString(s, " ")

	// Trim leading and trailing spaces
	s = strings.TrimSpace(s)

	return s
}

// fuzzyMatchBook attempts to match a book name or abbreviation to a book ID
func fuzzyMatchBook(query string, books []api.Book) (int, string, bool) {
	query = strings.ToLower(strings.TrimSpace(query))

	// Book name abbreviations mapping
	bookAbbrevs := map[string][]string{
		"genesis": {"gen", "ge", "gn"},
		"exodus": {"exo", "ex", "exod"},
		"leviticus": {"lev", "le", "lv"},
		"numbers": {"num", "nu", "nm", "nb"},
		"deuteronomy": {"deut", "de", "dt"},
		"joshua": {"josh", "jos", "jsh"},
		"judges": {"judg", "jdg", "jg", "jdgs"},
		"ruth": {"rut", "ru", "rth"},
		"1 samuel": {"1sam", "1sa", "1samuel", "1 sam", "1 sa", "1s"},
		"2 samuel": {"2sam", "2sa", "2samuel", "2 sam", "2 sa", "2s"},
		"1 kings": {"1king", "1kgs", "1ki", "1k", "1 kings", "1 kgs"},
		"2 kings": {"2king", "2kgs", "2ki", "2k", "2 kings", "2 kgs"},
		"1 chronicles": {"1chron", "1chr", "1ch", "1 chronicles", "1 chr"},
		"2 chronicles": {"2chron", "2chr", "2ch", "2 chronicles", "2 chr"},
		"ezra": {"ezr", "ez"},
		"nehemiah": {"neh", "ne"},
		"esther": {"est", "es"},
		"job": {"jb"},
		"psalms": {"psalm", "psa", "ps", "pss"},
		"proverbs": {"prov", "pro", "pr", "prv"},
		"ecclesiastes": {"eccl", "ecc", "ec", "qoh"},
		"song of solomon": {"song", "sos", "so", "canticle", "canticles", "song of songs"},
		"isaiah": {"isa", "is"},
		"jeremiah": {"jer", "je", "jr"},
		"lamentations": {"lam", "la"},
		"ezekiel": {"ezek", "eze", "ezk"},
		"daniel": {"dan", "da", "dn"},
		"hosea": {"hos", "ho"},
		"joel": {"joe", "jl"},
		"amos": {"amo", "am"},
		"obadiah": {"obad", "ob"},
		"jonah": {"jon", "jnh"},
		"micah": {"mic", "mi"},
		"nahum": {"nah", "na"},
		"habakkuk": {"hab", "hb"},
		"zephaniah": {"zeph", "zep", "zp"},
		"haggai": {"hag", "hg"},
		"zechariah": {"zech", "zec", "zc"},
		"malachi": {"mal", "ml"},
		"matthew": {"matt", "mat", "mt"},
		"mark": {"mar", "mrk", "mk", "mr"},
		"luke": {"luk", "lk"},
		"john": {"joh", "jhn", "jn"},
		"acts": {"act", "ac"},
		"romans": {"rom", "ro", "rm"},
		"1 corinthians": {"1cor", "1co", "1 corinthians", "1 cor"},
		"2 corinthians": {"2cor", "2co", "2 corinthians", "2 cor"},
		"galatians": {"gal", "ga"},
		"ephesians": {"eph", "ephes"},
		"philippians": {"phil", "php", "pp"},
		"colossians": {"col", "co"},
		"1 thessalonians": {"1thess", "1th", "1 thessalonians", "1 thess"},
		"2 thessalonians": {"2thess", "2th", "2 thessalonians", "2 thess"},
		"1 timothy": {"1tim", "1ti", "1 timothy", "1 tim"},
		"2 timothy": {"2tim", "2ti", "2 timothy", "2 tim"},
		"titus": {"tit", "ti"},
		"philemon": {"philem", "phm", "pm"},
		"hebrews": {"heb", "he"},
		"james": {"jam", "jas", "jm"},
		"1 peter": {"1pet", "1pe", "1pt", "1p", "1 peter", "1 pet"},
		"2 peter": {"2pet", "2pe", "2pt", "2p", "2 peter", "2 pet"},
		"1 john": {"1john", "1jn", "1jo", "1j", "1 john"},
		"2 john": {"2john", "2jn", "2jo", "2j", "2 john"},
		"3 john": {"3john", "3jn", "3jo", "3j", "3 john"},
		"jude": {"jud", "jd"},
		"revelation": {"rev", "re", "rv"},
	}

	// Try exact match first
	for _, book := range books {
		if strings.ToLower(book.Name) == query {
			return book.BookID, book.Name, true
		}
	}

	// Try abbreviation match
	for _, book := range books {
		bookNameLower := strings.ToLower(book.Name)
		if abbrevs, ok := bookAbbrevs[bookNameLower]; ok {
			for _, abbrev := range abbrevs {
				if query == abbrev {
					return book.BookID, book.Name, true
				}
			}
		}
	}

	// Try prefix match
	for _, book := range books {
		if strings.HasPrefix(strings.ToLower(book.Name), query) {
			return book.BookID, book.Name, true
		}
	}

	return 0, "", false
}

func parseReference(ref string, books []api.Book) (book, chapter, verseStart, verseEnd int, err error) {
	// Handle formats like "gal 20:2-4", "Gen 1:1", "1 1:1", "john 3:16-17"
	ref = strings.TrimSpace(ref)

	// Try to parse as "BookName Chapter:Verse-Verse" or "BookID Chapter:Verse-Verse"
	re := regexp.MustCompile(`^([a-zA-Z0-9\s]+)\s+(\d+):(\d+)(?:-(\d+))?$`)
	matches := re.FindStringSubmatch(ref)

	if len(matches) > 0 {
		// Extract book name/number
		bookPart := strings.TrimSpace(matches[1])

		// Try to parse as book ID first
		bookID, parseErr := strconv.Atoi(bookPart)
		if parseErr != nil {
			// Not a number, try fuzzy book name match
			if len(books) > 0 {
				var bookName string
				var found bool
				bookID, bookName, found = fuzzyMatchBook(bookPart, books)
				if !found {
					return 0, 0, 0, 0, fmt.Errorf("book not found: %s", bookPart)
				}
				_ = bookName // Used for matching
			} else {
				return 0, 0, 0, 0, fmt.Errorf("no books loaded")
			}
		}
		book = bookID

		// Parse chapter
		chapter, err = strconv.Atoi(matches[2])
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid chapter")
		}

		// Parse verse start
		verseStart, err = strconv.Atoi(matches[3])
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid verse")
		}

		// Parse verse end (if present)
		if len(matches) > 4 && matches[4] != "" {
			verseEnd, err = strconv.Atoi(matches[4])
			if err != nil {
				verseEnd = verseStart
			}
		} else {
			verseEnd = verseStart
		}

		return book, chapter, verseStart, verseEnd, nil
	}

	// Fallback to old simple numeric parser for backwards compatibility
	parts := strings.Fields(ref)
	if len(parts) == 0 {
		return 0, 0, 0, 0, fmt.Errorf("empty reference")
	}

	// Try to parse first part as book number
	book, err = strconv.Atoi(parts[0])
	if err != nil {
		// Could be book name
		if len(books) > 0 {
			var found bool
			book, _, found = fuzzyMatchBook(parts[0], books)
			if !found {
				return 0, 0, 0, 0, fmt.Errorf("book not found: %s", parts[0])
			}
		} else {
			book = 1 // Default to Genesis
		}
	}

	if len(parts) >= 2 {
		// Parse chapter:verse or chapter:verse-verse
		chapterVerse := parts[1]
		cvParts := strings.Split(chapterVerse, ":")
		chapter, err = strconv.Atoi(cvParts[0])
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid chapter")
		}
		if len(cvParts) > 1 {
			// Check for verse range
			versePart := cvParts[1]
			if strings.Contains(versePart, "-") {
				vRange := strings.Split(versePart, "-")
				verseStart, _ = strconv.Atoi(vRange[0])
				if len(vRange) > 1 {
					verseEnd, _ = strconv.Atoi(vRange[1])
				} else {
					verseEnd = verseStart
				}
			} else {
				verseStart, _ = strconv.Atoi(versePart)
				verseEnd = verseStart
			}
		}
	} else {
		// Only book provided
		chapter = 1
	}

	return book, chapter, verseStart, verseEnd, nil
}

func (m Model) renderAbout(header, help, errorMsg string) string {
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		Padding(1, 2).
		Width(m.width - 4)

	titleStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Bold(true).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary)

	labelStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Secondary).
		Bold(true)

	valueStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary)

	linkStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Underline(true)

	var content strings.Builder

	// Title
	content.WriteString(titleStyle.Render("sword-tui") + "\n")
	content.WriteString(sectionStyle.Render("A terminal-based Bible application") + "\n\n")

	// Version info
	content.WriteString(labelStyle.Render("Version: ") + valueStyle.Render(version.Version) + "\n")
	content.WriteString(labelStyle.Render("Build: ") + valueStyle.Render(version.BuildNumber) + "\n\n")

	// Repository
	content.WriteString(labelStyle.Render("Repository: ") + linkStyle.Render("https://github.com/kmf/sword-tui") + "\n")
	content.WriteString(labelStyle.Render("Report Issues: ") + linkStyle.Render("https://github.com/kmf/sword-tui/issues") + "\n\n")

	// API
	content.WriteString(labelStyle.Render("API: ") + valueStyle.Render("bolls.life") + "\n")
	content.WriteString(sectionStyle.Render("  https://bolls.life/api/") + "\n\n")

	// License
	content.WriteString(labelStyle.Render("License: ") + valueStyle.Render("GPL-2.0-or-later") + "\n\n")

	// Keyboard shortcuts section
	content.WriteString(titleStyle.Render("Keyboard Shortcuts") + "\n\n")

	shortcuts := []struct {
		key  string
		desc string
	}{
		{"[", "Toggle books sidebar"},
		{"v", "Open verse picker"},
		{"/", "Search for verse"},
		{"c", "Compare translations"},
		{"t", "Select translation"},
		{"T", "Select theme"},
		{"d", "Download translations"},
		{"y", "Yank/copy verse"},
		{"n / PgDn", "Next chapter"},
		{"p / PgUp", "Previous chapter"},
		{"?", "Show this about page"},
		{"q", "Quit"},
	}

	for _, s := range shortcuts {
		content.WriteString(labelStyle.Render(s.key) + " - " + sectionStyle.Render(s.desc) + "\n")
	}

	listContent := containerStyle.Render(content.String())
	return fmt.Sprintf("%s\n%s\n%s%s", header, listContent, help, errorMsg)
}
