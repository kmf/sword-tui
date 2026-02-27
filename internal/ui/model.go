package ui

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sword-tui/internal/api"
	"sword-tui/internal/settings"
	"sword-tui/internal/theme"
	"sword-tui/internal/version"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textarea"
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
	modeWordSearch
	modeLanguageSelect
	modeHighlightSettings
	modeVerseHighlight
	modeWordSelect
	modeNoteInput
	modeNoteSidebar
	modeReferenceSelect
	modeReferenceList
)

type Model struct {
	client                 *api.Client
	viewport               viewport.Model
	textInput              textinput.Model
	languages              []string
	selectedLanguage       string
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
	millerColumn         int // 0=books, 1=chapters, 2=verses
	millerBookIdx        int
	millerChapterIdx     int
	millerVerseIdx       int
	showMillerColumns    bool
	millerFilterInput    textinput.Model
	millerFilter         string
	millerFilteredBooks  []api.Book
	millerFilteredVerses []api.Verse
	millerFilterMode     bool // When true, all keys go to filter input
	// Cache management state
	cache                  CacheInterface
	cachedTranslations     []string
	cacheSelected          int
	downloadingTranslation string
	// Selection state
	languageSelected    int
	translationSelected int
	// Theme state
	currentTheme  theme.Theme
	themeSelected int
	// Highlight state
	highlights             []settings.Highlight
	selectedHighlightColor string
	highlightColorSelected int
	verseCursorPos         int // character index within current verse
	selectionStart         int // -1 if no selection active
	hKeyPending            bool
	// Note state
	notes               []settings.Note
	selectedSymbolStyle string
	wordIndex           int // index of selected word in verse
	noteInput           textarea.Model
	showNoteSidebar     bool
	noteSidebarSelected int
	referenceListSelected int
	tempReferences      []settings.Reference
	originalBook        int
	originalChapter     int
	editingNoteVersePK  int
	editingNoteWordIndex int
	nKeyPending         bool
	// Word search state
	wordSearchInput    textinput.Model
	wordSearchQuery    string
	wordSearchResults  []api.Verse
	wordSearchTotal    int
	wordSearchSelected int
	wordSearchLoading  bool
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

type (
	errMsg                  struct{ err error }
	languagesLoadedMsg      struct{ languages []string }
	translationsLoadedMsg   struct{ translations []api.Translation }
	booksLoadedMsg          struct{ books []api.Book }
	chapterLoadedMsg        struct{ verses []api.Verse }
	parallelVersesLoadedMsg struct{ verses map[string][]api.Verse }
	cacheListLoadedMsg      struct{ translations []string }
	downloadingTranslation string
	downloadCompleteMsg     struct{ translation string }
	downloadErrorMsg        struct {
		translation string
		err         error
	}
)

type searchResultsLoadedMsg struct {
	results []api.Verse
	total   int
	query   string
}

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

	wordSearch := textinput.New()
	wordSearch.Placeholder = "Search the Bible..."
	wordSearch.CharLimit = 100
	wordSearch.Width = 50

	noteInput := textarea.New()
	noteInput.Placeholder = "Enter note..."
	noteInput.CharLimit = 500
	noteInput.SetWidth(30)
	noteInput.SetHeight(5)

	// --- Load persisted settings (if any) ---
	cfg, err := settings.Load()

	selectedLanguage := "English"
	selectedTranslation := "NLT"
	currentBook := 1
	currentChapter := 1
	currentTheme := theme.CatppuccinMocha
	var highlights []settings.Highlight
	selectedHighlightColor := "#FFFF00" // Default yellow
	var notes []settings.Note
	selectedSymbolStyle := "numbers"

	if err == nil {
		if cfg.SelectedLanguage != "" {
			selectedLanguage = cfg.SelectedLanguage
		}
		if cfg.SelectedTranslation != "" {
			selectedTranslation = cfg.SelectedTranslation
		}
		if cfg.CurrentBook > 0 {
			currentBook = cfg.CurrentBook
		}
		if cfg.CurrentChapter > 0 {
			currentChapter = cfg.CurrentChapter
		}
		if cfg.Highlights != nil {
			highlights = cfg.Highlights
		}
		if cfg.SelectedHighlightColor != "" {
			selectedHighlightColor = cfg.SelectedHighlightColor
		}
		if cfg.Notes != nil {
			notes = cfg.Notes
		}
		if cfg.SelectedSymbolStyle != "" {
			selectedSymbolStyle = cfg.SelectedSymbolStyle
		}
		if cfg.CurrentTheme != "" {
			// Match by display name against all known themes
			for _, th := range theme.AllThemes() {
				if th.Name == cfg.CurrentTheme {
					currentTheme = th
					break
				}
			}
		}
	}

	return Model{
		client:                 api.NewClient(),
		textInput:              ti,
		millerFilterInput:      millerFilter,
		wordSearchInput:        wordSearch,
		noteInput:              noteInput,
		selectedLanguage:       selectedLanguage,
		selectedTranslation:    selectedTranslation,
		currentBook:            currentBook,
		currentChapter:         currentChapter,
		currentBookName:        "Genesis", // corrected after books load
		mode:                   modeReader,
		comparisonTranslations: []string{"NLT", "KJV", "WEB"},
		currentTheme:           currentTheme,
		themeSelected:          0,
		highlights:             highlights,
		selectedHighlightColor: selectedHighlightColor,
		selectionStart:         -1,
		notes:                  notes,
		selectedSymbolStyle:    selectedSymbolStyle,
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
		loadLanguages(m.client),
		loadTranslations(m.client, m.selectedLanguage),
		loadBooks(m.client, m.selectedTranslation),
		loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter),
	)
}

func loadLanguages(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		groups, err := client.GetLanguageGroups()
		if err != nil {
			return errMsg{err}
		}
		var langs []string
		for _, g := range groups {
			langs = append(langs, g.Language)
		}
		sort.Strings(langs)
		return languagesLoadedMsg{langs}
	}
}

func loadTranslations(client *api.Client, language string) tea.Cmd {
	return func() tea.Msg {
		translations, err := client.GetTranslations(language)
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

func loadSearchResults(client *api.Client, translation, query string) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.SearchVerses(translation, query)
		if err != nil {
			return errMsg{err}
		}
		return searchResultsLoadedMsg{
			results: resp.Results,
			total:   resp.Total,
			query:   query,
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "H": // Shift+h
			if m.mode == modeVerseHighlight {
				if m.selectionStart == -1 {
					m.selectionStart = m.verseCursorPos
				}
				if m.verseCursorPos > 0 {
					m.verseCursorPos--
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
				}
				return m, nil
			}
		case "L": // Shift+l
			if m.mode == modeVerseHighlight {
				if m.selectionStart == -1 {
					m.selectionStart = m.verseCursorPos
				}
				var currentText string
				for _, v := range m.currentVerses {
					if v.Verse == m.highlightedVerseStart {
						currentText = stripHTMLTags(v.Text)
						break
					}
				}
				if m.verseCursorPos < len(currentText)-1 {
					m.verseCursorPos++
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
				}
				return m, nil
			}
		case "ctrl+r":
			if m.mode == modeNoteInput {
				m.originalBook = m.currentBook
				m.originalChapter = m.currentChapter
				m.mode = modeReferenceSelect
				m.showMillerColumns = true
				m.millerColumn = 0
				return m, nil
			}
		case "ctrl+enter", "ctrl+l", "ctrl+j", "ctrl+m", "g":
			if m.mode == modeNoteSidebar {
				notes := m.getChapterNotes()
				if len(notes) > 0 && m.noteSidebarSelected < len(notes) {
					note := notes[m.noteSidebarSelected]
					if len(note.References) == 1 {
						// Single reference, go directly
						ref := note.References[0]
						m.originalBook = m.currentBook
						m.originalChapter = m.currentChapter
						m.currentBook = ref.BookID
						m.currentBookName = ref.BookName
						m.currentChapter = ref.Chapter
						m.highlightedVerseStart = ref.Verse
						m.highlightedVerseEnd = ref.Verse
						m.mode = modeReader
						m.loading = true
						return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
					} else if len(note.References) > 1 {
						// Multiple references, show selection list
						m.originalBook = m.currentBook
						m.originalChapter = m.currentChapter
						m.mode = modeReferenceList
						m.referenceListSelected = 0
						return m, nil
					}
				}
			}
		case "a":
			if m.mode == modeNoteSidebar {
				m.mode = modeWordSelect
				m.wordIndex = 0
				return m, nil
			}
		case "d":
			if m.mode == modeNoteSidebar {
				notes := m.getChapterNotes()
				if len(notes) > 0 && m.noteSidebarSelected < len(notes) {
					noteToDelete := notes[m.noteSidebarSelected]
					// Remove from m.notes
					for i, n := range m.notes {
						if n.VersePK == noteToDelete.VersePK && n.WordIndex == noteToDelete.WordIndex && n.Translation == noteToDelete.Translation {
							m.notes = append(m.notes[:i], m.notes[i+1:]...)
							break
						}
					}
					if m.noteSidebarSelected > 0 && m.noteSidebarSelected >= len(m.getChapterNotes()) {
						m.noteSidebarSelected--
					}
				}
				return m, nil
			}
			if m.mode == modeReader && !m.showSidebar {
				m.mode = modeCacheManager
				m.cacheSelected = 0
				if m.cache != nil {
					return m, loadCachedList(m.cache)
				}
				return m, nil
			}
		case "ctrl+c", "q":
			// Save settings synchronously before quitting to avoid race condition
			cfg := settings.Settings{
				SelectedLanguage:       m.selectedLanguage,
				SelectedTranslation:    m.selectedTranslation,
				CurrentBook:            m.currentBook,
				CurrentChapter:         m.currentChapter,
				CurrentTheme:           m.currentTheme.Name,
				Highlights:             m.highlights,
				SelectedHighlightColor: m.selectedHighlightColor,
				Notes:                  m.notes,
				SelectedSymbolStyle:    m.selectedSymbolStyle,
			}
			_ = settings.Save(cfg)
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
		case "J": // Shift+j (extend highlight forward)
			if m.mode == modeVerseHighlight {
				if m.selectionStart == -1 {
					m.selectionStart = m.verseCursorPos
				}
				var currentText string
				for _, v := range m.currentVerses {
					if v.Verse == m.highlightedVerseStart {
						currentText = stripHTMLTags(v.Text)
						break
					}
				}
				// Jump 5 chars forward
				m.verseCursorPos += 5
				if m.verseCursorPos >= len(currentText) {
					m.verseCursorPos = len(currentText) - 1
				}
				m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
				m.viewport.SetContent(m.content)
				return m, nil
			}
		case "K": // Shift+k (extend highlight backward)
			if m.mode == modeVerseHighlight {
				if m.selectionStart == -1 {
					m.selectionStart = m.verseCursorPos
				}
				m.verseCursorPos -= 5
				if m.verseCursorPos < 0 {
					m.verseCursorPos = 0
				}
				m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
				m.viewport.SetContent(m.content)
				return m, nil
			}
		case "up", "k":
			if m.mode == modeNoteSidebar {
				if m.noteSidebarSelected > 0 {
					m.noteSidebarSelected--
				}
				return m, nil
			}
			if m.mode == modeReferenceList {
				if m.referenceListSelected > 0 {
					m.referenceListSelected--
				}
				return m, nil
			}
			if m.mode == modeVerseHighlight {
				m.verseCursorPos -= 10 // Page-like jump in verse
				if m.verseCursorPos < 0 { m.verseCursorPos = 0 }
				m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
				m.viewport.SetContent(m.content)
				return m, nil
			}
			if m.mode == modeWordSelect {
				if m.wordIndex > 0 {
					m.wordIndex--
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
				}
				return m, nil
			}
			m.hKeyPending = false
			m.nKeyPending = false
			if m.mode == modeHighlightSettings && m.highlightColorSelected > 0 {
				m.highlightColorSelected--
				return m, nil
			} else if m.mode == modeLanguageSelect && m.languages != nil && m.languageSelected > 0 {
				m.languageSelected--
				return m, nil
			} else if m.mode == modeWordSearch && m.wordSearchResults != nil && m.wordSearchSelected > 0 {
				m.wordSearchSelected--
				return m, nil
			} else if m.mode == modeTranslationSelect && m.translations != nil && m.translationSelected > 0 {
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
			if m.mode == modeNoteSidebar {
				notes := m.getChapterNotes()
				if m.noteSidebarSelected < len(notes)-1 {
					m.noteSidebarSelected++
				}
				return m, nil
			}
			if m.mode == modeReferenceList {
				notes := m.getChapterNotes()
				if m.noteSidebarSelected < len(notes) {
					note := notes[m.noteSidebarSelected]
					if m.referenceListSelected < len(note.References)-1 {
						m.referenceListSelected++
					}
				}
				return m, nil
			}
			if m.mode == modeVerseHighlight {
				var currentText string
				for _, v := range m.currentVerses {
					if v.Verse == m.highlightedVerseStart {
						currentText = stripHTMLTags(v.Text)
						break
					}
				}
				m.verseCursorPos += 10
				if m.verseCursorPos >= len(currentText) { m.verseCursorPos = len(currentText) - 1 }
				m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
				m.viewport.SetContent(m.content)
				return m, nil
			}
			if m.mode == modeWordSelect {
				var currentText string
				for _, v := range m.currentVerses {
					if v.Verse == m.highlightedVerseStart {
						currentText = stripHTMLTags(v.Text)
						break
					}
				}
				words := strings.Split(currentText, " ")
				if m.wordIndex < len(words)-1 {
					m.wordIndex++
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
				}
				return m, nil
			}
			m.hKeyPending = false
			m.nKeyPending = false
			if m.mode == modeHighlightSettings && m.highlightColorSelected < 7 { // 8 basic colors
				m.highlightColorSelected++
				return m, nil
			} else if m.mode == modeLanguageSelect && m.languages != nil && m.languageSelected < len(m.languages)-1 {
				m.languageSelected++
				return m, nil
			} else if m.mode == modeWordSearch && m.wordSearchResults != nil && m.wordSearchSelected < len(m.wordSearchResults)-1 {
				m.wordSearchSelected++
				return m, nil
			} else if m.mode == modeTranslationSelect && m.translations != nil && m.translationSelected < len(m.translations)-1 {
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
			if m.mode == modeVerseHighlight {
				if m.verseCursorPos > 0 {
					m.verseCursorPos--
					// Refresh content to show cursor move
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
				}
				return m, nil
			}
			if m.mode == modeWordSelect {
				if m.wordIndex > 0 {
					m.wordIndex--
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
				}
				return m, nil
			}
			if m.mode == modeReader && !m.showSidebar && !m.showMillerColumns {
				if m.hKeyPending {
					m.hKeyPending = false
					m.mode = modeVerseHighlight
					m.verseCursorPos = 0
					m.selectionStart = -1
				} else {
					m.hKeyPending = true
				}
				return m, nil
			}
			m.hKeyPending = false
			m.nKeyPending = false
			if m.showMillerColumns && !m.millerFilterMode && m.millerColumn > 0 {
				m.millerColumn--
				return m, nil
			}
		case "right", "l":
			if m.mode == modeVerseHighlight {
				var currentText string
				for _, v := range m.currentVerses {
					if v.Verse == m.highlightedVerseStart {
						currentText = stripHTMLTags(v.Text)
						break
					}
				}
				if m.verseCursorPos < len(currentText)-1 {
					m.verseCursorPos++
					// Refresh content to show cursor move
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
				}
				return m, nil
			}
			if m.mode == modeWordSelect {
				var currentText string
				for _, v := range m.currentVerses {
					if v.Verse == m.highlightedVerseStart {
						currentText = stripHTMLTags(v.Text)
						break
					}
				}
				words := strings.Split(currentText, " ")
				if m.wordIndex < len(words)-1 {
					m.wordIndex++
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
				}
				return m, nil
			}
			m.hKeyPending = false
			m.nKeyPending = false
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
			} else if m.mode == modeReader && !m.showSidebar {
				m.mode = modeLanguageSelect
				m.languageSelected = 0
				if m.languages != nil {
					for i, lang := range m.languages {
						if lang == m.selectedLanguage {
							m.languageSelected = i
							break
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
			if m.mode == modeVerseHighlight && m.selectionStart != -1 {
				var versePK int
				for _, v := range m.currentVerses {
					if v.Verse == m.highlightedVerseStart {
						versePK = v.PK
						break
					}
				}

				start, end := m.selectionStart, m.verseCursorPos
				if start > end {
					start, end = end, start
				}
				end++ // make inclusive for comparison

				// Remove any highlights that overlap or match this range
				newHighlights := []settings.Highlight{}
				for _, h := range m.highlights {
					if h.VersePK == versePK && h.Start == start && h.End == end {
						// Match found - skip this one to remove it
						continue
					}
					newHighlights = append(newHighlights, h)
				}
				m.highlights = newHighlights
				m.selectionStart = -1
				// Re-render
				m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
				m.viewport.SetContent(m.content)
				return m, nil
			}
			// Don't intercept 'r' when typing in search inputs or notes
			if m.mode == modeSearch {
				// Let it pass through to verse reference input
			} else if m.mode == modeNoteInput {
				// Let it pass through to note input
			} else if m.mode == modeWordSearch && m.wordSearchResults == nil && !m.wordSearchLoading {
				// Let it pass through to word search input
			} else if m.mode != modeReader {
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
		case "?":
			if m.mode == modeReader && !m.showSidebar {
				m.mode = modeAbout
				return m, nil
			}
		case "n":
			if m.mode == modeReader && !m.showSidebar && !m.showMillerColumns {
				m.mode = modeNoteSidebar
				m.noteSidebarSelected = 0
				return m, nil
			}
			if m.mode == modeNoteSidebar {
				// While in sidebar, n could still mean go to word select if no notes exist
				// but let's stick to 'a' for add in sidebar mode as per help text
				return m, nil
			}
			// Fallback to next chapter for 'n' if not in reader mode or overlays open
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
		case "s":
			if m.hKeyPending {
				m.hKeyPending = false
				m.mode = modeHighlightSettings
				m.highlightColorSelected = 0
				return m, nil
			}
			if m.nKeyPending {
				m.nKeyPending = false
				// For now, toggle between numbers and symbols
				if m.selectedSymbolStyle == "numbers" {
					m.selectedSymbolStyle = "symbols"
				} else {
					m.selectedSymbolStyle = "numbers"
				}
				return m, nil
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
			if m.mode == modeNoteSidebar {
				notes := m.getChapterNotes()
				if len(notes) > 0 && m.noteSidebarSelected < len(notes) {
					note := notes[m.noteSidebarSelected]
					m.mode = modeNoteInput
					m.wordIndex = note.WordIndex
					m.editingNoteVersePK = note.VersePK
					m.editingNoteWordIndex = note.WordIndex
					// We need to set highlightedVerseStart to match the note's verse
					for _, v := range m.currentVerses {
						if v.PK == note.VersePK {
							m.highlightedVerseStart = v.Verse
							m.highlightedVerseEnd = v.Verse
							break
						}
					}
					m.noteInput.Focus()
					m.noteInput.SetValue(note.Text)
					m.noteInput.SetWidth(m.width - 10)
					m.tempReferences = append([]settings.Reference{}, note.References...)
				}
				return m, nil
			}
			if m.mode == modeReferenceList {
				notes := m.getChapterNotes()
				if len(notes) > 0 && m.noteSidebarSelected < len(notes) {
					note := notes[m.noteSidebarSelected]
					if m.referenceListSelected < len(note.References) {
						ref := note.References[m.referenceListSelected]
						m.currentBook = ref.BookID
						m.currentBookName = ref.BookName
						m.currentChapter = ref.Chapter
						m.highlightedVerseStart = ref.Verse
						m.highlightedVerseEnd = ref.Verse
						m.mode = modeReader
						m.loading = true
						return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
					}
				}
				return m, nil
			}
			if m.hKeyPending {
				m.hKeyPending = false
				m.mode = modeVerseHighlight
				m.verseCursorPos = 0
				m.selectionStart = -1
				return m, nil
			}
			if m.nKeyPending {
				m.nKeyPending = false
				m.mode = modeWordSelect
				m.wordIndex = 0
				return m, nil
			}
			if m.mode == modeWordSelect {
				// Transition to note input for selected word
				m.mode = modeNoteInput
				m.noteInput.Focus()
				m.noteInput.SetWidth(m.width - 10)
				// Load existing note if any
				m.noteInput.SetValue("")
				m.tempReferences = nil
				var versePK int
				for _, v := range m.currentVerses {
					if v.Verse == m.highlightedVerseStart {
						versePK = v.PK
						break
					}
				}
				m.editingNoteVersePK = versePK
				m.editingNoteWordIndex = m.wordIndex
				for _, note := range m.notes {
					if note.VersePK == versePK && note.WordIndex == m.wordIndex && note.Translation == m.selectedTranslation {
						m.noteInput.SetValue(note.Text)
						m.tempReferences = append([]settings.Reference{}, note.References...)
						break
					}
				}
				return m, nil
			}
			if m.mode == modeNoteInput {
				// Save note using the preserved context
				versePK := m.editingNoteVersePK
				wordIdx := m.editingNoteWordIndex
				
				noteText := strings.TrimSpace(m.noteInput.Value())
				
				// Precise one-to-one synchronization of m.tempReferences with noteText
				// This handles multiple identical references correctly.
				var finalRefs []settings.Reference
				remainingText := noteText
				for _, ref := range m.tempReferences {
					refStr := fmt.Sprintf("(%s %d:%d)", ref.BookName, ref.Chapter, ref.Verse)
					if strings.Contains(remainingText, refStr) {
						finalRefs = append(finalRefs, ref)
						// Remove only the FIRST occurrence of this ref from remainingText 
						// so we don't match it again for duplicate structured refs
						remainingText = strings.Replace(remainingText, refStr, "", 1)
					}
				}
				m.tempReferences = finalRefs

				// Clean up any existing notes for this same word to prevent duplicates/ghost text
				var filteredNotes []settings.Note
				for _, note := range m.notes {
					if note.VersePK == versePK && note.WordIndex == wordIdx && note.Translation == m.selectedTranslation {
						continue
					}
					filteredNotes = append(filteredNotes, note)
				}
				m.notes = filteredNotes
				
				// Add the updated/new note if not empty
				if noteText != "" || len(m.tempReferences) > 0 {
					m.notes = append(m.notes, settings.Note{
						Translation: m.selectedTranslation,
						VersePK:     versePK,
						WordIndex:   wordIdx,
						Symbol:      "*",
						Text:        noteText,
						References:  m.tempReferences,
					})
				}
				
				m.mode = modeNoteSidebar
				m.noteSidebarSelected = 0
				return m, nil
			}
			if m.mode == modeVerseHighlight {
				// Save highlight if selection exists
				if m.selectionStart != -1 {
					var versePK int
					for _, v := range m.currentVerses {
						if v.Verse == m.highlightedVerseStart {
							versePK = v.PK
							break
						}
					}

					start := m.selectionStart
					end := m.verseCursorPos
					if start > end {
						start, end = end, start
					}
					// end is exclusive in slices usually, but here we'll include it
					m.highlights = append(m.highlights, settings.Highlight{
						VersePK: versePK,
						Start:   start,
						End:     end + 1,
						Color:   m.selectedHighlightColor,
					})
					m.selectionStart = -1
					// Re-render
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
				}
				return m, nil
			}
			if m.mode == modeHighlightSettings {
				colors := []string{"#FFFF00", "#FF00FF", "#00FFFF", "#00FF00", "#FF0000", "#0000FF", "#FFFFFF", "#FFA500"}
				m.selectedHighlightColor = colors[m.highlightColorSelected]
				m.mode = modeReader
				return m, nil
			}
			if m.mode == modeLanguageSelect && m.languages != nil && m.languageSelected < len(m.languages) {
				m.selectedLanguage = m.languages[m.languageSelected]
				m.mode = modeTranslationSelect
				m.translationSelected = 0
				return m, loadTranslations(m.client, m.selectedLanguage)
			} else if m.mode == modeTranslationSelect && m.translations != nil && m.translationSelected < len(m.translations) {
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
				// Navigate or select reference
				booksToUse := m.books
				if m.millerFilter != "" && m.millerFilteredBooks != nil {
					booksToUse = m.millerFilteredBooks
				}
				if m.millerBookIdx < len(booksToUse) {
					selectedBook := booksToUse[m.millerBookIdx]
					selectedChapter := m.millerChapterIdx + 1
					
					if m.mode == modeReferenceSelect {
						// Add reference instead of navigating
						versesToUse := m.currentVerses
						if m.millerFilter != "" && m.millerFilteredVerses != nil {
							versesToUse = m.millerFilteredVerses
						}
						
						if m.millerVerseIdx < len(versesToUse) {
							verse := versesToUse[m.millerVerseIdx].Verse
							ref := settings.Reference{
								BookID:      selectedBook.BookID,
								BookName:    selectedBook.Name,
								Chapter:     selectedChapter,
								Verse:       verse,
								Translation: m.selectedTranslation,
							}
							m.tempReferences = append(m.tempReferences, ref)
							
							// Also append a text representation to the note
							refText := fmt.Sprintf("(%s %d:%d)", ref.BookName, ref.Chapter, ref.Verse)
							m.noteInput.SetValue(m.noteInput.Value() + refText)
							m.noteInput.CursorEnd()
							
							m.mode = modeNoteInput
							m.noteInput.SetWidth(m.width - 10)
							m.showMillerColumns = false
							// Restore original position
							m.currentBook = m.originalBook
							m.currentChapter = m.originalChapter
							m.loading = true
							return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
						}
					}

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
			} else if m.mode == modeWordSearch {
				if m.wordSearchResults == nil && !m.wordSearchLoading {
					// Submit search query
					query := m.wordSearchInput.Value()
					if query != "" {
						m.wordSearchLoading = true
						m.wordSearchInput.Blur()
						return m, loadSearchResults(m.client, m.selectedTranslation, query)
					}
				} else if m.wordSearchResults != nil && len(m.wordSearchResults) > 0 {
					// Navigate to selected result
					result := m.wordSearchResults[m.wordSearchSelected]
					m.currentBook = result.Book
					m.currentChapter = result.Chapter
					m.highlightedVerseStart = result.Verse
					m.highlightedVerseEnd = result.Verse

					// Look up the book name
					for _, b := range m.books {
						if b.BookID == result.Book {
							m.currentBookName = b.Name
							break
						}
					}

					m.mode = modeReader
					m.loading = true
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
			m.hKeyPending = false
			m.nKeyPending = false
			if m.mode == modeReferenceSelect {
				m.mode = modeNoteInput
				m.noteInput.SetWidth(m.width - 10)
				m.showMillerColumns = false
				// Restore original position
				m.currentBook = m.originalBook
				m.currentChapter = m.originalChapter
				m.loading = true
				return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
			}
			if m.mode == modeReferenceList {
				m.mode = modeNoteSidebar
				return m, nil
			}
			if m.mode == modeNoteSidebar {
				m.mode = modeReader
				return m, nil
			}
			if m.mode == modeWordSelect || m.mode == modeNoteInput {
				notes := m.getChapterNotes()
				if len(notes) > 0 {
					m.mode = modeNoteSidebar
				} else {
					m.mode = modeReader
				}
				m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.width, m.highlightedVerseStart, m.highlightedVerseEnd)
				m.viewport.SetContent(m.content)
				return m, nil
			}
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
			if m.mode == modeSearch || m.mode == modeTranslationSelect || m.mode == modeThemeSelect || m.mode == modeAbout || m.mode == modeComparison || m.mode == modeWordSearch || m.mode == modeLanguageSelect {
				m.mode = modeReader
				m.wordSearchResults = nil
				m.wordSearchInput.SetValue("")
				return m, nil
			}
		}

	case tea.MouseMsg:
		if m.showMillerColumns {
			switch msg.Type {
			case tea.MouseWheelUp:
				switch m.millerColumn {
				case 0:
					if m.millerBookIdx > 0 {
						m.millerBookIdx--
						m.millerChapterIdx = 0
						m.millerVerseIdx = 0
					}
				case 1:
					if m.millerChapterIdx > 0 {
						m.millerChapterIdx--
						m.millerVerseIdx = 0
					}
				case 2:
					if m.millerVerseIdx > 0 {
						m.millerVerseIdx--
					}
				}
				return m, nil
			case tea.MouseWheelDown:
				switch m.millerColumn {
				case 0:
					booksToUse := m.books
					if m.millerFilter != "" && m.millerFilteredBooks != nil {
						booksToUse = m.millerFilteredBooks
					}
					if booksToUse != nil && m.millerBookIdx < len(booksToUse)-1 {
						m.millerBookIdx++
						m.millerChapterIdx = 0
						m.millerVerseIdx = 0
					}
				case 1:
					booksToUse := m.books
					if m.millerFilter != "" && m.millerFilteredBooks != nil {
						booksToUse = m.millerFilteredBooks
					}
					if booksToUse != nil && m.millerBookIdx < len(booksToUse) {
						selectedBook := booksToUse[m.millerBookIdx]
						if m.millerChapterIdx < selectedBook.Chapters-1 {
							m.millerChapterIdx++
							m.millerVerseIdx = 0
						}
					}
				case 2:
					versesToUse := m.currentVerses
					if m.millerFilter != "" && m.millerFilteredVerses != nil {
						versesToUse = m.millerFilteredVerses
					}
					if versesToUse != nil && m.millerVerseIdx < len(versesToUse)-1 {
						m.millerVerseIdx++
					}
				}
				return m, nil
			case tea.MouseLeft:
				// Each column is 30 chars wide
				columnWidth := 30
				if msg.X >= columnWidth*3+6 {
					m.showMillerColumns = false
					return m, nil
				}

				clickedColumn := msg.X / (columnWidth + 2)
				if clickedColumn > 2 {
					clickedColumn = 2
				}

				// If switching columns
				if clickedColumn != m.millerColumn {
					m.millerColumn = clickedColumn
					// When moving to verses column, load the chapter if not already loaded
					if m.millerColumn == 2 {
						booksToUse := m.books
						if m.millerFilter != "" && m.millerFilteredBooks != nil {
							booksToUse = m.millerFilteredBooks
						}
						if m.millerBookIdx < len(booksToUse) {
							selectedBook := booksToUse[m.millerBookIdx]
							selectedChapter := m.millerChapterIdx + 1
							if selectedBook.BookID != m.currentBook || selectedChapter != m.currentChapter {
								return m, loadChapter(m.client, m.selectedTranslation, selectedBook.BookID, selectedChapter)
							}
						}
					}
					return m, nil
				}

				// Map click to item within column
				clickY := msg.Y - 2 // Account for border/padding
				if clickY < 0 {
					return m, nil
				}

				// Calculate startIdx and heights for mapping (same logic as renderColumn)
				var items []string
				var selectedIdx int
				availableLines := m.height - 8
				if availableLines < 1 {
					availableLines = 1
				}

				switch m.millerColumn {
				case 0: // Books
					booksToUse := m.books
					if m.millerFilter != "" && m.millerFilteredBooks != nil {
						booksToUse = m.millerFilteredBooks
					}
					for _, b := range booksToUse {
						items = append(items, b.Name)
					}
					selectedIdx = m.millerBookIdx
				case 1: // Chapters
					booksToUse := m.books
					if m.millerFilter != "" && m.millerFilteredBooks != nil {
						booksToUse = m.millerFilteredBooks
					}
					if booksToUse != nil && m.millerBookIdx < len(booksToUse) {
						selectedBook := booksToUse[m.millerBookIdx]
						for i := 0; i < selectedBook.Chapters; i++ {
							items = append(items, fmt.Sprintf("Chapter %d", i+1))
						}
					}
					selectedIdx = m.millerChapterIdx
				case 2: // Verses
					versesToUse := m.currentVerses
					if m.millerFilter != "" && m.millerFilteredVerses != nil {
						versesToUse = m.millerFilteredVerses
					}
					for _, v := range versesToUse {
						items = append(items, fmt.Sprintf("%d. %s", v.Verse, stripHTMLTags(v.Text)))
					}
					selectedIdx = m.millerVerseIdx
				}

				if items == nil || len(items) == 0 {
					return m, nil
				}

				// Same height/window logic as renderColumn
				contentWidth := columnWidth - 4
				heights := make([]int, len(items))
				for i, item := range items {
					heights[i] = lipgloss.Height(lipgloss.NewStyle().Width(contentWidth).Padding(0, 1).Render("  " + item))
				}

				startIdx := 0
				if selectedIdx != -1 {
					targetLines := availableLines
					if selectedIdx > 0 {
						targetLines--
					}
					if selectedIdx < len(items)-1 {
						targetLines--
					}
					if targetLines < 1 {
						targetLines = 1
					}
					currentLines := heights[selectedIdx]
					startIdx = selectedIdx
					endIdx := selectedIdx + 1
					for {
						expanded := false
						if startIdx > 0 && currentLines+heights[startIdx-1] <= targetLines {
							startIdx--
							currentLines += heights[startIdx]
							expanded = true
						}
						if endIdx < len(items) && currentLines+heights[endIdx] <= targetLines {
							currentLines += heights[endIdx]
							endIdx++
							expanded = true
						}
						if !expanded || currentLines >= targetLines {
							break
						}
					}
				}

				// Map clickY to item
				currentY := 2 // Header + blank line
				if startIdx > 0 {
					if clickY == currentY { // Clicked "... above"
						switch m.millerColumn {
						case 0:
							if m.millerBookIdx > 0 {
								m.millerBookIdx--
							}
						case 1:
							if m.millerChapterIdx > 0 {
								m.millerChapterIdx--
							}
						case 2:
							if m.millerVerseIdx > 0 {
								m.millerVerseIdx--
							}
						}
						return m, nil
					}
					currentY++
				}

				for i := startIdx; i < len(items); i++ {
					if clickY >= currentY && clickY < currentY+heights[i] {
						// Found the item
						switch m.millerColumn {
						case 0:
							m.millerBookIdx = i
							m.millerChapterIdx = 0
							m.millerVerseIdx = 0
						case 1:
							m.millerChapterIdx = i
							m.millerVerseIdx = 0
						case 2:
							m.millerVerseIdx = i
							// On double click or already selected, we might want to navigate
							// For now, just selecting is enough as it updates Column 1/2/3
						}
						return m, nil
					}
					currentY += heights[i]
					if currentY > clickY {
						break
					}
				}

				// Handle "more below" click
				if clickY == currentY && currentY < availableLines+3 {
					switch m.millerColumn {
					case 0:
						booksToUse := m.books
						if m.millerFilter != "" && m.millerFilteredBooks != nil {
							booksToUse = m.millerFilteredBooks
						}
						if booksToUse != nil && m.millerBookIdx < len(booksToUse)-1 {
							m.millerBookIdx++
						}
					case 1:
						booksToUse := m.books
						if m.millerFilter != "" && m.millerFilteredBooks != nil {
							booksToUse = m.millerFilteredBooks
						}
						if booksToUse != nil && m.millerBookIdx < len(booksToUse) {
							selectedBook := booksToUse[m.millerBookIdx]
							if m.millerChapterIdx < selectedBook.Chapters-1 {
								m.millerChapterIdx++
							}
						}
					case 2:
						versesToUse := m.currentVerses
						if m.millerFilter != "" && m.millerFilteredVerses != nil {
							versesToUse = m.millerFilteredVerses
						}
						if versesToUse != nil && m.millerVerseIdx < len(versesToUse)-1 {
							m.millerVerseIdx++
						}
					}
				}
			}
			return m, nil
		}

		if m.showSidebar {
			// Handle all mouse events when sidebar is open to prevent fallthrough
			switch msg.Type {
			case tea.MouseWheelUp:
				if m.sidebarSelected > 0 {
					m.sidebarSelected--
				}
				return m, nil
			case tea.MouseWheelDown:
				if m.books != nil && m.sidebarSelected < len(m.books)-1 {
					m.sidebarSelected++
				}
				return m, nil
			case tea.MouseLeft:
				sidebarWidth := 30
				// sidebarWidth + border/shadow (approx 4 chars)
				if msg.X < sidebarWidth+4 {
					// We need the heights of entries to map the click
					// Re-calculating logic from renderSidebar for click mapping
					contentWidth := sidebarWidth - 4
					if contentWidth < 10 { contentWidth = 10 }
					
					entries := m.getSidebarEntries()
					if entries == nil { return m, nil }

					// Calculate heights (same logic as renderSidebar)
					selectedIdx := -1
					for i, entry := range entries {
						var h int
						if entry.isHeader {
							if entry.headerText == "" {
								h = 1
							} else {
								h = lipgloss.Height(lipgloss.NewStyle().Padding(0, 1).Width(contentWidth).Bold(true).Render(entry.headerText))
							}
						} else {
							if entry.bookIndex == m.sidebarSelected { selectedIdx = i }
							h = lipgloss.Height(lipgloss.NewStyle().Padding(0, 1).Width(contentWidth).Render("  " + entry.book.Name))
						}
						entries[i].height = h
					}

					// Find startIdx/endIdx (same logic as renderSidebar)
					availableLines := m.height - 6
					if availableLines < 1 { availableLines = 1 }
					
					startIdx := 0
					if selectedIdx != -1 {
						reservedForMore := 0
						if selectedIdx > 0 { reservedForMore++ }
						if selectedIdx < len(entries)-1 { reservedForMore++ }
						targetLines := availableLines - reservedForMore
						if targetLines < 1 { targetLines = 1 }
						
						currentLines := entries[selectedIdx].height
						startIdx = selectedIdx
						endIdx := selectedIdx + 1
						for {
							expanded := false
							if startIdx > 0 && currentLines + entries[startIdx-1].height <= targetLines {
								startIdx--
								currentLines += entries[startIdx].height
								expanded = true
							}
							if endIdx < len(entries) && currentLines + entries[endIdx].height <= targetLines {
								currentLines += entries[endIdx].height
								endIdx++
								expanded = true
							}
							if !expanded || currentLines >= targetLines { break }
						}
					}

					// Now map clickY to entry
					clickY := msg.Y - 2 // Account for border/padding
					if clickY < 0 { return m, nil }

					currentY := 0
					if startIdx > 0 {
						if clickY == 0 {
							// Clicked "more above"
							if m.sidebarSelected > 0 { m.sidebarSelected-- }
							return m, nil
						}
						currentY = 1 // Start after "more above" indicator
					}

					for i := startIdx; i < len(entries); i++ {
						if clickY >= currentY && clickY < currentY+entries[i].height {
							entry := entries[i]
							if !entry.isHeader {
								m.sidebarSelected = entry.bookIndex
								m.currentBook = entry.book.BookID
								m.currentBookName = entry.book.Name
								m.currentChapter = 1
								m.showSidebar = false
								m.loading = true
								m.highlightedVerseStart = 0
								m.highlightedVerseEnd = 0
								return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
							}
							return m, nil
						}
						currentY += entries[i].height
						// Stop if we've passed the click point
						if currentY > clickY { break }
					}
					
					// If we're at the very bottom and there's a "more below" indicator
					if clickY == currentY && currentY < availableLines {
						if m.books != nil && m.sidebarSelected < len(m.books)-1 {
							m.sidebarSelected++
						}
						return m, nil
					}
				} else {
					// Clicked outside sidebar - close it
					m.showSidebar = false
					return m, nil
				}
			}
			// Intercept all other mouse events
			return m, nil
		} else {
			// Reader mode mouse interactions
			if msg.Type == tea.MouseLeft && msg.Shift {
				// Potential highlight action
				// Mapping screen click to verse character is complex in TUI, 
				// for now we'll support the keyboard-based highlighting primarily
				// but we can enter highlight mode on Shift+Click
				if m.mode == modeReader {
					m.mode = modeVerseHighlight
					m.verseCursorPos = 0 // Approximate
					m.selectionStart = 0
				}
				return m, nil
			}

			// Pass mouse events to viewport for scrolling
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update note input width
		m.noteInput.SetWidth(m.width - 10)

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

	case languagesLoadedMsg:
		m.languages = msg.languages

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
		// Track if we came from a search or reference (highlighted verse was set)
		cameFromSearch := m.highlightedVerseStart > 0
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

	case searchResultsLoadedMsg:
		m.wordSearchLoading = false
		m.wordSearchResults = msg.results
		m.wordSearchTotal = msg.total
		m.wordSearchQuery = msg.query
		m.wordSearchSelected = 0
		// Sort results by book order
		sort.Slice(m.wordSearchResults, func(i, j int) bool {
			if m.wordSearchResults[i].Book != m.wordSearchResults[j].Book {
				return m.wordSearchResults[i].Book < m.wordSearchResults[j].Book
			}
			if m.wordSearchResults[i].Chapter != m.wordSearchResults[j].Chapter {
				return m.wordSearchResults[i].Chapter < m.wordSearchResults[j].Chapter
			}
			return m.wordSearchResults[i].Verse < m.wordSearchResults[j].Verse
		})

	case errMsg:
		m.err = msg.err
		m.loading = false
		m.wordSearchLoading = false
	}

	if m.mode == modeSearch {
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.mode == modeNoteInput {
		m.noteInput, cmd = m.noteInput.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.mode == modeWordSearch && m.wordSearchResults == nil && !m.wordSearchLoading {
		// Update word search input when typing query
		m.wordSearchInput, cmd = m.wordSearchInput.Update(msg)
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
		BorderForeground(m.currentTheme.Border).
		Width(m.width)

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
	} else if m.mode == modeLanguageSelect {
		header = headerStyle.Render(logoStyle.Render(logo) + " Select Language")
	} else if m.mode == modeHighlightSettings {
		header = headerStyle.Render(logoStyle.Render(logo) + " Select Highlight Color")
	} else if m.mode == modeVerseHighlight {
		header = headerStyle.Render(logoStyle.Render(logo) + " Verse Highlighting")
	} else if m.mode == modeWordSelect {
		header = headerStyle.Render(logoStyle.Render(logo) + " Select Word for Note")
	} else if m.mode == modeNoteInput {
		header = headerStyle.Render(logoStyle.Render(logo) + " Enter Note")
	} else if m.mode == modeTranslationSelect {
		header = headerStyle.Render(logoStyle.Render(logo) + " Select Translation - " + m.selectedLanguage)
	} else if m.mode == modeThemeSelect {
		header = headerStyle.Render(logoStyle.Render(logo) + " Select Theme")
	} else if m.mode == modeComparison {
		header = headerStyle.Render(logoStyle.Render(logo) + " " + fmt.Sprintf("Comparison View - %s %d", m.currentBookName, m.currentChapter))
	} else if m.mode == modeCacheManager {
		header = headerStyle.Render(logoStyle.Render(logo) + " Download Translations")
	} else if m.mode == modeAbout {
		header = headerStyle.Render(logoStyle.Render(logo) + " About")
	} else if m.mode == modeWordSearch {
		header = headerStyle.Render(logoStyle.Render(logo) + " Search Bible")
	} else if m.mode == modeReferenceSelect {
		header = headerStyle.Render(logoStyle.Render(logo) + " Select Reference for Note")
	} else if m.mode == modeReferenceList {
		header = headerStyle.Render(logoStyle.Render(logo) + " Select Reference to Navigate")
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
	} else if m.mode == modeHighlightSettings {
		helpText = "↑/↓ or j/k: navigate | enter: select | esc: close"
	} else if m.mode == modeVerseHighlight {
		helpText = "h/j/k/l: move cursor | Shift+move: highlight | enter: save | esc: cancel"
	} else if m.mode == modeWordSelect {
		helpText = "h/l: select word | enter: add/edit note | esc: cancel"
	} else if m.mode == modeNoteInput {
		helpText = "type note... | ctrl+r: add ref | enter: save | esc: cancel"
	} else if m.mode == modeLanguageSelect {
		helpText = "↑/↓ or j/k: navigate | enter: select | esc: close"
	} else if m.mode == modeTranslationSelect {
		helpText = "↑/↓ or j/k: navigate | enter: select | esc: close"
	} else if m.mode == modeThemeSelect {
		helpText = "↑/↓ or j/k: navigate | enter: select | esc: close"
	} else if m.mode == modeCacheManager {
		helpText = "↑/↓ or j/k: navigate | enter: download | x: delete | esc: close"
	} else if m.mode == modeAbout {
		helpText = "esc: close"
	} else if m.mode == modeWordSearch {
		if m.wordSearchResults != nil {
			helpText = "↑/↓ or j/k: navigate | enter: go to verse | esc: close"
		} else {
			helpText = "enter: search | esc: close"
		}
	} else if m.mode == modeComparison {
		helpText = "↑/↓ or j/k: scroll | r/esc: return to reader"
	} else if m.showMillerColumns && m.millerFilterMode {
		helpText = "Type to filter | enter/esc: exit filter mode"
	} else if m.showMillerColumns {
		if m.mode == modeReferenceSelect {
			helpText = "↑/↓ or j/k: navigate | ←/→ or h/l: switch column | enter: select reference | esc: cancel"
		} else {
			helpText = "↑/↓ or j/k: navigate | ←/→ or h/l: switch column | /: filter | enter: select | v/esc: close"
		}
	} else if m.showSidebar {
		helpText = "↑/↓ or j/k: navigate | enter: select | [/esc: close"
	} else if m.mode == modeNoteSidebar || m.mode == modeReferenceList {
		helpText = "↑/↓ or j/k: navigate | enter: edit | g: go to ref | a: add | d: delete | esc: back"
	} else {
		helpText = "[: books | v: verse picker | /: search | s: word search | c: compare | l: language | t: translation | T: theme | d: download | h: highlight | n: notes | y: yank | ?: about | q: quit"
	}

	// Calculate padding to right-align version
	helpLen := len(helpText)
	versionLen := len(versionString)
	totalLen := helpLen + versionLen + 3 // 3 for spacing
	padding := ""
	if m.width > totalLen {
		padding = strings.Repeat(" ", m.width-totalLen)
	}

	help := statusBarStyle.Width(m.width).Render(helpStyle.Render(helpText) + padding + versionStyle.Render(versionString))

	var errorMsg string
	if m.err != nil {
		errorMsg = "\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	// Calculate dynamic viewport height to prevent overflow
	headerHeight := lipgloss.Height(header)
	helpHeight := lipgloss.Height(help)
	errorHeight := 0
	if errorMsg != "" {
		errorHeight = lipgloss.Height(errorMsg)
	}

	// Calculate available height for viewport (2 for newlines in fmt.Sprintf)
	m.viewport.Height = m.height - headerHeight - helpHeight - errorHeight - 2
	if m.viewport.Height < 1 {
		m.viewport.Height = 1
	}

	// Side content for notes
	var sideContent string
	sidebarWidth := 35
	if m.mode == modeNoteSidebar || m.mode == modeWordSelect || m.mode == modeNoteInput || m.mode == modeReferenceList {
		sideContent = m.renderNoteSidebar()
	}

	var viewportView string
	if sideContent != "" {
		m.viewport.Width = m.width - sidebarWidth - 2
		
		sidebar := lipgloss.NewStyle().
			Width(sidebarWidth).
			Height(m.viewport.Height).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(m.currentTheme.Border).
			Padding(0, 1).
			Render(sideContent)
		
		viewportView = lipgloss.JoinHorizontal(lipgloss.Top, m.viewport.View(), sidebar)
	} else {
		m.viewport.Width = m.width
		viewportView = m.viewport.View()
	}

	mainContent := fmt.Sprintf("%s\n%s\n%s%s", header, viewportView, help, errorMsg)

	if m.mode == modeLanguageSelect {
		return m.renderLanguageSelect(header, help, errorMsg)
	}

	if m.mode == modeHighlightSettings {
		return m.renderHighlightSettings(header, help, errorMsg)
	}

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

	if m.mode == modeNoteInput {
		return m.renderNoteInput(header, help, errorMsg)
	}

	if m.mode == modeWordSearch {
		return m.renderWordSearch(header, help, errorMsg)
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

	// Calculate which verse is at the top of the viewport
	yOffset := m.viewport.YOffset

	// Calculate text width the same way formatChapter does
	textWidth := m.width - 6
	if textWidth < 20 {
		textWidth = 20
	}
	if textWidth > m.width-2 {
		textWidth = m.width - 2
	}

	// Count lines to find which verse we're at
	currentLine := 0
	indent := 6 // verse number width + 2 spaces

	for i, verse := range m.currentVerses {
		text := stripHTMLTags(verse.Text)

		// Check if this verse is highlighted (which would add a border)
		isHighlighted := m.highlightedVerseStart > 0 && verse.Verse >= m.highlightedVerseStart && verse.Verse <= m.highlightedVerseEnd

		// Check if next verse is also highlighted
		nextIsHighlighted := false
		if i+1 < len(m.currentVerses) {
			nextVerse := m.currentVerses[i+1]
			nextIsHighlighted = m.highlightedVerseStart > 0 && nextVerse.Verse >= m.highlightedVerseStart && nextVerse.Verse <= m.highlightedVerseEnd
		}

		var verseTotalLines int
		if isHighlighted {
			// Highlighted verses use narrower width due to border padding
			wrappedText := wrapTextWithIndent(text, textWidth-4, indent)
			linesInVerse := strings.Count(wrappedText, "\n") + 1

			if !nextIsHighlighted {
				// End of highlighted range - border adds 2 lines (top + bottom)
				verseTotalLines = linesInVerse + 2 + 2
			} else {
				// Middle of highlighted range
				verseTotalLines = linesInVerse + 1
			}
		} else {
			wrappedText := wrapTextWithIndent(text, textWidth, indent)
			linesInVerse := strings.Count(wrappedText, "\n") + 1
			verseTotalLines = linesInVerse + 1
		}

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
	// This must match exactly how formatChapter renders verses
	currentLine := 0

	// Calculate text width the same way formatChapter does
	textWidth := m.width - 6
	if textWidth < 20 {
		textWidth = 20
	}
	if textWidth > m.width-2 {
		textWidth = m.width - 2
	}

	for i, verse := range m.currentVerses {
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

		// Calculate lines for this verse - must match formatChapter logic
		text := stripHTMLTags(verse.Text)
		indent := 6 // verse number width + 2 spaces

		// Check if this verse is highlighted (which would add a border)
		isHighlighted := m.highlightedVerseStart > 0 && verse.Verse >= m.highlightedVerseStart && verse.Verse <= m.highlightedVerseEnd

		// Check if next verse is also highlighted
		nextIsHighlighted := false
		if i+1 < len(m.currentVerses) {
			nextVerse := m.currentVerses[i+1]
			nextIsHighlighted = m.highlightedVerseStart > 0 && nextVerse.Verse >= m.highlightedVerseStart && nextVerse.Verse <= m.highlightedVerseEnd
		}

		if isHighlighted {
			// Highlighted verses use narrower width due to border padding
			wrappedText := wrapTextWithIndent(text, textWidth-4, indent)
			linesInVerse := strings.Count(wrappedText, "\n") + 1

			if !nextIsHighlighted {
				// End of highlighted range - border adds 2 lines (top + bottom)
				// Plus verse lines, plus blank line after
				verseTotalLines := linesInVerse + 2 + 2 // border top/bottom + blank line + content
				currentLine += verseTotalLines
			} else {
				// Middle of highlighted range - just the verse text + blank line within border
				verseTotalLines := linesInVerse + 1 // +1 for blank line between verses in border
				currentLine += verseTotalLines
			}
		} else {
			wrappedText := wrapTextWithIndent(text, textWidth, indent)
			linesInVerse := strings.Count(wrappedText, "\n") + 1
			verseTotalLines := linesInVerse + 1 // +1 for blank line after verse
			currentLine += verseTotalLines
		}
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
	contentWidth := columnWidth - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

	columnStyle := lipgloss.NewStyle().
		Width(columnWidth).
		Height(m.height - 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.currentTheme.Border).
		Background(m.currentTheme.Background).
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
		Padding(0, 1).
		Width(contentWidth)

	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(m.currentTheme.Background).
		Padding(0, 1).
		Width(contentWidth)

	headerStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Background(m.currentTheme.Background).
		Bold(true).
		Padding(0, 1).
		Width(contentWidth)

	availableLines := m.height - 8
	if availableLines < 1 {
		availableLines = 1
	}

	// Helper to render a column with wrapping and virtual scrolling
	renderColumn := func(title string, items []string, selectedIdx int, showFilter bool, filter string) string {
		var sb strings.Builder
		sb.WriteString(headerStyle.Render(title) + "\n")

		if showFilter && m.millerFilterMode {
			sb.WriteString(m.millerFilterInput.View() + "\n")
		} else if showFilter && filter != "" {
			filterStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Warning).Background(m.currentTheme.Background)
			sb.WriteString(filterStyle.Render("Filter: "+filter) + "\n\n")
		} else {
			sb.WriteString("\n")
		}

		if items == nil || len(items) == 0 {
			if title == "VERSES" {
				sb.WriteString(normalStyle.Render("  Loading..."))
			}
			return sb.String()
		}

		// Calculate heights of all items
		heights := make([]int, len(items))
		rendered := make([]string, len(items))
		for i, item := range items {
			prefix := "  "
			style := normalStyle
			if i == selectedIdx {
				prefix = "> "
				style = selectedStyle
			}
			text := style.Render(prefix + item)
			rendered[i] = text + "\n"
			heights[i] = lipgloss.Height(text)
		}

		// Calculate window
		startIdx := 0
		endIdx := len(items)
		if selectedIdx != -1 {
			targetLines := availableLines
			if selectedIdx > 0 {
				targetLines--
			}
			if selectedIdx < len(items)-1 {
				targetLines--
			}
			if targetLines < 1 {
				targetLines = 1
			}

			currentLines := heights[selectedIdx]
			startIdx = selectedIdx
			endIdx = selectedIdx + 1

			for {
				expanded := false
				if startIdx > 0 && currentLines+heights[startIdx-1] <= targetLines {
					startIdx--
					currentLines += heights[startIdx]
					expanded = true
				}
				if endIdx < len(items) && currentLines+heights[endIdx] <= targetLines {
					currentLines += heights[endIdx]
					endIdx++
					expanded = true
				}
				if !expanded || currentLines >= targetLines {
					break
				}
			}
		}

		if startIdx > 0 {
			sb.WriteString(normalStyle.Render(fmt.Sprintf("... (%d above)", startIdx)) + "\n")
		}
		for i := startIdx; i < endIdx; i++ {
			sb.WriteString(rendered[i])
		}
		if endIdx < len(items) {
			sb.WriteString(normalStyle.Render(fmt.Sprintf("... (%d below)", len(items)-endIdx)))
		}

		return sb.String()
	}

	// Column 1: Books
	booksToDisplay := m.books
	if m.millerFilter != "" && m.millerFilteredBooks != nil {
		booksToDisplay = m.millerFilteredBooks
	}
	var bookNames []string
	for _, b := range booksToDisplay {
		bookNames = append(bookNames, b.Name)
	}
	booksContent := renderColumn("BOOKS", bookNames, m.millerBookIdx, m.millerColumn == 0, m.millerFilter)
	var booksColumn string
	if m.millerColumn == 0 {
		booksColumn = activeColumnStyle.Render(booksContent)
	} else {
		booksColumn = columnStyle.Render(booksContent)
	}

	// Column 2: Chapters
	var chapterNames []string
	if m.books != nil && m.millerBookIdx < len(m.books) {
		selectedBook := m.books[m.millerBookIdx]
		if m.millerFilter != "" && m.millerFilteredBooks != nil {
			selectedBook = m.millerFilteredBooks[m.millerBookIdx]
		}
		for i := 0; i < selectedBook.Chapters; i++ {
			chapterNames = append(chapterNames, fmt.Sprintf("Chapter %d", i+1))
		}
	}
	chaptersContent := renderColumn("CHAPTERS", chapterNames, m.millerChapterIdx, m.millerColumn == 1, "")
	var chaptersColumn string
	if m.millerColumn == 1 {
		chaptersColumn = activeColumnStyle.Render(chaptersContent)
	} else {
		chaptersColumn = columnStyle.Render(chaptersContent)
	}

	// Column 3: Verses
	versesToDisplay := m.currentVerses
	if m.millerFilter != "" && m.millerFilteredVerses != nil {
		versesToDisplay = m.millerFilteredVerses
	}
	var verseLabels []string
	for _, v := range versesToDisplay {
		verseLabels = append(verseLabels, fmt.Sprintf("%d. %s", v.Verse, stripHTMLTags(v.Text)))
	}
	versesContent := renderColumn("VERSES", verseLabels, m.millerVerseIdx, m.millerColumn == 2, m.millerFilter)
	var versesColumn string
	if m.millerColumn == 2 {
		versesColumn = activeColumnStyle.Render(versesContent)
	} else {
		versesColumn = columnStyle.Render(versesContent)
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
	var result strings.Builder
	for i := 0; i < len(columnsLines); i++ {
		result.WriteString(columnsLines[i])
		if i < len(shadowLines) {
			result.WriteString(shadowLines[i])
		}
		if i < len(columnsLines)-1 {
			result.WriteString("\n")
		}
	}

	// Add status bar at the bottom
	statusBarStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Muted).
		Background(m.currentTheme.Background).
		Width(columnWidth*3+6). // 3 columns + borders
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
	return lipgloss.JoinVertical(lipgloss.Left, result.String(), statusBar)
}

// sidebarEntry represents a single item in the sidebar (header, book, or space)
type sidebarEntry struct {
	isHeader   bool
	headerText string
	bookIndex  int
	book       api.Book
	height     int // calculated during render
}

func (m Model) getSidebarEntries() []sidebarEntry {
	if m.books == nil {
		return nil
	}

	var entries []sidebarEntry

	// Old Testament header
	entries = append(entries, sidebarEntry{isHeader: true, headerText: "OLD TESTAMENT"})

	// Old Testament books
	for i, book := range m.books {
		if book.BookID > 39 {
			break
		}
		entries = append(entries, sidebarEntry{isHeader: false, bookIndex: i, book: book})
	}

	// New Testament header (with spacing)
	entries = append(entries, sidebarEntry{isHeader: true, headerText: ""}) // blank line
	entries = append(entries, sidebarEntry{isHeader: true, headerText: "NEW TESTAMENT"})

	// New Testament books
	for i, book := range m.books {
		if book.BookID < 40 {
			continue
		}
		entries = append(entries, sidebarEntry{isHeader: false, bookIndex: i, book: book})
	}

	return entries
}

func (m Model) renderSidebar() string {
	sidebarWidth := 30
	contentWidth := sidebarWidth - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

	sidebarStyle := lipgloss.NewStyle().
		Width(sidebarWidth).
		Height(m.height - 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		Background(m.currentTheme.Background).
		Padding(1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(m.currentTheme.Background).
		Bold(true).
		Padding(0, 1).
		Width(contentWidth)

	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(m.currentTheme.Background).
		Padding(0, 1).
		Width(contentWidth)

	sectionHeaderStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Background(m.currentTheme.Background).
		Bold(true).
		Padding(0, 1).
		Width(contentWidth)

	moreStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Muted).
		Background(m.currentTheme.Background).
		Italic(true).
		Padding(0, 1).
		Width(contentWidth)

	// Internal height: total - borders - padding
	availableLines := m.height - 6
	if availableLines < 1 {
		availableLines = 1
	}

	entries := m.getSidebarEntries()
	if entries == nil {
		return ""
	}

	// First pass: calculate heights for ALL entries to correctly handle scrolling
	type renderedEntry struct {
		entry sidebarEntry
		text  string
	}
	rendered := make([]renderedEntry, len(entries))
	selectedIdx := -1

	for i, entry := range entries {
		var text string
		var height int
		if entry.isHeader {
			if entry.headerText == "" {
				text = "\n"
				height = 1
			} else {
				renderedText := sectionHeaderStyle.Render(entry.headerText)
				text = renderedText + "\n"
				height = lipgloss.Height(renderedText)
			}
		} else {
			prefix := "  "
			style := normalStyle
			if entry.bookIndex == m.sidebarSelected {
				prefix = "> "
				style = selectedStyle
				selectedIdx = i
			}
			renderedText := style.Render(prefix + entry.book.Name)
			text = renderedText + "\n"
			height = lipgloss.Height(renderedText)
		}
		entries[i].height = height
		rendered[i] = renderedEntry{entry: entries[i], text: text}
	}

	// Calculate which entries to show
	startIdx := 0
	endIdx := len(entries)

	if selectedIdx != -1 {
		// Attempt to find a window of entries that fits availableLines
		reservedForMore := 0
		if selectedIdx > 0 {
			reservedForMore++
		}
		if selectedIdx < len(entries)-1 {
			reservedForMore++
		}

		targetLines := availableLines - reservedForMore
		if targetLines < 1 {
			targetLines = 1
		}

		currentLines := entries[selectedIdx].height
		startIdx = selectedIdx
		endIdx = selectedIdx + 1

		for {
			expanded := false
			if startIdx > 0 && currentLines+entries[startIdx-1].height <= targetLines {
				startIdx--
				currentLines += entries[startIdx].height
				expanded = true
			}
			if endIdx < len(entries) && currentLines+entries[endIdx].height <= targetLines {
				currentLines += entries[endIdx].height
				endIdx++
				expanded = true
			}
			if !expanded || currentLines >= targetLines {
				break
			}
		}
	}

	var sb strings.Builder
	if startIdx > 0 {
		sb.WriteString(moreStyle.Render(fmt.Sprintf("... (%d more above)", startIdx)) + "\n")
	}
	for i := startIdx; i < endIdx; i++ {
		sb.WriteString(rendered[i].text)
	}
	if endIdx < len(entries) {
		sb.WriteString(moreStyle.Render(fmt.Sprintf("... (%d more below)", len(entries)-endIdx)))
	}

	sidebar := sidebarStyle.Render(strings.TrimSuffix(sb.String(), "\n"))

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

func (m Model) renderHighlightSettings(header, help, errorMsg string) string {
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
		Background(m.currentTheme.Background).
		Padding(0, 1)

	colors := []string{"#FFFF00", "#FF00FF", "#00FFFF", "#00FF00", "#FF0000", "#0000FF", "#FFFFFF", "#FFA500"}
	colorNames := []string{"Yellow", "Magenta", "Cyan", "Green", "Red", "Blue", "White", "Orange"}

	var content strings.Builder
	for i, name := range colorNames {
		prefix := "  "
		style := normalStyle
		if i == m.highlightColorSelected {
			prefix = "> "
			style = selectedStyle
		}

		// Show a small color preview box
		preview := lipgloss.NewStyle().Background(lipgloss.Color(colors[i])).Render("  ")
		content.WriteString(style.Render(prefix+name) + " " + preview + "\n")
	}

	listContent := containerStyle.Width(m.width - 4).Render(strings.TrimSuffix(content.String(), "\n"))
	return fmt.Sprintf("%s\n%s\n%s%s", header, listContent, help, errorMsg)
}

func (m Model) renderNoteInput(header, help, errorMsg string) string {
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Bold(true)

	var currentWord string
	for _, v := range m.currentVerses {
		if v.Verse == m.highlightedVerseStart {
			words := strings.Split(stripHTMLTags(v.Text), " ")
			if m.wordIndex >= 0 && m.wordIndex < len(words) {
				currentWord = words[m.wordIndex]
			}
			break
		}
	}

	var content strings.Builder
	content.WriteString(titleStyle.Render("Add Note for word:") + " " + currentWord + "\n\n")
	
	content.WriteString(m.noteInput.View())
	content.WriteString("\n\nPress Enter to save, Esc to cancel")

	listContent := containerStyle.Width(m.width - 4).Render(content.String())
	return fmt.Sprintf("%s\n%s\n%s%s", header, listContent, help, errorMsg)
}

func (m Model) getChapterNotes() []settings.Note {
	var chapterNotes []settings.Note
	versePKs := make(map[int]int) // PK -> Verse Number
	for _, v := range m.currentVerses {
		versePKs[v.PK] = v.Verse
	}

	for _, note := range m.notes {
		if note.Translation == m.selectedTranslation {
			if _, ok := versePKs[note.VersePK]; ok {
				chapterNotes = append(chapterNotes, note)
			}
		}
	}

	// Sort by verse number then word index
	sort.Slice(chapterNotes, func(i, j int) bool {
		v1 := versePKs[chapterNotes[i].VersePK]
		v2 := versePKs[chapterNotes[j].VersePK]
		if v1 != v2 {
			return v1 < v2
		}
		return chapterNotes[i].WordIndex < chapterNotes[j].WordIndex
	})

	return chapterNotes
}

func (m Model) renderReferenceList(header, help, errorMsg string) string {
	chapterNotes := m.getChapterNotes()
	if len(chapterNotes) == 0 || m.noteSidebarSelected >= len(chapterNotes) {
		return m.renderNoteSidebar() // Fallback
	}

	note := chapterNotes[m.noteSidebarSelected]
	
	sidebarWidth := 35
	contentWidth := sidebarWidth - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Bold(true).
		MarginBottom(1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(m.currentTheme.Highlight).
		Bold(true).
		Padding(0, 1).
		Width(contentWidth)

	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Padding(0, 1).
		Width(contentWidth)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("SELECT REFERENCE") + "\n\n")

	for i, ref := range note.References {
		prefix := "  "
		style := normalStyle
		if i == m.referenceListSelected {
			prefix = "> "
			style = selectedStyle
		}

		refStr := fmt.Sprintf("%s %d:%d", ref.BookName, ref.Chapter, ref.Verse)
		sb.WriteString(style.Render(prefix+refStr) + "\n")
	}

	sb.WriteString("\n\n" + lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Render("j/k: nav | enter: go\nesc: back"))
	return sb.String()
}

func (m Model) highlightReferencesInText(text string, baseStyle lipgloss.Style) string {
	// Robust regex for (Book Chapter:Verse) or (Chapter:Verse)
	// Matches patterns like (John 3:16), (1 John 1:9), (Song of Solomon 1:1), (3:16), (Gen. 1:1)
	re := regexp.MustCompile(`\((?:(?:[1-3]\s+)?[a-zA-Z\.]+(?:\s+[a-zA-Z\.]+)*\s+)?\d+:\d+\)`)
	
	refStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#32CD32")).Italic(true)
	
	matches := re.FindAllStringIndex(text, -1)
	if matches == nil {
		return baseStyle.Render(text)
	}
	var sb strings.Builder
	lastIdx := 0
	for _, match := range matches {
		if match[0] > lastIdx {
			sb.WriteString(baseStyle.Render(text[lastIdx:match[0]]))
		}
		sb.WriteString(refStyle.Render(text[match[0]:match[1]]))
		lastIdx = match[1]
	}
	if lastIdx < len(text) {
		sb.WriteString(baseStyle.Render(text[lastIdx:]))
	}
	return sb.String()
}

func (m Model) renderNoteSidebar() string {
	chapterNotes := m.getChapterNotes()
	if len(chapterNotes) == 0 && m.mode != modeWordSelect && m.mode != modeNoteInput {
		return ""
	}
	sidebarWidth := 35
	contentWidth := sidebarWidth - 4
	if contentWidth < 10 { contentWidth = 10 }
	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Bold(true).MarginBottom(1)
	selectedStyle := lipgloss.NewStyle().UnsetForeground().Background(m.currentTheme.Highlight).Bold(true).Padding(0, 1).Width(contentWidth)
	normalStyle := lipgloss.NewStyle().UnsetForeground().Padding(0, 1).Width(contentWidth)
	verseRefStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Bold(true)
	noteTextStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	refCountStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Warning).Italic(true)
	var sb strings.Builder
	if m.mode == modeReferenceList {
		return m.renderReferenceList("", "", "")
	}
	sb.WriteString(titleStyle.Render("CHAPTER NOTES") + "\n\n")
	if len(chapterNotes) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Padding(0, 1).Render("No notes in this chapter."))
		if m.mode == modeNoteSidebar {
			sb.WriteString("\n\n" + lipgloss.NewStyle().Foreground(m.currentTheme.Success).Padding(0, 1).Render("Press 'a' to add a note"))
		}
	} else {
		for i, note := range chapterNotes {
			verseNum := 0
			for _, v := range m.currentVerses {
				if v.PK == note.VersePK { verseNum = v.Verse; break }
			}
			prefix := "  "
			style := normalStyle
			if (m.mode == modeNoteSidebar || m.mode == modeReferenceList) && i == m.noteSidebarSelected {
				prefix = "> "; style = selectedStyle
			}
			refPrefix := fmt.Sprintf("%d:%d | ", m.currentChapter, verseNum)
			noteText := note.Text
			if len(noteText) > 150 { noteText = noteText[:147] + "..." }
			innerContent := lipgloss.NewStyle().Bold(style.GetBold()).Render(prefix) + verseRefStyle.Render(refPrefix) + m.highlightReferencesInText(noteText, noteTextStyle)
			sb.WriteString(style.Render(innerContent) + "\n")
			if len(note.References) > 0 {
				refStr := fmt.Sprintf("    Refs: %d", len(note.References))
				sb.WriteString(refCountStyle.Render(refStr) + "\n")
			}
		}
	}
	if m.mode == modeNoteSidebar {
		sb.WriteString("\n\n" + lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Render("j/k: nav | enter: edit\ng: go to ref\na: add | d: delete\nesc: back"))
	}
	return sb.String()
}

func (m Model) renderLanguageSelect(header, help, errorMsg string) string {
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
		Background(m.currentTheme.Background).
		Padding(0, 1)

	currentStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Background(m.currentTheme.Background).
		Padding(0, 1)

	moreStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Muted).
		Background(m.currentTheme.Background).
		Italic(true).
		Padding(0, 1)

	var content strings.Builder

	if m.languages != nil {
		// Calculate available lines: height - header - help - error - borders/padding
		headerHeight := lipgloss.Height(header)
		helpHeight := lipgloss.Height(help)
		errorHeight := 0
		if errorMsg != "" {
			errorHeight = lipgloss.Height(errorMsg)
		}
		availableLines := m.height - headerHeight - helpHeight - errorHeight - 6
		if availableLines < 1 {
			availableLines = 1
		}

		total := len(m.languages)
		visibleCount := availableLines
		if visibleCount > total {
			visibleCount = total
		}

		startIdx := m.languageSelected - (visibleCount / 2)
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := startIdx + visibleCount
		if endIdx > total {
			endIdx = total
			startIdx = endIdx - visibleCount
			if startIdx < 0 {
				startIdx = 0
			}
		}

		if startIdx > 0 {
			content.WriteString(moreStyle.Render(fmt.Sprintf("... (%d more above)", startIdx)) + "\n")
		}

		for i := startIdx; i < endIdx; i++ {
			lang := m.languages[i]
			prefix := "  "
			style := normalStyle
			suffix := ""

			isCurrent := lang == m.selectedLanguage

			if i == m.languageSelected {
				prefix = "> "
				style = selectedStyle
			} else if isCurrent {
				style = currentStyle
			}

			if isCurrent && i != m.languageSelected {
				suffix = " [Current]"
			}

			style = style.Width(m.width - 10)
			content.WriteString(style.Render(prefix+lang+suffix) + "\n")
		}

		if endIdx < total {
			content.WriteString(moreStyle.Render(fmt.Sprintf("... (%d more below)", total-endIdx)) + "\n")
		}
	} else {
		content.WriteString(normalStyle.Render("  Loading languages..."))
	}

	listContent := containerStyle.Width(m.width - 4).Render(strings.TrimSuffix(content.String(), "\n"))
	return fmt.Sprintf("%s\n%s\n%s%s", header, listContent, help, errorMsg)
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
		Background(m.currentTheme.Background).
		Padding(0, 1)

	currentStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Background(m.currentTheme.Background).
		Padding(0, 1)

	moreStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Muted).
		Background(m.currentTheme.Background).
		Italic(true).
		Padding(0, 1)

	var content strings.Builder

	if m.translations != nil {
		// Calculate available height: terminal height - header - help - error - borders - padding
		headerHeight := lipgloss.Height(header)
		helpHeight := lipgloss.Height(help)
		errorHeight := 0
		if errorMsg != "" {
			errorHeight = lipgloss.Height(errorMsg)
		}
		availableLines := m.height - headerHeight - helpHeight - errorHeight - 6
		if availableLines < 1 {
			availableLines = 1
		}

		total := len(m.translations)
		visibleCount := availableLines
		if visibleCount > total {
			visibleCount = total
		}

		// Calculate virtual scroll window
		startIdx := m.translationSelected - (visibleCount / 2)
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := startIdx + visibleCount
		if endIdx > total {
			endIdx = total
			startIdx = endIdx - visibleCount
			if startIdx < 0 {
				startIdx = 0
			}
		}

		if startIdx > 0 {
			content.WriteString(moreStyle.Render(fmt.Sprintf("... (%d more above)", startIdx)) + "\n")
		}

		for i := startIdx; i < endIdx; i++ {
			trans := m.translations[i]
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

			// Ensure width for background coloring
			style = style.Width(m.width - 10)
			content.WriteString(style.Render(prefix+name+suffix) + "\n")
		}

		if endIdx < total {
			content.WriteString(moreStyle.Render(fmt.Sprintf("... (%d more below)", total-endIdx)) + "\n")
		}
	} else {
		content.WriteString(normalStyle.Render("  Loading translations..."))
	}

	listContent := containerStyle.Width(m.width - 4).Render(strings.TrimSuffix(content.String(), "\n"))
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
		Background(m.currentTheme.Background).
		Padding(0, 1)

	cachedStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Background(m.currentTheme.Background).
		Padding(0, 1)

	downloadingStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Warning).
		Background(m.currentTheme.Background).
		Padding(0, 1)

	moreStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Muted).
		Background(m.currentTheme.Background).
		Italic(true).
		Padding(0, 1)

	var content strings.Builder

	if m.translations != nil {
		// Calculate available height: terminal height - header - help - error - borders - padding
		headerHeight := lipgloss.Height(header)
		helpHeight := lipgloss.Height(help)
		errorHeight := 0
		if errorMsg != "" {
			errorHeight = lipgloss.Height(errorMsg)
		}
		// -8 for borders/padding/spacing and extra for cache size line
		availableLines := m.height - headerHeight - helpHeight - errorHeight - 8
		if availableLines < 1 {
			availableLines = 1
		}

		total := len(m.translations)
		visibleCount := availableLines
		if visibleCount > total {
			visibleCount = total
		}

		// Calculate virtual scroll window
		startIdx := m.cacheSelected - (visibleCount / 2)
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := startIdx + visibleCount
		if endIdx > total {
			endIdx = total
			startIdx = endIdx - visibleCount
			if startIdx < 0 {
				startIdx = 0
			}
		}

		if startIdx > 0 {
			content.WriteString(moreStyle.Render(fmt.Sprintf("... (%d more above)", startIdx)) + "\n")
		}

		for i := startIdx; i < endIdx; i++ {
			trans := m.translations[i]
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
				if i != m.cacheSelected {
					style = cachedStyle
				}
			}

			// Ensure width for background coloring
			style = style.Width(m.width - 10)
			content.WriteString(style.Render(prefix+name+suffix) + "\n")
		}

		if endIdx < total {
			content.WriteString(moreStyle.Render(fmt.Sprintf("... (%d more below)", total-endIdx)) + "\n")
		}
	} else {
		content.WriteString(normalStyle.Render("  Loading translations..."))
	}

	// Show cache size if available
	if m.cache != nil {
		size, err := m.cache.GetCacheSize()
		if err == nil && size > 0 {
			sizeStr := fmt.Sprintf("\nCache Size: %.2f MB", float64(size)/(1024*1024))
			content.WriteString("\n" + normalStyle.Render(sizeStr))
		}
	}

	listContent := containerStyle.Width(m.width - 4).Render(strings.TrimSuffix(content.String(), "\n"))
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

		// Check if this verse is in the search-highlighted range (bordered)
		isSearchHighlighted := highlightedVerseStart > 0 && v.Verse >= highlightedVerseStart && v.Verse <= highlightedVerseEnd

		// Check if next verse is also search-highlighted
		nextIsSearchHighlighted := false
		if i+1 < len(verses) {
			nextVerse := verses[i+1]
			nextIsSearchHighlighted = highlightedVerseStart > 0 && nextVerse.Verse >= highlightedVerseStart && nextVerse.Verse <= highlightedVerseEnd
		}

		// Apply persistent highlights and cursor
		// A verse is focused if it's the start of the current viewport focus and we are in an interactive mode
		isFocusedVerse := v.Verse == m.highlightedVerseStart && (m.mode == modeVerseHighlight || m.mode == modeWordSelect)
		styledText := m.applyHighlightsToText(v.PK, text, isFocusedVerse)

		if isSearchHighlighted {
			if !inHighlightedRange {
				// Start of highlighted range
				inHighlightedRange = true
				highlightedContent.Reset()
			}

			verseNum := highlightedVerseStyle.Render(verseNumStr)

			// Calculate indent for wrapped lines (verse number width + 2 spaces)
			indent := 6
			// Account for border padding (2 chars on each side)
			wrappedText := wrapTextWithIndent(styledText, textWidth-4, indent)
			// Apply color with width set to prevent terminal wrapping
			verseText := highlightedTextStyle.Width(textWidth - 4).Render(wrappedText)

			highlightedContent.WriteString(fmt.Sprintf("%s  %s", verseNum, verseText))

			// If next verse is also highlighted, add spacing within the border
			if nextIsSearchHighlighted {
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
			wrappedText := wrapTextWithIndent(styledText, textWidth, indent)
			// Apply color with width set to prevent terminal wrapping
			verseText := textStyle.Width(textWidth).Render(wrappedText)

			sb.WriteString(fmt.Sprintf("%s  %s\n\n", verseNum, verseText))
		}
	}

	return sb.String()
}

func (m Model) applyHighlightsToText(versePK int, text string, isFocused bool) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}

	// 1. First pass: Map each original character to its persistent highlight style
	type charState struct {
		r     rune
		style lipgloss.Style
	}
	states := make([]charState, len(runes))
	for i, r := range runes {
		states[i] = charState{r: r, style: lipgloss.NewStyle()}
	}

	for _, h := range m.highlights {
		if h.VersePK == versePK {
			hStyle := lipgloss.NewStyle().Background(lipgloss.Color(h.Color)).Foreground(lipgloss.Color("#000000"))
			for i := h.Start; i < h.End && i < len(states); i++ {
				states[i].style = hStyle
			}
		}
	}

	// 2. Active character highlighting (real-time feedback)
	if isFocused && m.mode == modeVerseHighlight {
		if m.selectionStart != -1 {
			start, end := m.selectionStart, m.verseCursorPos
			if start > end { start, end = end, start }
			selStyle := lipgloss.NewStyle().Background(lipgloss.Color(m.selectedHighlightColor)).Foreground(lipgloss.Color("#000000")).Underline(true)
			for i := start; i <= end && i < len(states); i++ {
				states[i].style = selStyle
			}
		}
		// Cursor (highest priority)
		cursorStyle := lipgloss.NewStyle().Background(lipgloss.Color("#FFFFFF")).Foreground(lipgloss.Color("#000000")).Bold(true)
		if m.verseCursorPos >= 0 && m.verseCursorPos < len(states) {
			states[m.verseCursorPos].style = cursorStyle
		}
	}

	// 3. Group characters into words to handle symbol injection and word selection
	// We split by space but keep track of the original character states
	type wordRange struct {
		start, end int // indices into states slice
	}
	var wordRanges []wordRange
	currentStart := 0
	for i := 0; i <= len(runes); i++ {
		if i == len(runes) || runes[i] == ' ' {
			if i > currentStart {
				wordRanges = append(wordRanges, wordRange{currentStart, i})
			}
			currentStart = i + 1
		}
	}

	// Create map of notes for this verse
	verseNotes := make(map[int]settings.Note)
	for _, note := range m.notes {
		if note.VersePK == versePK && note.Translation == m.selectedTranslation {
			verseNotes[note.WordIndex] = note
		}
	}

	var finalResult strings.Builder
	for i, wr := range wordRanges {
		if i > 0 { finalResult.WriteString(" ") }

		// Build the word text with its character styles
		var wordContent strings.Builder
		for j := wr.start; j < wr.end; j++ {
			wordContent.WriteString(states[j].style.Render(string(states[j].r)))
		}
		wordStr := wordContent.String()

		// Apply word selection style (overlay)
		if isFocused && m.mode == modeWordSelect && i == m.wordIndex {
			// Use Inverse or a very distinct background for selection
			wordStr = lipgloss.NewStyle().
				Background(m.currentTheme.Accent).
				Foreground(lipgloss.Color("#000000")).
				Bold(true).
				Render(stripANSI(wordStr)) // Strip internal styles to prevent artifacts during selection
		}

		finalResult.WriteString(wordStr)

		// Inject note symbol AFTER the word
		if note, ok := verseNotes[i]; ok {
			symbolStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Warning).Bold(true)
			finalResult.WriteString(symbolStyle.Render("[" + note.Symbol + "]"))
		}
	}

	return finalResult.String()
}

// Helper to strip ANSI codes when we need raw text for a specific overlay style
func stripANSI(str string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return re.ReplaceAllString(str, "")
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	var currentLine strings.Builder
	currentLength := 0

	words := strings.Split(text, " ")
	for i, word := range words {
		// Use lipgloss.Width to get visual width excluding ANSI codes
		wordWidth := lipgloss.Width(word)

		// If adding this word would exceed width, start a new line
		if currentLength > 0 && currentLength+1+wordWidth > width {
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
		currentLength += wordWidth

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

	words := strings.Split(text, " ")
	for i, word := range words {
		wordWidth := lipgloss.Width(word)

		// If adding this word would exceed width, start a new line
		if currentLength > 0 && currentLength+1+wordWidth > width {
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
		currentLength += wordWidth

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
	s = strings.ReplaceAll(s, "&ldquo;", "\u201C")  // Left double quote
	s = strings.ReplaceAll(s, "&rdquo;", "\u201D")  // Right double quote
	s = strings.ReplaceAll(s, "&lsquo;", "\u2018")  // Left single quote
	s = strings.ReplaceAll(s, "&rsquo;", "\u2019")  // Right single quote
	s = strings.ReplaceAll(s, "&mdash;", "\u2014")  // Em dash
	s = strings.ReplaceAll(s, "&ndash;", "\u2013")  // En dash
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
		"genesis":         {"gen", "ge", "gn"},
		"exodus":          {"exo", "ex", "exod"},
		"leviticus":       {"lev", "le", "lv"},
		"numbers":         {"num", "nu", "nm", "nb"},
		"deuteronomy":     {"deut", "de", "dt"},
		"joshua":          {"josh", "jos", "jsh"},
		"judges":          {"judg", "jdg", "jg", "jdgs"},
		"ruth":            {"rut", "ru", "rth"},
		"1 samuel":        {"1sam", "1sa", "1samuel", "1 sam", "1 sa", "1s"},
		"2 samuel":        {"2sam", "2sa", "2samuel", "2 sam", "2 sa", "2s"},
		"1 kings":         {"1king", "1kgs", "1ki", "1k", "1 kings", "1 kgs"},
		"2 kings":         {"2king", "2kgs", "2ki", "2k", "2 kings", "2 kgs"},
		"1 chronicles":    {"1chron", "1chr", "1ch", "1 chronicles", "1 chr"},
		"2 chronicles":    {"2chron", "2chr", "2ch", "2 chronicles", "2 chr"},
		"ezra":            {"ezr", "ez"},
		"nehemiah":        {"neh", "ne"},
		"esther":          {"est", "es"},
		"job":             {"jb"},
		"psalms":          {"psalm", "psa", "ps", "pss"},
		"proverbs":        {"prov", "pro", "pr", "prv"},
		"ecclesiastes":    {"eccl", "ecc", "ec", "qoh"},
		"song of solomon": {"song", "sos", "so", "canticle", "canticles", "song of songs"},
		"isaiah":          {"isa", "is"},
		"jeremiah":        {"jer", "je", "jr"},
		"lamentations":    {"lam", "la"},
		"ezekiel":         {"ezek", "eze", "ezk"},
		"daniel":          {"dan", "da", "dn"},
		"hosea":           {"hos", "ho"},
		"joel":            {"joe", "jl"},
		"amos":            {"amo", "am"},
		"obadiah":         {"obad", "ob"},
		"jonah":           {"jon", "jnh"},
		"micah":           {"mic", "mi"},
		"nahum":           {"nah", "na"},
		"habakkuk":        {"hab", "hb"},
		"zephaniah":       {"zeph", "zep", "zp"},
		"haggai":          {"hag", "hg"},
		"zechariah":       {"zech", "zec", "zc"},
		"malachi":         {"mal", "ml"},
		"matthew":         {"matt", "mat", "mt"},
		"mark":            {"mar", "mrk", "mk", "mr"},
		"luke":            {"luk", "lk"},
		"john":            {"joh", "jhn", "jn"},
		"acts":            {"act", "ac"},
		"romans":          {"rom", "ro", "rm"},
		"1 corinthians":   {"1cor", "1co", "1 corinthians", "1 cor"},
		"2 corinthians":   {"2cor", "2co", "2 corinthians", "2 cor"},
		"galatians":       {"gal", "ga"},
		"ephesians":       {"eph", "ephes"},
		"philippians":     {"phil", "php", "pp"},
		"colossians":      {"col", "co"},
		"1 thessalonians": {"1thess", "1th", "1 thessalonians", "1 thess"},
		"2 thessalonians": {"2thess", "2th", "2 thessalonians", "2 thess"},
		"1 timothy":       {"1tim", "1ti", "1 timothy", "1 tim"},
		"2 timothy":       {"2tim", "2ti", "2 timothy", "2 tim"},
		"titus":           {"tit", "ti"},
		"philemon":        {"philem", "phm", "pm"},
		"hebrews":         {"heb", "he"},
		"james":           {"jam", "jas", "jm"},
		"1 peter":         {"1pet", "1pe", "1pt", "1p", "1 peter", "1 pet"},
		"2 peter":         {"2pet", "2pe", "2pt", "2p", "2 peter", "2 pet"},
		"1 john":          {"1john", "1jn", "1jo", "1j", "1 john"},
		"2 john":          {"2john", "2jn", "2jo", "2j", "2 john"},
		"3 john":          {"3john", "3jn", "3jo", "3j", "3 john"},
		"jude":            {"jud", "jd"},
		"revelation":      {"rev", "re", "rv"},
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
		{"/", "Search for verse reference"},
		{"s", "Search Bible for word/phrase"},
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

func (m Model) renderWordSearch(header, help, errorMsg string) string {
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

	verseNumStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Secondary).
		Bold(true)

	bookNameStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent)

	var content strings.Builder

	// Show search input if no results yet
	if m.wordSearchResults == nil && !m.wordSearchLoading {
		content.WriteString("Enter search term:\n\n")
		content.WriteString(m.wordSearchInput.View())
		content.WriteString("\n\nPress Enter to search, Esc to cancel")
	} else if m.wordSearchLoading {
		content.WriteString("Searching...")
	} else if len(m.wordSearchResults) == 0 {
		content.WriteString(fmt.Sprintf("No results found for \"%s\"", m.wordSearchQuery))
		content.WriteString("\n\nPress Esc to go back")
	} else {
		// Show results count
		content.WriteString(fmt.Sprintf("Found %d results for \"%s\" (showing %d)\n\n",
			m.wordSearchTotal, m.wordSearchQuery, len(m.wordSearchResults)))

		// Calculate visible window for virtual scrolling
		headerHeight := lipgloss.Height(header)
		helpHeight := lipgloss.Height(help)
		errorHeight := 0
		if errorMsg != "" {
			errorHeight = lipgloss.Height(errorMsg)
		}
		// Calculate available lines: height - header - help - error - borders/padding/count line
		visibleItems := m.height - headerHeight - helpHeight - errorHeight - 10
		if visibleItems < 5 {
			visibleItems = 5
		}

		startIdx := 0
		if m.wordSearchSelected >= visibleItems {
			startIdx = m.wordSearchSelected - visibleItems + 1
		}
		endIdx := startIdx + visibleItems
		if endIdx > len(m.wordSearchResults) {
			endIdx = len(m.wordSearchResults)
		}

		// Show "more above" indicator
		if startIdx > 0 {
			content.WriteString(fmt.Sprintf("  ... (%d more above)\n", startIdx))
		}

		// Group results by book for display
		currentBook := -1
		for i := startIdx; i < endIdx; i++ {
			result := m.wordSearchResults[i]

			// Show book header when book changes
			if result.Book != currentBook {
				currentBook = result.Book
				bookName := m.getBookName(result.Book)
				content.WriteString("\n" + bookNameStyle.Render(bookName) + "\n")
			}

			// Format verse reference
			ref := fmt.Sprintf("%d:%d", result.Chapter, result.Verse)
			verseText := stripHTMLTags(result.Text)

			// Truncate long text
			maxLen := m.width - 20
			if maxLen < 30 {
				maxLen = 30
			}
			if len(verseText) > maxLen {
				verseText = verseText[:maxLen-3] + "..."
			}

			line := verseNumStyle.Render(ref) + " " + verseText

			if i == m.wordSearchSelected {
				content.WriteString(selectedStyle.Render("> "+line) + "\n")
			} else {
				content.WriteString(normalStyle.Render("  "+line) + "\n")
			}
		}

		// Show "more below" indicator
		if endIdx < len(m.wordSearchResults) {
			content.WriteString(fmt.Sprintf("  ... (%d more below)\n", len(m.wordSearchResults)-endIdx))
		}
	}

	listContent := containerStyle.Render(content.String())
	return fmt.Sprintf("%s\n%s\n%s%s", header, listContent, help, errorMsg)
}

func (m Model) getBookName(bookID int) string {
	if m.books != nil {
		for _, book := range m.books {
			if book.BookID == bookID {
				return book.Name
			}
		}
	}
	return fmt.Sprintf("Book %d", bookID)
}

