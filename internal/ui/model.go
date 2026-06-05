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
	"time"

	"github.com/atotto/clipboard"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
)

type focusPane int

const (
	paneBooks focusPane = iota
	paneContent
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
	// Translation selection state
	translationSelected int
	// Theme state
	currentTheme  theme.Theme
	themeSelected int
	// Word search state
	wordSearchInput    textinput.Model
	wordSearchQuery    string
	wordSearchResults  []api.Verse
	wordSearchTotal    int
	wordSearchSelected int
	wordSearchLoading  bool
	// Pane focus (book list vs content)
	focus focusPane
	// themePinned is true when the user has an explicit theme stored in
	// settings (or has picked one this session). When false, an incoming
	// BackgroundColorMsg from the terminal may swap us to a light or
	// dark default.
	themePinned bool
	// topVisibleVerse mirrors the verse number currently at the top of
	// the viewport. The right pane title surfaces it as a sticky scroll
	// indicator so the reader always knows where they are.
	topVisibleVerse int
	// Last known mouse position. Updated on every MouseClickMsg /
	// MouseMotionMsg / MouseWheelMsg. The render functions read these
	// to surface hover state (book row hover in the left pane, verse
	// number hover in the right pane title).
	mouseX, mouseY int
	// dragAnchorVerse holds the verse number where the user pressed
	// the left mouse button in the reader. While the button is held
	// and the mouse moves, the highlighted range is extended from this
	// anchor to whichever verse the cursor is now over. 0 means no
	// drag in progress.
	dragAnchorVerse int
	// comparisonPickerColumn is the column index in comparisonTranslations
	// whose translation is currently being swapped via the translation
	// picker. -1 means the picker is not scoped to a column (i.e. the
	// normal "set the active translation" flow).
	comparisonPickerColumn int
	// Translation download progress in [0, 1]. Polled from the cache
	// every ~120ms while a download is running.
	downloadProgress float64
	progressBar      progress.Model
}

type CacheInterface interface {
	IsCached(translation string) bool
	GetChapter(translation string, book, chapter int) ([]api.Verse, error)
	GetVerse(translation string, book, chapter, verse int) (*api.Verse, error)
	DownloadTranslation(translation string) error
	// DownloadProgress reports the byte-level progress of the currently
	// running download as a value in [0, 1] and the translation
	// short-name being downloaded ("" if idle). Safe to call from any
	// goroutine.
	DownloadProgress() (float64, string)
	ListCached() ([]string, error)
	GetCacheSize() (int64, error)
	RemoveTranslation(translation string) error
	ClearCache() error
}

type (
	errMsg                  struct{ err error }
	translationsLoadedMsg   struct{ translations []api.Translation }
	booksLoadedMsg          struct{ books []api.Book }
	chapterLoadedMsg        struct{ verses []api.Verse }
	parallelVersesLoadedMsg struct{ verses map[string][]api.Verse }
	cacheListLoadedMsg      struct{ translations []string }
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

// downloadTickMsg fires roughly every 120ms while a translation download
// is running so the UI can poll the cache for byte-level progress.
type downloadTickMsg struct{}

func downloadTick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return downloadTickMsg{}
	})
}

func (e errMsg) Error() string { return e.err.Error() }

func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "Enter verse reference (e.g., 1 1:1 or Gen 1:1)"
	ti.Focus()
	ti.CharLimit = 50
	ti.SetWidth(50)

	millerFilter := textinput.New()
	millerFilter.Placeholder = "Type to filter..."
	millerFilter.CharLimit = 50
	millerFilter.SetWidth(25)

	wordSearch := textinput.New()
	wordSearch.Placeholder = "Search the Bible..."
	wordSearch.CharLimit = 100
	wordSearch.SetWidth(50)

	// --- Load persisted settings (if any) ---
	cfg, err := settings.Load()

	selectedTranslation := "NLT"
	currentBook := 1
	currentChapter := 1
	currentTheme := theme.CatppuccinMocha

	if err == nil {
		if cfg.SelectedTranslation != "" {
			selectedTranslation = cfg.SelectedTranslation
		}
		if cfg.CurrentBook > 0 {
			currentBook = cfg.CurrentBook
		}
		if cfg.CurrentChapter > 0 {
			currentChapter = cfg.CurrentChapter
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
		selectedTranslation:    selectedTranslation,
		currentBook:            currentBook,
		currentChapter:         currentChapter,
		currentBookName:        "Genesis", // corrected after books load
		mode:                   modeReader,
		comparisonTranslations: []string{"NLT", "KJV", "WEB"},
		currentTheme:           currentTheme,
		themeSelected:          0,
		focus:                  paneContent,
		// If the user had a theme stored in settings, treat it as pinned
		// so auto-detect from the terminal background doesn't override it.
		themePinned:            err == nil && cfg.CurrentTheme != "",
		progressBar:            progress.New(progress.WithDefaultBlend(), progress.WithoutPercentage()),
		comparisonPickerColumn: -1,
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
		// Ask the terminal for its background color so we can auto-pick
		// a light or dark default theme if the user hasn't pinned one.
		tea.RequestBackgroundColor,
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
		case "ctrl+c", "q":
			// Save settings synchronously before quitting to avoid race condition
			cfg := settings.Settings{
				SelectedTranslation: m.selectedTranslation,
				CurrentBook:         m.currentBook,
				CurrentChapter:      m.currentChapter,
				CurrentTheme:        m.currentTheme.Name,
			}
			_ = settings.Save(cfg)
			return m, tea.Quit
		case "[":
			if m.mode == modeReader {
				m.focus = paneBooks
				if m.books != nil {
					for i, book := range m.books {
						if book.BookID == m.currentBook {
							m.sidebarSelected = i
							break
						}
					}
				}
				return m, nil
			}
		case "]":
			if m.mode == modeReader {
				m.focus = paneContent
				return m, nil
			}
		case "tab":
			if m.mode == modeReader {
				if m.focus == paneBooks {
					m.focus = paneContent
				} else {
					m.focus = paneBooks
					if m.books != nil && m.sidebarSelected == 0 {
						for i, book := range m.books {
							if book.BookID == m.currentBook {
								m.sidebarSelected = i
								break
							}
						}
					}
				}
				return m, nil
			}
		case "shift+tab":
			if m.mode == modeReader {
				if m.focus == paneBooks {
					m.focus = paneContent
				} else {
					m.focus = paneBooks
				}
				return m, nil
			}
		case "v":
			if m.mode == modeReader {
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
				m.focus = paneContent
				m.mode = modeSearch
				m.textInput.Focus()
				return m, nil
			}
		case "up", "k":
			if m.mode == modeWordSearch && m.wordSearchResults != nil && m.wordSearchSelected > 0 {
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
			} else if m.focus == paneBooks && m.sidebarSelected > 0 {
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
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.viewport.Width(), m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
					m.scrollToHighlightedVerse()
				}
				return m, nil
			}
		case "down", "j":
			if m.mode == modeWordSearch && m.wordSearchResults != nil && m.wordSearchSelected < len(m.wordSearchResults)-1 {
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
			} else if m.focus == paneBooks && m.books != nil && m.sidebarSelected < len(m.books)-1 {
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
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.viewport.Width(), m.highlightedVerseStart, m.highlightedVerseEnd)
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
			if m.mode == modeReader {
				m.mode = modeComparison
				verses := []int{}
				for i := 1; i <= 31; i++ {
					verses = append(verses, i)
				}
				return m, loadParallelVerses(m.client, m.comparisonTranslations, m.currentBook, m.currentChapter, verses)
			}
		case "r":
			// Don't intercept 'r' when typing in search inputs
			if m.mode == modeSearch {
				// Let it pass through to verse reference input
			} else if m.mode == modeWordSearch && m.wordSearchResults == nil && !m.wordSearchLoading {
				// Let it pass through to word search input
			} else if m.mode != modeReader {
				m.mode = modeReader
				return m, nil
			}
		case "t":
			if m.mode == modeReader {
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
			if m.mode == modeReader {
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
			if m.mode == modeReader {
				m.mode = modeCacheManager
				m.cacheSelected = 0
				if m.cache != nil {
					return m, loadCachedList(m.cache)
				}
				return m, nil
			}
		case "?":
			if m.mode == modeReader {
				m.mode = modeAbout
				return m, nil
			}
		case "s":
			if m.mode == modeReader {
				m.mode = modeWordSearch
				m.wordSearchInput.SetValue("")
				m.wordSearchInput.Focus()
				m.wordSearchResults = nil
				m.wordSearchSelected = 0
				m.wordSearchLoading = false
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
				newTrans := m.translations[m.translationSelected].ShortName
				// Picker was opened from a comparison column header:
				// swap that column instead of changing the main reader.
				if m.comparisonPickerColumn >= 0 && m.comparisonPickerColumn < len(m.comparisonTranslations) {
					m.comparisonTranslations[m.comparisonPickerColumn] = newTrans
					m.comparisonPickerColumn = -1
					m.mode = modeComparison
					m.loading = true
					return m, loadParallelVerses(m.client, m.comparisonTranslations, m.currentBook, m.currentChapter, m.comparisonVerseList())
				}
				m.selectedTranslation = newTrans
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
				m.themePinned = true
				m.mode = modeReader
				return m, nil
			} else if m.mode == modeCacheManager && m.translations != nil && m.cacheSelected < len(m.translations) {
				// Download selected translation
				translation := m.translations[m.cacheSelected].ShortName
				if m.cache != nil && !m.cache.IsCached(translation) {
					m.downloadingTranslation = translation
					m.downloadProgress = 0
					return m, tea.Batch(downloadTranslation(m.cache, translation), downloadTick())
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
			} else if m.focus == paneBooks && m.books != nil {
				// Select book from sidebar
				if m.sidebarSelected < len(m.books) {
					m.currentBook = m.books[m.sidebarSelected].BookID
					m.currentBookName = m.books[m.sidebarSelected].Name
					m.currentChapter = 1
					m.focus = paneContent
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
					query := m.wordSearchInput.Value()
					if query != "" {
						// If the query contains digits AND parses as a verse
						// reference (e.g. "rom8", "rom 8:8", "john 3:16"),
						// jump there instead of doing a full-text search.
						// Plain words like "love" or "rom" fall through to
						// the full-text path.
						if strings.ContainsAny(query, "0123456789") {
							if book, chapter, vs, ve, refErr := parseReference(query, m.books); refErr == nil && book > 0 {
								m.currentBook = book
								m.currentChapter = chapter
								m.highlightedVerseStart = vs
								m.highlightedVerseEnd = ve
								for _, b := range m.books {
									if b.BookID == book {
										m.currentBookName = b.Name
										break
									}
								}
								m.mode = modeReader
								m.loading = true
								m.wordSearchInput.SetValue("")
								m.wordSearchInput.Blur()
								return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
							}
						}
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
			if m.mode == modeSearch || m.mode == modeTranslationSelect || m.mode == modeThemeSelect || m.mode == modeAbout || m.mode == modeComparison || m.mode == modeWordSearch || m.mode == modeCacheManager {
				// Picker was opened from a comparison column: dismiss
				// it back into comparison view instead of dropping all
				// the way down to the reader.
				if m.mode == modeTranslationSelect && m.comparisonPickerColumn >= 0 {
					m.comparisonPickerColumn = -1
					m.mode = modeComparison
					return m, nil
				}
				m.mode = modeReader
				m.wordSearchResults = nil
				m.wordSearchInput.SetValue("")
				return m, nil
			}
			if m.focus == paneBooks {
				m.focus = paneContent
				return m, nil
			}
		}

	case tea.MouseClickMsg:
		m.mouseX, m.mouseY = msg.X, msg.Y
		if msg.Button != tea.MouseLeft {
			break
		}

		// Overlay is active: clicks inside its panel select items;
		// clicks outside close the overlay.
		if m.overlayActive() {
			px, py, pw, ph := m.overlayPanelBounds()
			inside := msg.X >= px && msg.X < px+pw && msg.Y >= py && msg.Y < py+ph
			if !inside {
				// Picker scoped to a comparison column drops back to
				// comparison view, not all the way to the reader.
				if m.mode == modeTranslationSelect && m.comparisonPickerColumn >= 0 {
					m.comparisonPickerColumn = -1
					m.mode = modeComparison
					return m, nil
				}
				m.mode = modeReader
				m.wordSearchResults = nil
				m.wordSearchInput.SetValue("")
				return m, nil
			}
			// Inner content area sits one row in from the top border and
			// one row down for the box's top padding, plus the panel's
			// title line and its trailing blank line. Item rows then start.
			rowInPanel := msg.Y - py - 1 - 1 - 1 - 1
			cmd := m.overlayClick(rowInPanel)
			return m, cmd
		}

		// Click in the left (books) pane — select & load that book.
		if msg.X >= 0 && msg.X < leftPaneOuterWidth && msg.Y >= headerOuterHeight+2 {
			m.focus = paneBooks
			if i, ok := m.bookAtRow(msg.Y); ok {
				m.sidebarSelected = i
				m.currentBook = m.books[i].BookID
				m.currentBookName = m.books[i].Name
				m.currentChapter = 1
				m.focus = paneContent
				m.loading = true
				return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
			}
			return m, nil
		}

		// Comparison view: click on a column header opens the
		// translation picker scoped to that column.
		if m.mode == modeComparison && msg.X >= leftPaneOuterWidth {
			viewportTopY := headerOuterHeight + 4 // app header (3) + pane border (1) + top pad (1) + title (1) + spacer (1) - 2
			// Header occupies the first 2 lines of the viewport
			// (translation labels + ─ separator). Only trigger when
			// the viewport is at the top so the click coordinates match.
			if m.viewport.YOffset() == 0 && (msg.Y == viewportTopY || msg.Y == viewportTopY+1) {
				if col := m.comparisonColumnAtX(msg.X); col >= 0 {
					m.comparisonPickerColumn = col
					m.mode = modeTranslationSelect
					m.translationSelected = 0
					if m.translations != nil {
						for i, t := range m.translations {
							if t.ShortName == m.comparisonTranslations[col] {
								m.translationSelected = i
								break
							}
						}
					}
					return m, nil
				}
			}
		}

		// Click in the right (content) pane — focus it, and if the click
		// landed on a verse line, treat it as a selection.
		//
		//   plain left-click       → highlight just that verse, record
		//                            it as the drag anchor so a
		//                            subsequent drag extends a range
		//   shift+left-click       → extend the existing highlight to
		//                            span from the previous anchor to
		//                            the clicked verse
		//   left-click and drag    → range grows live as the mouse moves
		//                            (handled in MouseMotionMsg)
		//
		// The range is always normalized so start ≤ end.
		if msg.X >= leftPaneOuterWidth {
			m.focus = paneContent
			if v := m.verseAtMouseY(msg.Y); v > 0 && m.mode == modeReader && m.currentVerses != nil {
				if msg.Mod&tea.ModShift != 0 && m.highlightedVerseStart > 0 {
					anchor := m.highlightedVerseStart
					if m.highlightedVerseStart > m.highlightedVerseEnd {
						anchor = m.highlightedVerseEnd
					}
					start, end := anchor, v
					if start > end {
						start, end = end, start
					}
					m.highlightedVerseStart = start
					m.highlightedVerseEnd = end
				} else {
					m.highlightedVerseStart = v
					m.highlightedVerseEnd = v
					m.dragAnchorVerse = v
				}
				m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.viewport.Width(), m.highlightedVerseStart, m.highlightedVerseEnd)
				m.viewport.SetContent(m.content)
			}
		}

	case tea.MouseReleaseMsg:
		m.mouseX, m.mouseY = msg.X, msg.Y
		// End of a click-and-drag selection.
		m.dragAnchorVerse = 0

	case tea.MouseMotionMsg:
		// Skip the model update when the cursor didn't actually change
		// cells so we avoid a re-render on every micro-movement.
		if msg.X == m.mouseX && msg.Y == m.mouseY {
			return m, nil
		}
		m.mouseX, m.mouseY = msg.X, msg.Y

		// If the left button is held AND we recorded a drag anchor on
		// the original click, extend the highlight range live to span
		// from the anchor to whichever verse is now under the cursor.
		// This is the modifier-less alternative to shift+click — any
		// terminal that passes mouse motion through (the same ones
		// that pass clicks) will let it work.
		if msg.Button == tea.MouseLeft && m.dragAnchorVerse > 0 && m.mode == modeReader && m.currentVerses != nil {
			if v := m.verseAtMouseY(msg.Y); v > 0 {
				start, end := m.dragAnchorVerse, v
				if start > end {
					start, end = end, start
				}
				if start != m.highlightedVerseStart || end != m.highlightedVerseEnd {
					m.highlightedVerseStart = start
					m.highlightedVerseEnd = end
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.viewport.Width(), m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
				}
			}
		}
		return m, nil

	case tea.MouseWheelMsg:
		m.mouseX, m.mouseY = msg.X, msg.Y
		// Inside an active overlay, the wheel navigates its selection.
		if m.overlayActive() {
			switch msg.Button {
			case tea.MouseWheelUp:
				m.overlayNudge(-1)
			case tea.MouseWheelDown:
				m.overlayNudge(+1)
			}
			return m, nil
		}
		// Inside the books pane, the wheel moves the highlighted book.
		if msg.X >= 0 && msg.X < leftPaneOuterWidth && m.books != nil {
			switch msg.Button {
			case tea.MouseWheelUp:
				if m.sidebarSelected > 0 {
					m.sidebarSelected--
				}
			case tea.MouseWheelDown:
				if m.sidebarSelected < len(m.books)-1 {
					m.sidebarSelected++
				}
			}
			m.focus = paneBooks
			return m, nil
		}
		// Otherwise forward to the viewport in the content pane.
		if m.focus == paneContent || msg.X >= leftPaneOuterWidth {
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.BackgroundColorMsg:
		// Only act on the first BackgroundColorMsg if the user hasn't
		// pinned a theme. Pick a sensible default for the terminal's
		// luma. This runs once at startup (Init asks for the bg color).
		if !m.themePinned {
			var chosen theme.Theme
			if msg.IsDark() {
				chosen = theme.CatppuccinMocha
			} else {
				chosen = theme.CatppuccinLatte
			}
			m.currentTheme = chosen
			// Sync themeSelected so the picker opens on the right row
			// next time the user presses T.
			for i, th := range theme.AllThemes() {
				if th.Name == chosen.Name {
					m.themeSelected = i
					break
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		vpW, vpH := m.viewportSize()

		if !m.ready {
			m.viewport = viewport.New(viewport.WithWidth(vpW), viewport.WithHeight(vpH))
			m.viewport.YPosition = 4
			m.ready = true
		} else {
			m.viewport.SetWidth(vpW)
			m.viewport.SetHeight(vpH)
		}

		// Reformat content with new width
		if m.currentVerses != nil {
			m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, vpW, m.highlightedVerseStart, m.highlightedVerseEnd)
		} else if m.currentParallelVerses != nil {
			m.content = m.formatParallelVerses(m.currentParallelVerses, m.comparisonTranslations, m.currentBookName, m.currentChapter, vpW)
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
		m.content = m.formatChapter(msg.verses, m.currentBookName, m.currentChapter, m.viewport.Width(), m.highlightedVerseStart, m.highlightedVerseEnd)
		m.viewport.SetContent(m.content)

		// If we came from a search, scroll to the highlighted verse
		if cameFromSearch {
			m.scrollToHighlightedVerse()
			m.topVisibleVerse = m.highlightedVerseStart
		} else {
			m.viewport.GotoTop()
			m.topVisibleVerse = 0
		}

	case parallelVersesLoadedMsg:
		m.loading = false
		m.currentParallelVerses = msg.verses
		m.currentVerses = nil
		m.content = m.formatParallelVerses(msg.verses, m.comparisonTranslations, m.currentBookName, m.currentChapter, m.viewport.Width())
		m.viewport.SetContent(m.content)
		m.viewport.GotoTop()

	case cacheListLoadedMsg:
		m.cachedTranslations = msg.translations

	case downloadCompleteMsg:
		m.downloadingTranslation = ""
		m.downloadProgress = 0
		if m.cache != nil {
			return m, loadCachedList(m.cache)
		}

	case downloadErrorMsg:
		m.downloadingTranslation = ""
		m.downloadProgress = 0
		m.err = msg.err

	case downloadTickMsg:
		// Poll the cache for current byte-level progress and reschedule
		// the next tick if a download is still running.
		if m.downloadingTranslation != "" && m.cache != nil {
			p, _ := m.cache.DownloadProgress()
			m.downloadProgress = p
			return m, downloadTick()
		}

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
		oldYOffset := m.viewport.YOffset()
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

		// Update highlighted verse + the sticky-header top-visible verse
		// based on viewport position.
		if m.currentVerses != nil && oldYOffset != m.viewport.YOffset() {
			newTopVerse := m.calculateHighlightedVerse()
			m.topVisibleVerse = newTopVerse
			if m.viewport.YOffset() == 0 {
				// Back at the top: drop the sticky indicator.
				m.topVisibleVerse = 0
			}
			if newTopVerse != m.highlightedVerseStart {
				m.highlightedVerseStart = newTopVerse
				m.highlightedVerseEnd = newTopVerse
				m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.viewport.Width(), m.highlightedVerseStart, m.highlightedVerseEnd)
				m.viewport.SetContent(m.content)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() tea.View {
	return tea.View{
		Content:   m.renderView(),
		AltScreen: true,
		// AllMotion gives us motion events even when no button is held,
		// which is what we need for hover highlights.
		MouseMode: tea.MouseModeAllMotion,
	}
}

// Layout constants for the two-pane shell.
const (
	leftPaneOuterWidth = 30 // books pane outer width incl. rounded border
	headerOuterHeight  = 3  // header rounded box: 1 content + 2 border lines
	statusOuterHeight  = 3  // status bar rounded box: same
)

// viewportSize returns the inner content width/height of the right pane.
// The right pane outer width is m.width - leftPaneOuterWidth.
// We subtract: 2 for the rounded border and 4 for padding(1, 2) so the
// viewport fills the pane's inner content area exactly (no unstyled gap
// at the right edge).
func (m Model) viewportSize() (int, int) {
	w := m.width - leftPaneOuterWidth - 2 - 4
	if w < 20 {
		w = 20
	}
	h := m.height - headerOuterHeight - statusOuterHeight - 2 - 2 - 2
	if h < 5 {
		h = 5
	}
	return w, h
}

// overlayActive reports whether the current mode draws a floating panel
// on top of the two-pane shell.
func (m Model) overlayActive() bool {
	switch m.mode {
	case modeSearch, modeTranslationSelect, modeThemeSelect,
		modeCacheManager, modeAbout, modeWordSearch:
		return true
	}
	return false
}

func (m Model) renderView() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	if m.width < 60 || m.height < 18 {
		fitStyle := lipgloss.NewStyle().
			Foreground(m.currentTheme.Warning).
			Bold(true)
		return "\n  " + fitStyle.Render("Terminal too small — resize to at least 60×18.")
	}

	header := m.renderHeader()
	body := m.renderBody()
	status := m.renderStatusBar()

	base := lipgloss.JoinVertical(lipgloss.Left, header, body, status)

	if !m.overlayActive() {
		return base
	}

	return m.composeOverlay(base)
}

func (m Model) renderHeader() string {
	width := m.width

	bg := m.currentTheme.Background
	accent := m.currentTheme.Accent
	successCol := m.currentTheme.Success

	logo := "†"
	logoStyle := lipgloss.NewStyle().Foreground(accent).Background(bg).Bold(true)
	breadcrumbStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Background(bg)
	separatorStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg)
	versionStyle := lipgloss.NewStyle().Foreground(successCol).Background(bg).Bold(true)

	bookName := m.currentBookName
	if bookName == "" {
		bookName = "—"
	}
	chapter := fmt.Sprintf("%d", m.currentChapter)
	if m.currentChapter == 0 {
		chapter = "—"
	}

	breadcrumb := logoStyle.Render(logo+" sword-tui") +
		separatorStyle.Render("  ·  ") +
		breadcrumbStyle.Render(m.selectedTranslation) +
		separatorStyle.Render(" › ") +
		breadcrumbStyle.Render(bookName) +
		separatorStyle.Render(" › ") +
		breadcrumbStyle.Render(chapter)

	versionStr := versionStyle.Render(version.Version)

	innerWidth := width - 4 - 2 // -2 border -2 padding -2 safety
	rightW := lipgloss.Width(versionStr)
	leftW := innerWidth - rightW - 1
	if leftW < 1 {
		leftW = 1
	}
	leftSlot := lipgloss.NewStyle().
		Background(bg).
		Width(leftW).
		MaxWidth(leftW).
		Render(breadcrumb)
	gap := lipgloss.NewStyle().Background(bg).Render(" ")
	content := leftSlot + gap + versionStr

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(width - 2).
		Padding(0, 1)

	return box.Render(content)
}

func (m Model) renderStatusBar() string {
	width := m.width
	bg := m.currentTheme.Background

	hintStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg)
	keyStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
	rightStyle := lipgloss.NewStyle().Background(bg)

	hints := m.statusHints(keyStyle, hintStyle)

	// Right side: loading indicator or error condensed
	var right string
	if m.loading {
		right = lipgloss.NewStyle().Foreground(m.currentTheme.Warning).Background(bg).Bold(true).Render("● loading")
	} else if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Error).Background(bg).Bold(true)
		msg := m.err.Error()
		if len(msg) > 40 {
			msg = msg[:37] + "..."
		}
		right = errStyle.Render("⚠ " + msg)
	} else if m.cache != nil && m.cache.IsCached(m.selectedTranslation) {
		right = lipgloss.NewStyle().Foreground(m.currentTheme.Success).Background(bg).Render("● offline")
	} else {
		right = hintStyle.Render("● online")
	}

	innerWidth := width - 4 - 2 // -2 border -2 padding -2 safety
	rightW := lipgloss.Width(right)
	leftW := innerWidth - rightW - 1
	if leftW < 1 {
		leftW = 1
	}
	hintsSlot := rightStyle.Width(leftW).MaxWidth(leftW).Render(hints)
	gap := lipgloss.NewStyle().Background(bg).Render(" ")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.Border).
		BorderBackground(bg).
		Background(bg).
		Width(width - 2).
		Padding(0, 1)

	return box.Render(hintsSlot + gap + right)
}

func (m Model) statusHints(key, dim lipgloss.Style) string {
	type hint struct{ k, label string }

	var hs []hint
	switch m.mode {
	case modeTranslationSelect, modeThemeSelect:
		hs = []hint{{"↑↓", "navigate"}, {"⏎", "select"}, {"esc", "close"}}
	case modeCacheManager:
		hs = []hint{{"↑↓", "navigate"}, {"⏎", "download"}, {"x", "delete"}, {"esc", "close"}}
	case modeAbout:
		hs = []hint{{"esc", "close"}}
	case modeWordSearch:
		if m.wordSearchResults != nil {
			hs = []hint{{"↑↓", "navigate"}, {"⏎", "go to verse"}, {"esc", "close"}}
		} else {
			hs = []hint{{"⏎", "search"}, {"esc", "close"}}
		}
	case modeComparison:
		hs = []hint{{"↑↓", "scroll"}, {"r", "reader"}, {"esc", "back"}}
	case modeSearch:
		hs = []hint{{"⏎", "go"}, {"esc", "cancel"}}
	default:
		hs = []hint{
			{"tab", "focus"},
			{"⏎", "open"},
			{"n/p", "chapter"},
			{"t", "translation"},
			{"T", "theme"},
			{"/", "verse"},
			{"s", "search"},
			{"?", "about"},
			{"q", "quit"},
		}
	}

	// Each part bundles the key + leading space + label into a single
	// dim.Render() so the gap between the styled key and label gets the
	// pane background (raw spaces between two styled spans would inherit
	// the terminal default and show through as black blocks).
	var parts []string
	for _, h := range hs {
		parts = append(parts, key.Render(h.k)+dim.Render(" "+h.label))
	}
	return strings.Join(parts, dim.Render("  ·  "))
}

func (m Model) renderBody() string {
	bodyHeight := m.height - headerOuterHeight - statusOuterHeight
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	leftW := leftPaneOuterWidth
	rightW := m.width - leftW

	left := m.renderLeftPane(leftW, bodyHeight)
	right := m.renderRightPane(rightW, bodyHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) renderLeftPane(outerW, outerH int) string {
	active := m.focus == paneBooks && !m.overlayActive()
	border := m.currentTheme.Border
	if active {
		border = m.currentTheme.BorderActive
	}
	bg := m.currentTheme.Background

	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Success).Background(bg).Bold(true)
	selectedStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Background).
		Background(m.currentTheme.Accent).
		Bold(true)
	currentStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Warning).Background(bg).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Background(bg)
	hoverStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Background(m.currentTheme.Highlight)
	mutedStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)

	// Resolve hover only when the mouse is actually inside this pane.
	hoveredBookIdx := -1
	if !m.overlayActive() && m.mouseX >= 0 && m.mouseX < leftPaneOuterWidth {
		if hi, ok := m.bookAtRow(m.mouseY); ok {
			hoveredBookIdx = hi
		}
	}

	innerW := outerW - 4 // 2 border + 2 padding
	innerH := outerH - 4 // 2 border + 2 padding

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Books"))
	sb.WriteString("\n")

	// Reserve: 1 for the "Books" title above, plus 2 (one per direction)
	// for the "↑ N more" / "↓ N more" indicators when virtual scrolling.
	contentLines := innerH - 1 - 2
	if contentLines < 1 {
		contentLines = 1
	}

	if m.books == nil {
		sb.WriteString(mutedStyle.Render("Loading…"))
	} else {
		type entry struct {
			isHeader bool
			label    string
			bookIdx  int
			isCur    bool
			isSel    bool
		}
		var entries []entry
		entries = append(entries, entry{isHeader: true, label: "OLD TESTAMENT"})
		for i, b := range m.books {
			if b.BookID > 39 {
				continue
			}
			label := b.Name
			if lipgloss.Width(label) > innerW-2 {
				label = label[:innerW-2]
			}
			entries = append(entries, entry{
				label:   label,
				bookIdx: i,
				isCur:   b.BookID == m.currentBook,
				isSel:   i == m.sidebarSelected,
			})
		}
		entries = append(entries, entry{isHeader: true, label: "NEW TESTAMENT"})
		for i, b := range m.books {
			if b.BookID < 40 {
				continue
			}
			label := b.Name
			if lipgloss.Width(label) > innerW-2 {
				label = label[:innerW-2]
			}
			entries = append(entries, entry{
				label:   label,
				bookIdx: i,
				isCur:   b.BookID == m.currentBook,
				isSel:   i == m.sidebarSelected,
			})
		}

		// Virtual scrolling: center on selected index
		selIdx := -1
		for i, e := range entries {
			if e.isSel {
				selIdx = i
				break
			}
		}
		start, end := 0, len(entries)
		if len(entries) > contentLines {
			if selIdx < 0 {
				selIdx = 0
			}
			half := contentLines / 2
			start = selIdx - half
			if start < 0 {
				start = 0
			}
			end = start + contentLines
			if end > len(entries) {
				end = len(entries)
				start = end - contentLines
				if start < 0 {
					start = 0
				}
			}
		}

		if start > 0 {
			sb.WriteString(mutedStyle.Render(fmt.Sprintf("↑ %d more", start)) + "\n")
		}
		for i := start; i < end; i++ {
			e := entries[i]
			if e.isHeader {
				sb.WriteString(sectionStyle.Render(e.label) + "\n")
				continue
			}
			line := "  " + e.label
			isHovered := e.bookIdx == hoveredBookIdx && !e.isSel
			if e.isSel {
				line = "▸ " + e.label
				// pad to innerW to fill the highlight bar
				if lipgloss.Width(line) < innerW {
					line = line + strings.Repeat(" ", innerW-lipgloss.Width(line))
				}
				sb.WriteString(selectedStyle.Render(line) + "\n")
				continue
			}
			if isHovered {
				// Pad to fill the row so the hover bg covers the entire
				// pane width, not just the text.
				if lipgloss.Width(line) < innerW {
					line = line + strings.Repeat(" ", innerW-lipgloss.Width(line))
				}
				sb.WriteString(hoverStyle.Render(line) + "\n")
				continue
			}
			if e.isCur {
				sb.WriteString(currentStyle.Render(line) + "\n")
				continue
			}
			sb.WriteString(normalStyle.Render(line) + "\n")
		}
		if end < len(entries) {
			sb.WriteString(mutedStyle.Render(fmt.Sprintf("↓ %d more", len(entries)-end)))
		}
	}

	// Pad content to exactly fill innerH so the box sizes consistently with
	// the right pane.
	written := sb.String()
	lines := strings.Count(written, "\n") + 1
	if !strings.HasSuffix(written, "\n") {
		// last line already accounted for
	}
	for lines < innerH {
		sb.WriteString("\n")
		lines++
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		BorderBackground(bg).
		Background(bg).
		Width(outerW - 2).
		Padding(1, 1)

	return box.Render(sb.String())
}

func (m Model) renderRightPane(outerW, outerH int) string {
	active := m.focus == paneContent && !m.overlayActive()
	border := m.currentTheme.Border
	if active {
		border = m.currentTheme.BorderActive
	}
	bg := m.currentTheme.Background

	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)

	var titleText string
	switch m.mode {
	case modeComparison:
		titleText = fmt.Sprintf("Comparison · %s %d", m.currentBookName, m.currentChapter)
	default:
		if m.currentBookName == "" {
			titleText = "Reader"
		} else {
			titleText = fmt.Sprintf("%s %d", m.currentBookName, m.currentChapter)
		}
	}

	title := titleStyle.Render(titleText)

	// Single sticky header: the title row itself surfaces the current
	// reading position. When the viewport is scrolled past the top,
	// append "↑ v. N" so the reader always sees where they are. When
	// the user has explicitly selected a verse range but isn't scrolled
	// (j/k inside the visible area), surface the verse range. The two
	// states never duplicate.
	scrolled := m.ready && m.viewport.YOffset() > 0 && m.mode == modeReader
	var locator string
	switch {
	case scrolled && m.highlightedVerseStart > 0:
		locator = mutedStyle.Render(fmt.Sprintf("  ↑ v. %d", m.highlightedVerseStart))
	case !scrolled && m.highlightedVerseStart > 0 && m.highlightedVerseStart != m.highlightedVerseEnd:
		locator = mutedStyle.Render(fmt.Sprintf("  v. %d–%d", m.highlightedVerseStart, m.highlightedVerseEnd))
	case !scrolled && m.highlightedVerseStart > 1:
		// Show the verse only when it's not the obvious "verse 1 at top".
		locator = mutedStyle.Render(fmt.Sprintf("  v. %d", m.highlightedVerseStart))
	}

	// Hover indicator: when the mouse cursor is over a verse in the
	// viewport, surface that verse number with a distinct ⊙ marker.
	// Suppressed when the hovered verse happens to be the same one the
	// locator above is already showing — no point saying it twice.
	if hoveredVerse := m.verseAtMouseY(m.mouseY); hoveredVerse > 0 && hoveredVerse != m.highlightedVerseStart {
		hoverStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
		locator += hoverStyle.Render(fmt.Sprintf("  ⊙ v. %d", hoveredVerse))
	}

	header := title + locator

	body := m.viewport.View()

	innerW := outerW - 2 - 4 // border + padding(1,2)
	spacer := lipgloss.NewStyle().Background(bg).Width(innerW).Render("")

	content := header + "\n" + spacer + "\n" + body

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		BorderBackground(bg).
		Background(bg).
		Width(outerW - 2).
		Height(outerH - 2).
		Padding(1, 2)

	return box.Render(content)
}

// comparisonVerseList returns the verse-number list we want
// loadParallelVerses to fetch. In comparison mode m.currentVerses is
// nil (cleared when parallelVersesLoadedMsg lands), so we derive the
// list from the longest column we already have. Falls back to 1..31
// when nothing's loaded yet (matches the initial "c" trigger).
func (m Model) comparisonVerseList() []int {
	maxV := 0
	if m.currentVerses != nil {
		for _, v := range m.currentVerses {
			if v.Verse > maxV {
				maxV = v.Verse
			}
		}
	}
	if m.currentParallelVerses != nil {
		for _, vs := range m.currentParallelVerses {
			for _, v := range vs {
				if v.Verse > maxV {
					maxV = v.Verse
				}
			}
		}
	}
	if maxV == 0 {
		maxV = 31
	}
	out := make([]int, maxV)
	for i := range out {
		out[i] = i + 1
	}
	return out
}

// comparisonColumnAtX returns the 0-based column index in
// m.comparisonTranslations whose header sits under screen X x, or -1
// if x is outside any column header. Only meaningful in modeComparison.
func (m Model) comparisonColumnAtX(x int) int {
	if m.mode != modeComparison || len(m.comparisonTranslations) == 0 {
		return -1
	}
	// Right pane content area starts at: left pane (30) + right pane
	// left border (1) + left padding (2) = 33.
	contentX := leftPaneOuterWidth + 1 + 2
	n := len(m.comparisonTranslations)
	gaps := n - 1
	colWidth := (m.viewport.Width() - gaps) / n
	if colWidth < 20 {
		colWidth = 20
	}
	relX := x - contentX
	if relX < 0 {
		return -1
	}
	stride := colWidth + 1 // colWidth + 1-cell gutter
	for j := 0; j < n; j++ {
		start := j * stride
		if relX >= start && relX < start+colWidth {
			return j
		}
	}
	return -1
}

// verseAtMouseY returns the verse number the mouse cursor is currently
// over inside the right pane viewport, or 0 if the cursor is somewhere
// else (left pane, chrome, overlay, header/status bar). It mirrors the
// line-counting logic in formatChapter so its mapping is consistent
// with what's actually drawn.
func (m Model) verseAtMouseY(y int) int {
	if m.currentVerses == nil || len(m.currentVerses) == 0 {
		return 0
	}
	if m.mouseX < leftPaneOuterWidth || m.overlayActive() {
		return 0
	}
	// Viewport content starts at: app header (3) + right-pane border (1)
	// + top padding (1) + title row (1) + spacer row (1) = 7.
	viewportTopY := headerOuterHeight + 4
	bottomY := viewportTopY + m.viewport.Height()
	if y < viewportTopY || y >= bottomY {
		return 0
	}
	line := y - viewportTopY + m.viewport.YOffset()
	if line < 0 {
		return 0
	}

	// Same width math as formatChapter (width - 8 from viewport width).
	textWidth := m.viewport.Width() - 8
	if textWidth < 12 {
		textWidth = 12
	}
	indent := 6
	currentLine := 0

	for i, v := range m.currentVerses {
		text := stripHTMLTags(v.Text)
		isHighlighted := m.highlightedVerseStart > 0 && v.Verse >= m.highlightedVerseStart && v.Verse <= m.highlightedVerseEnd
		nextIsHighlighted := false
		if i+1 < len(m.currentVerses) {
			nv := m.currentVerses[i+1]
			nextIsHighlighted = m.highlightedVerseStart > 0 && nv.Verse >= m.highlightedVerseStart && nv.Verse <= m.highlightedVerseEnd
		}

		var verseLines int
		if isHighlighted {
			wrapped := wrapTextWithIndent(text, textWidth-4, indent)
			ln := strings.Count(wrapped, "\n") + 1
			if !nextIsHighlighted {
				// end of range: wrapped content + top + bottom borders + 1 trailing blank
				verseLines = ln + 2 + 1
			} else {
				// in-range verse contributes its lines plus a 1-line spacer
				verseLines = ln + 1
			}
		} else {
			wrapped := wrapTextWithIndent(text, textWidth, indent)
			ln := strings.Count(wrapped, "\n") + 1
			verseLines = ln + 1
		}

		if currentLine+verseLines > line {
			return v.Verse
		}
		currentLine += verseLines
	}
	return 0
}

// overlayPanelBounds returns the (x, y, width, height) of the floating
// overlay panel as it appears on screen. Must match composeOverlay.
func (m Model) overlayPanelBounds() (int, int, int, int) {
	panel := m.renderOverlayPanel()
	if panel == "" {
		return 0, 0, 0, 0
	}
	panelW := lipgloss.Width(panel)
	panelH := lipgloss.Height(panel)
	rightX := leftPaneOuterWidth
	rightW := m.width - rightX
	x := rightX + (rightW-panelW)/2
	y := (m.height-panelH)/2 + 1
	if x < 1 {
		x = 1
	}
	if y < 1 {
		y = 1
	}
	return x, y, panelW, panelH
}

// overlayClick resolves a click at row N (0-indexed from the first
// list item) of the active overlay.
func (m *Model) overlayClick(row int) tea.Cmd {
	if row < 0 {
		return nil
	}
	switch m.mode {
	case modeThemeSelect:
		themes := theme.AllThemes()
		if row < len(themes) {
			m.themeSelected = row
			m.currentTheme = themes[row]
			m.themePinned = true
			m.mode = modeReader
		}
	case modeTranslationSelect:
		if m.translations == nil {
			return nil
		}
		start := m.overlayWindowStart(m.translationSelected, len(m.translations), 16)
		offset := 0
		if start > 0 {
			offset = 1 // the "↑ N more" indicator line
		}
		idx := start + row - offset
		if idx < 0 || idx >= len(m.translations) {
			return nil
		}
		m.translationSelected = idx
		newTrans := m.translations[idx].ShortName
		// Picker scoped to a comparison column: swap that column.
		if m.comparisonPickerColumn >= 0 && m.comparisonPickerColumn < len(m.comparisonTranslations) {
			m.comparisonTranslations[m.comparisonPickerColumn] = newTrans
			m.comparisonPickerColumn = -1
			m.mode = modeComparison
			m.loading = true
			return loadParallelVerses(m.client, m.comparisonTranslations, m.currentBook, m.currentChapter, m.comparisonVerseList())
		}
		if newTrans == m.selectedTranslation {
			m.mode = modeReader
			return nil
		}
		m.selectedTranslation = newTrans
		m.mode = modeReader
		m.loading = true
		return loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
	case modeCacheManager:
		if m.translations == nil {
			return nil
		}
		start := m.overlayWindowStart(m.cacheSelected, len(m.translations), 14)
		offset := 0
		if start > 0 {
			offset = 1
		}
		idx := start + row - offset
		if idx < 0 || idx >= len(m.translations) {
			return nil
		}
		m.cacheSelected = idx
		trans := m.translations[idx].ShortName
		if m.cache != nil && !m.cache.IsCached(trans) && m.downloadingTranslation == "" {
			m.downloadingTranslation = trans
			m.downloadProgress = 0
			return tea.Batch(downloadTranslation(m.cache, trans), downloadTick())
		}
	}
	return nil
}

// overlayNudge moves the selection in the active overlay by delta (±1).
func (m *Model) overlayNudge(delta int) {
	switch m.mode {
	case modeThemeSelect:
		next := m.themeSelected + delta
		max := len(theme.AllThemes()) - 1
		if next < 0 {
			next = 0
		}
		if next > max {
			next = max
		}
		m.themeSelected = next
	case modeTranslationSelect:
		if m.translations == nil {
			return
		}
		next := m.translationSelected + delta
		if next < 0 {
			next = 0
		}
		if next > len(m.translations)-1 {
			next = len(m.translations) - 1
		}
		m.translationSelected = next
	case modeCacheManager:
		if m.translations == nil {
			return
		}
		next := m.cacheSelected + delta
		if next < 0 {
			next = 0
		}
		if next > len(m.translations)-1 {
			next = len(m.translations) - 1
		}
		m.cacheSelected = next
	case modeWordSearch:
		if m.wordSearchResults == nil {
			return
		}
		next := m.wordSearchSelected + delta
		if next < 0 {
			next = 0
		}
		if next > len(m.wordSearchResults)-1 {
			next = len(m.wordSearchResults) - 1
		}
		m.wordSearchSelected = next
	}
}

// overlayWindowStart returns the start index of the visible window for a
// list of n items with the given centered selection and window size.
// Must mirror the windowing math used by the modal renderers.
func (Model) overlayWindowStart(sel, n, window int) int {
	if n <= window {
		return 0
	}
	start := sel - window/2
	if start < 0 {
		start = 0
	}
	end := start + window
	if end > n {
		start = n - window
	}
	return start
}

// bookAtRow returns the book index whose row matches screen y in the
// books pane, or false if y doesn't land on a book.
func (m Model) bookAtRow(y int) (int, bool) {
	if m.books == nil {
		return 0, false
	}

	// Inner content area starts at: header(3) + top border(1) + top padding(1).
	contentY := y - headerOuterHeight - 2
	if contentY < 0 {
		return 0, false
	}

	// Replay the same windowing the renderer uses.
	innerH := m.height - headerOuterHeight - statusOuterHeight - 4 // -2 border -2 padding
	contentLines := innerH - 1 - 2                                // -title -indicators
	if contentLines < 1 {
		contentLines = 1
	}

	type entry struct {
		isHeader bool
		bookIdx  int
	}
	var entries []entry
	entries = append(entries, entry{isHeader: true})
	for i, b := range m.books {
		if b.BookID > 39 {
			continue
		}
		entries = append(entries, entry{bookIdx: i})
	}
	entries = append(entries, entry{isHeader: true})
	for i, b := range m.books {
		if b.BookID < 40 {
			continue
		}
		entries = append(entries, entry{bookIdx: i})
	}

	selIdx := -1
	for i := range entries {
		if !entries[i].isHeader && entries[i].bookIdx == m.sidebarSelected {
			selIdx = i
			break
		}
	}
	start, end := 0, len(entries)
	if len(entries) > contentLines {
		if selIdx < 0 {
			selIdx = 0
		}
		half := contentLines / 2
		start = selIdx - half
		if start < 0 {
			start = 0
		}
		end = start + contentLines
		if end > len(entries) {
			end = len(entries)
			start = end - contentLines
			if start < 0 {
				start = 0
			}
		}
	}

	// Now walk the rendered rows starting at contentY = 0 = "Books" title.
	row := 0
	if contentY == row {
		return 0, false // "Books" title
	}
	row++
	if start > 0 {
		if contentY == row {
			return 0, false // "↑ N more" indicator
		}
		row++
	}
	for i := start; i < end; i++ {
		if contentY == row {
			if entries[i].isHeader {
				return 0, false
			}
			return entries[i].bookIdx, true
		}
		row++
	}
	return 0, false
}

func (m Model) composeOverlay(base string) string {
	panel := m.renderOverlayPanel()
	if panel == "" {
		return base
	}
	panelW := lipgloss.Width(panel)
	panelH := lipgloss.Height(panel)

	// Center over the right pane
	rightX := leftPaneOuterWidth
	rightW := m.width - rightX
	x := rightX + (rightW-panelW)/2
	y := (m.height-panelH)/2 + 1
	if x < 1 {
		x = 1
	}
	if y < 1 {
		y = 1
	}

	shadowStyle := lipgloss.NewStyle().
		Background(m.currentTheme.Shadow).
		Foreground(m.currentTheme.Shadow)
	shadowLine := shadowStyle.Render(strings.Repeat(" ", panelW))
	shadowLines := make([]string, panelH)
	for i := range shadowLines {
		shadowLines[i] = shadowLine
	}
	shadow := strings.Join(shadowLines, "\n")

	compositor := lipgloss.NewCompositor(
		lipgloss.NewLayer(base).X(0).Y(0).Z(0),
		lipgloss.NewLayer(shadow).X(x+2).Y(y+1).Z(1),
		lipgloss.NewLayer(panel).X(x).Y(y).Z(2),
	)
	canvas := lipgloss.NewCanvas(m.width, m.height)
	canvas.Compose(compositor)
	return canvas.Render()
}

func (m Model) renderOverlayPanel() string {
	switch m.mode {
	case modeSearch:
		return m.renderSearchPanel()
	case modeTranslationSelect:
		return m.renderTranslationSelect()
	case modeThemeSelect:
		return m.renderThemeSelect()
	case modeCacheManager:
		return m.renderCacheManager()
	case modeAbout:
		return m.renderAbout()
	case modeWordSearch:
		return m.renderWordSearch()
	}
	return ""
}

func (m Model) renderSearchPanel() string {
	bg := m.currentTheme.Background
	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)

	// Size from the available right-pane area.
	maxAvail := m.width - leftPaneOuterWidth - 8
	width := maxAvail
	if width > 64 {
		width = 64
	}
	if width < 40 {
		width = 40
	}
	innerW := width - 6

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(width).
		Padding(1, 2)

	// Apply the theme's background to the bubbles textinput so it doesn't
	// punch a terminal-default-colored hole through the panel.
	ti := m.textInput
	ti.SetStyles(m.themedInputStyles())
	ti.SetWidth(innerW - 2)

	body := titleStyle.Render("Go to verse") + "\n\n" +
		ti.View() + "\n\n" +
		hintStyle.Render("e.g. \"John 3:16\" or \"1 1:1\"")

	return box.Render(body)
}

// themedInputStyles returns textinput Styles that paint every cell of the
// rendered input (prompt, text, placeholder, cursor backdrop) with the
// current theme's background so the input blends into its panel.
func (m Model) themedInputStyles() textinput.Styles {
	bg := m.currentTheme.Background
	primary := lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Background(bg)
	muted := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)
	prompt := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)

	state := textinput.StyleState{
		Text:        primary,
		Placeholder: muted,
		Suggestion:  muted,
		Prompt:      prompt,
	}
	return textinput.Styles{
		Focused: state,
		Blurred: state,
		Cursor: textinput.CursorStyle{
			Color: m.currentTheme.Accent,
			Shape: tea.CursorBlock,
			Blink: true,
		},
	}
}

func (m Model) calculateHighlightedVerse() int {
	if m.currentVerses == nil || len(m.currentVerses) == 0 {
		return 1
	}

	// Calculate which verse is at the top of the viewport
	yOffset := m.viewport.YOffset()

	// Calculate text width the same way formatChapter does. We use the
	// viewport's width (not m.width — that's the full terminal) and
	// match formatChapter's `width - 8` for the textWidth.
	textWidth := m.viewport.Width() - 8
	if textWidth < 12 {
		textWidth = 12
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
				// End of highlighted range: wrapped content + 2 border
				// lines (top + bottom) + 1 trailing blank.
				verseTotalLines = linesInVerse + 2 + 1
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
			maxOffset := totalLines - m.viewport.Height()
			if maxOffset < 0 {
				maxOffset = 0
			}

			if targetOffset > maxOffset {
				targetOffset = maxOffset
			}

			m.viewport.SetYOffset(targetOffset)
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
		Width(columnWidth - 2)

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

	moreStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Muted).
		Italic(true).
		Padding(0, 1)

	var sb strings.Builder

	if m.books != nil {
		// Calculate available height for book list
		// Account for border (2), padding (2), and header lines
		availableHeight := m.height - 8

		// Build a combined list with section headers
		type bookEntry struct {
			isHeader   bool
			headerText string
			bookIndex  int
			book       api.Book
		}

		var entries []bookEntry

		// Old Testament header
		entries = append(entries, bookEntry{isHeader: true, headerText: "OLD TESTAMENT"})

		// Old Testament books
		for i, book := range m.books {
			if book.BookID > 39 {
				break
			}
			entries = append(entries, bookEntry{isHeader: false, bookIndex: i, book: book})
		}

		// New Testament header (with spacing)
		entries = append(entries, bookEntry{isHeader: true, headerText: ""}) // blank line
		entries = append(entries, bookEntry{isHeader: true, headerText: "NEW TESTAMENT"})

		// New Testament books
		for i, book := range m.books {
			if book.BookID < 40 {
				continue
			}
			entries = append(entries, bookEntry{isHeader: false, bookIndex: i, book: book})
		}

		// Find the entry index for the selected book
		selectedEntryIdx := 0
		for i, entry := range entries {
			if !entry.isHeader && entry.bookIndex == m.sidebarSelected {
				selectedEntryIdx = i
				break
			}
		}

		// Calculate virtual scroll window
		totalEntries := len(entries)
		visibleCount := availableHeight
		if visibleCount < 5 {
			visibleCount = 5
		}
		if visibleCount > totalEntries {
			visibleCount = totalEntries
		}

		// Center the selected item in the visible window
		startIdx := selectedEntryIdx - visibleCount/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := startIdx + visibleCount
		if endIdx > totalEntries {
			endIdx = totalEntries
			startIdx = endIdx - visibleCount
			if startIdx < 0 {
				startIdx = 0
			}
		}

		// Show "more above" indicator
		if startIdx > 0 {
			sb.WriteString(moreStyle.Render(fmt.Sprintf("... (%d more above)", startIdx)) + "\n")
		}

		// Render visible entries
		for i := startIdx; i < endIdx; i++ {
			entry := entries[i]
			if entry.isHeader {
				if entry.headerText == "" {
					sb.WriteString("\n")
				} else {
					sb.WriteString(sectionHeaderStyle.Render(entry.headerText) + "\n")
				}
			} else {
				if entry.bookIndex == m.sidebarSelected {
					sb.WriteString(selectedStyle.Render("> "+entry.book.Name) + "\n")
				} else {
					sb.WriteString(normalStyle.Render("  "+entry.book.Name) + "\n")
				}
			}
		}

		// Show "more below" indicator
		if endIdx < totalEntries {
			sb.WriteString(moreStyle.Render(fmt.Sprintf("... (%d more below)", totalEntries-endIdx)) + "\n")
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

func (m Model) renderTranslationSelect() string {
	bg := m.currentTheme.Background

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(56).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)

	selectedStyle := lipgloss.NewStyle().
		Foreground(bg).
		Background(m.currentTheme.Accent).
		Bold(true).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(bg).
		Padding(0, 1)

	currentStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Background(bg).
		Padding(0, 1)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Select Translation") + "\n\n")

	if m.translations != nil {
		// Show at most 16 translations centered on selection.
		const window = 16
		start, end := 0, len(m.translations)
		if len(m.translations) > window {
			start = m.translationSelected - window/2
			if start < 0 {
				start = 0
			}
			end = start + window
			if end > len(m.translations) {
				end = len(m.translations)
				start = end - window
			}
		}
		mutedStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)
		if start > 0 {
			content.WriteString(mutedStyle.Render(fmt.Sprintf("  ↑ %d more\n", start)))
		}
		for i := start; i < end; i++ {
			trans := m.translations[i]
			prefix := "  "
			style := normalStyle
			suffix := ""
			isCurrent := trans.ShortName == m.selectedTranslation
			if i == m.translationSelected {
				prefix = "▸ "
				style = selectedStyle
			} else if isCurrent {
				style = currentStyle
			}
			name := fmt.Sprintf("%-6s · %s", trans.ShortName, trans.FullName)
			if isCurrent && i != m.translationSelected {
				suffix = "  ●"
			}
			content.WriteString(style.Render(prefix+name+suffix) + "\n")
		}
		if end < len(m.translations) {
			content.WriteString(mutedStyle.Render(fmt.Sprintf("  ↓ %d more", len(m.translations)-end)))
		}
	} else {
		content.WriteString(normalStyle.Render("  Loading translations..."))
	}

	return containerStyle.Render(content.String())
}

func (m Model) renderCacheManager() string {
	bg := m.currentTheme.Background

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(56).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)

	selectedStyle := lipgloss.NewStyle().
		Foreground(bg).
		Background(m.currentTheme.Accent).
		Bold(true).
		Padding(0, 1)
	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(bg).
		Padding(0, 1)
	cachedStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Background(bg).
		Padding(0, 1)
	downloadingStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Warning).
		Background(bg).
		Padding(0, 1)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Download Translations") + "\n\n")

	if m.translations != nil {
		const window = 14
		start, end := 0, len(m.translations)
		if len(m.translations) > window {
			start = m.cacheSelected - window/2
			if start < 0 {
				start = 0
			}
			end = start + window
			if end > len(m.translations) {
				end = len(m.translations)
				start = end - window
			}
		}
		if start > 0 {
			content.WriteString(mutedStyle.Render(fmt.Sprintf("  ↑ %d more\n", start)))
		}
		for i := start; i < end; i++ {
			trans := m.translations[i]
			prefix := "  "
			style := normalStyle
			suffix := ""
			isCached := m.cache != nil && m.cache.IsCached(trans.ShortName)
			isDownloading := m.downloadingTranslation == trans.ShortName
			if i == m.cacheSelected {
				prefix = "▸ "
				style = selectedStyle
			}
			name := fmt.Sprintf("%-6s · %s", trans.ShortName, trans.FullName)
			if isDownloading {
				suffix = "  ⟳ downloading"
				if i != m.cacheSelected {
					style = downloadingStyle
				}
			} else if isCached {
				suffix = "  ✓"
				if i != m.cacheSelected {
					style = cachedStyle
				}
			}
			content.WriteString(style.Render(prefix+name+suffix) + "\n")
		}
		if end < len(m.translations) {
			content.WriteString(mutedStyle.Render(fmt.Sprintf("  ↓ %d more", len(m.translations)-end)))
		}
	} else {
		content.WriteString(normalStyle.Render("  Loading translations..."))
	}

	// Live download progress bar, rendered just inside the panel below
	// the list when a download is running. The bubbles/v2/progress bar
	// is asked for a static view via ViewAs(p) so it doesn't animate
	// independently of our poll-driven m.downloadProgress.
	if m.downloadingTranslation != "" {
		bar := m.progressBar
		bar.SetWidth(48)
		content.WriteString("\n\n" + mutedStyle.Render(fmt.Sprintf("Downloading %s", m.downloadingTranslation)) + "\n")
		content.WriteString(bar.ViewAs(m.downloadProgress))
	}

	if m.cache != nil {
		if size, err := m.cache.GetCacheSize(); err == nil && size > 0 {
			content.WriteString("\n\n" + mutedStyle.Render(fmt.Sprintf("Cache: %.2f MB", float64(size)/(1024*1024))))
		}
	}

	return containerStyle.Render(content.String())
}

func (m Model) renderThemeSelect() string {
	// The picker uses the CURRENTLY APPLIED theme for its own chrome
	// (title, list, container border) so the picker keeps a stable look
	// while the user is arrowing through options. The PREVIEW pane uses
	// the FOCUSED theme (themes[themeSelected]) so the user can see what
	// they'd be committing to without pressing Enter.
	chromeBg := m.currentTheme.Background

	themes := theme.AllThemes()
	listWidth := 24
	previewWidth := 48
	innerW := listWidth + 1 + previewWidth // 1-cell gutter between the two halves

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(chromeBg).
		Background(chromeBg).
		Width(innerW + 4). // +2 padding +2 border
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(chromeBg).
		Bold(true)

	// --- Left column: theme list ---
	listSelectedStyle := lipgloss.NewStyle().
		Foreground(chromeBg).
		Background(m.currentTheme.Accent).
		Bold(true)
	listNormalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(chromeBg)
	listCurrentStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Background(chromeBg)

	padRow := func(s string) string {
		w := lipgloss.Width(s)
		if w >= listWidth {
			return s
		}
		return s + lipgloss.NewStyle().Background(chromeBg).Render(strings.Repeat(" ", listWidth-w))
	}

	// Build the list as a slice of pre-padded rendered rows so we can
	// row-pair manually with the preview without relying on lipgloss's
	// JoinHorizontal (which has been mis-pairing rows here when the
	// preview contains multi-row primitives like the highlight box).
	var listRows []string
	listRows = append(listRows, listNormalStyle.Render(padRow(titleStyle.Render("Select Theme"))))
	listRows = append(listRows, listNormalStyle.Render(padRow("")))
	for i, thm := range themes {
		label := thm.Name
		if lipgloss.Width(label) > listWidth-4 {
			label = label[:listWidth-4]
		}
		isCurrent := thm.Name == m.currentTheme.Name
		isFocused := i == m.themeSelected
		var row string
		switch {
		case isFocused:
			text := "▸ " + label
			if isCurrent {
				text += " ●"
			}
			row = listSelectedStyle.Render(padRow(text))
		case isCurrent:
			row = listCurrentStyle.Render(padRow("  " + label + " ●"))
		default:
			row = listNormalStyle.Render(padRow("  " + label))
		}
		listRows = append(listRows, row)
	}

	// --- Right column: live preview using the focused theme ---
	focused := themes[m.themeSelected]
	previewRows := strings.Split(m.themePreview(focused, previewWidth), "\n")
	// Drop a trailing empty row that strings.Split produces when the
	// preview ends with a newline.
	if len(previewRows) > 0 && previewRows[len(previewRows)-1] == "" {
		previewRows = previewRows[:len(previewRows)-1]
	}

	// Equalize heights with bg-styled blank rows.
	listBlank := listNormalStyle.Render(padRow(""))
	previewBlank := lipgloss.NewStyle().Background(focused.Background).Width(previewWidth).Render("")
	for len(listRows) < len(previewRows) {
		listRows = append(listRows, listBlank)
	}
	for len(previewRows) < len(listRows) {
		previewRows = append(previewRows, previewBlank)
	}

	// Single-cell gutter in chrome bg between list and preview.
	gutter := lipgloss.NewStyle().Background(chromeBg).Render(" ")

	var body strings.Builder
	for i := range listRows {
		body.WriteString(listRows[i])
		body.WriteString(gutter)
		body.WriteString(previewRows[i])
		if i < len(listRows)-1 {
			body.WriteByte('\n')
		}
	}

	return containerStyle.Render(body.String())
}

// themePreview renders a small reader-style sample in the given theme,
// wrapped in its own rounded card so it reads as a self-contained
// preview of how the reader would look in this theme. The card uses
// the focused theme for border, background, and every text style.
func (m Model) themePreview(th theme.Theme, w int) string {
	bg := th.Background
	hbg := th.Highlight

	// The card is the full preview block, w cells wide. Its inner
	// content area is w - 2 (border) - 4 (padding 1,2) = w - 6.
	inner := w - 6
	if inner < 14 {
		inner = 14
	}

	titleStyle := lipgloss.NewStyle().Foreground(th.Accent).Background(bg).Bold(true)
	bookStyle := lipgloss.NewStyle().Foreground(th.Success).Background(bg).Bold(true)
	verseNumStyle := lipgloss.NewStyle().Foreground(th.Warning).Background(bg).Bold(true).Width(4).Align(lipgloss.Right)
	textStyle := lipgloss.NewStyle().Foreground(th.Primary).Background(bg)
	mutedStyle := lipgloss.NewStyle().Foreground(th.Muted).Background(bg).Italic(true)

	hlVerseNumStyle := lipgloss.NewStyle().Foreground(th.Accent).Background(hbg).Bold(true).Width(4).Align(lipgloss.Right)
	hlTextStyle := lipgloss.NewStyle().Foreground(th.Primary).Background(hbg).Bold(true)
	hlBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.BorderActive).
		BorderBackground(bg).
		Background(hbg).
		Padding(0, 1)

	// Pad each emitted line out to inner so the entire card interior is
	// covered with bg cells — no chrome bleed-through.
	pad := func(s string) string {
		v := lipgloss.Width(s)
		if v >= inner {
			return s
		}
		return s + lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", inner-v))
	}
	blank := lipgloss.NewStyle().Background(bg).Width(inner).Render("")

	// Sample text widths. textWidth is the visible width of the verse
	// text. Highlighted box outer = textWidth + 6, so we pull in by 2
	// extra cells so it never matches inner (lipgloss wraps on exact
	// width).
	textWidth := inner - 8
	if textWidth < 12 {
		textWidth = 12
	}
	clip := func(s string) string {
		if len(s) > textWidth {
			s = s[:textWidth-1] + "…"
		}
		return s
	}

	var body strings.Builder
	body.WriteString(pad(titleStyle.Render(th.Name)) + "\n")
	body.WriteString(pad(mutedStyle.Render("preview")) + "\n")
	body.WriteString(blank + "\n")
	body.WriteString(pad(bookStyle.Render("John 3")) + "\n")
	body.WriteString(blank + "\n")

	sep := lipgloss.NewStyle().Background(bg).Render("  ")

	v14 := textStyle.Width(textWidth).Render(clip("Moses lifted up the serpent…"))
	body.WriteString(pad(verseNumStyle.Render("14")+sep+v14) + "\n")
	body.WriteString(blank + "\n")

	hsep := lipgloss.NewStyle().Background(hbg).Render("  ")
	hlText := hlTextStyle.Width(textWidth - 4).Render(clip("For God so loved the world…"))
	hlInner := hlVerseNumStyle.Render("16") + hsep + hlText
	hl := hlBoxStyle.Render(hlInner)
	for _, ln := range strings.Split(strings.TrimRight(hl, "\n"), "\n") {
		body.WriteString(pad(ln) + "\n")
	}
	body.WriteString(blank + "\n")

	v17 := textStyle.Width(textWidth).Render(clip("God did not send his Son to condemn…"))
	body.WriteString(pad(verseNumStyle.Render("17")+sep+v17))

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(w - 2).
		Padding(1, 2).
		Render(body.String())

	return card
}

func (m Model) formatChapter(verses []api.Verse, bookName string, chapter int, width int, highlightedVerseStart, highlightedVerseEnd int) string {
	bg := m.currentTheme.Background
	hbg := m.currentTheme.Highlight

	verseStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Warning).
		Background(bg).
		Bold(true).
		Width(4).
		Align(lipgloss.Right)

	highlightedVerseStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(hbg).
		Bold(true).
		Width(4).
		Align(lipgloss.Right)

	textStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(bg)

	highlightedTextStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(hbg).
		Bold(true)

	highlightedContainerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(hbg).
		Padding(0, 1)

	// Styled separators so the gap between verse number and text gets the
	// pane background, and the trailing blank line between verses does too.
	sep := lipgloss.NewStyle().Background(bg).Render("  ")
	hsep := lipgloss.NewStyle().Background(hbg).Render("  ")
	blankLine := lipgloss.NewStyle().Background(bg).Width(width).Render("")
	bgPadStyle := lipgloss.NewStyle().Background(bg)
	padToWidth := func(line string) string {
		w := lipgloss.Width(line)
		if w >= width {
			return line
		}
		return line + bgPadStyle.Render(strings.Repeat(" ", width-w))
	}

	var sb strings.Builder

	// Calculate available width for text. Verse number is right-aligned
	// in 4 chars + 2 spaces = 6 chars total. We leave an extra 2 cells of
	// safety so the highlighted-verse rounded box (which costs 6 cells of
	// border+padding around the inner text) doesn't equal viewport width
	// exactly (lipgloss wraps on exact-width matches).
	textWidth := width - 8
	if textWidth < 20 {
		textWidth = 20 // Minimum width for readability
	}
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

			highlightedContent.WriteString(verseNum + hsep + verseText)

			// If next verse is also highlighted, add spacing within the border
			if nextIsHighlighted {
				highlightedContent.WriteString("\n\n")
			} else {
				// End of highlighted range - render the border, then pad
				// each rendered row out to width so the right edge meets
				// the pane background instead of the terminal default.
				borderedVerse := highlightedContainerStyle.Render(highlightedContent.String())
				for _, ln := range strings.Split(borderedVerse, "\n") {
					sb.WriteString(padToWidth(ln) + "\n")
				}
				sb.WriteString(blankLine + "\n")
				inHighlightedRange = false
			}
		} else {
			verseNum := verseStyle.Render(verseNumStr)

			// Calculate indent for wrapped lines (verse number width + 2 spaces)
			indent := 6
			wrappedText := wrapTextWithIndent(text, textWidth, indent)
			verseText := textStyle.Width(textWidth).Render(wrappedText)

			// Each wrapped line of the verse is verseNum (4) + sep (2) +
			// verseText (textWidth). The continuation lines already carry
			// their leading indent inside wrappedText (from wrapTextWithIndent),
			// so we only prepend the verse-number block on the first line.
			// padToWidth then fills the right edge with bg for every row.
			textLines := strings.Split(verseText, "\n")
			for idx, ln := range textLines {
				if idx == 0 {
					sb.WriteString(padToWidth(verseNum+sep+ln) + "\n")
				} else {
					sb.WriteString(padToWidth(ln) + "\n")
				}
			}
			sb.WriteString(blankLine + "\n")
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
	if len(translations) == 0 {
		return ""
	}

	bg := m.currentTheme.Background

	headerStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(bg).
		Bold(true)
	verseNumStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Warning).
		Background(bg).
		Bold(true)
	textStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(bg)
	separatorStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Border).
		Background(bg)
	bgPad := lipgloss.NewStyle().Background(bg)

	// Column geometry: split the available width across N translations,
	// leaving a 1-cell gutter between each pair.
	n := len(translations)
	gaps := n - 1
	colWidth := (width - gaps) / n
	if colWidth < 20 {
		colWidth = 20
	}
	// Inner text width allows for a "NN " verse number prefix (4 cells).
	textWidth := colWidth - 4
	if textWidth < 12 {
		textWidth = 12
	}

	// padCol pads a logical column line to colWidth using bg-styled
	// spaces so the rows merge into a clean grid regardless of theme.
	padCol := func(s string) string {
		w := lipgloss.Width(s)
		if w >= colWidth {
			return s
		}
		return s + bgPad.Render(strings.Repeat(" ", colWidth-w))
	}

	// maxVerses across all translations.
	maxVerses := 0
	for _, vs := range versesMap {
		if len(vs) > maxVerses {
			maxVerses = len(vs)
		}
	}

	// Build the header row: one column per translation, padded to colWidth.
	// "▾" hints that the header opens a translation picker on click.
	headerCells := make([]string, n)
	for j, trans := range translations {
		label := trans + " ▾"
		if lipgloss.Width(label) > colWidth {
			label = label[:colWidth]
		}
		headerCells[j] = padCol(headerStyle.Render(label))
	}
	gutter := bgPad.Render(" ")
	header := strings.Join(headerCells, gutter)

	// Separator row under the header.
	sepRow := padCol(separatorStyle.Render(strings.Repeat("─", colWidth)))
	separator := strings.Join(repeatString(sepRow, n), gutter)

	// For each verse number, build a row of columns and JoinHorizontal
	// them. JoinHorizontal automatically pads shorter columns at the
	// bottom so all rows in a verse stay aligned.
	var rows []string
	rows = append(rows, header, separator)

	for i := 1; i <= maxVerses; i++ {
		cells := make([]string, n)
		for j, trans := range translations {
			verses := versesMap[trans]
			var text string
			for _, v := range verses {
				if v.Verse == i {
					text = stripHTMLTags(v.Text)
					break
				}
			}
			if text == "" {
				cells[j] = padCol("")
				continue
			}
			// First line: "N  text…", continuation lines indent under
			// the text so the verse number stays as a visual anchor.
			wrapped := wrapTextWithIndent(text, textWidth, 4)
			lines := strings.Split(wrapped, "\n")
			styled := make([]string, len(lines))
			for k, ln := range lines {
				if k == 0 {
					styled[k] = padCol(verseNumStyle.Render(fmt.Sprintf("%-3d", i)) + bgPad.Render(" ") + textStyle.Render(ln))
				} else {
					styled[k] = padCol(textStyle.Render(ln))
				}
			}
			cells[j] = strings.Join(styled, "\n")
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, intersperse(cells, gutter)...))
		// Blank row between verses, styled in bg so it covers the full
		// width and the grid stays painted.
		blankRow := padCol("")
		rows = append(rows, strings.Join(repeatString(blankRow, n), gutter))
	}

	return strings.Join(rows, "\n")
}

func repeatString(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

// intersperse returns ss with sep placed between each consecutive pair.
// "a","b","c" + "|" → "a","|","b","|","c"
func intersperse(ss []string, sep string) []string {
	if len(ss) == 0 {
		return ss
	}
	out := make([]string, 0, 2*len(ss)-1)
	for i, s := range ss {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, s)
	}
	return out
}

func stripHTMLTags(s string) string {
	// Strip HTML tags. The bolls.life API wraps the matched search term
	// in <em>…</em> *inside* words (e.g. "lov<em>e</em>d"), so replacing
	// tags with a space would split such words. Drop them outright and
	// collapse any resulting double spaces at the end.
	re := regexp.MustCompile(`<[^>]*>`)
	s = re.ReplaceAllString(s, "")

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

// parseReference accepts a wide variety of reference formats:
//   - "John 3:16"       canonical
//   - "john3:16"        no spaces
//   - "john 3 16"       spaces instead of colon
//   - "john 3:16-17"    range
//   - "john 3 16-17"    range with spaces
//   - "1 John 3:16"     book name starting with a digit
//   - "1john3:16"       no spaces, book starts with digit
//   - "1 1:1"           book by numeric id + chapter:verse
//   - "gen 1"           book + chapter only
//   - "gen"             book only (defaults to chapter 1)
//
// The split into book and the rest happens at the boundary between the
// (optional digit-prefixed) word that names the book and the numeric
// chapter/verse data that follows. Then any digit-runs in the rest are
// pulled out in order as chapter / verse-start / verse-end, with a
// hyphen interpreted as the range separator.
func parseReference(ref string, books []api.Book) (book, chapter, verseStart, verseEnd int, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 0, 0, 0, 0, fmt.Errorf("empty reference")
	}

	// Book identifier alternatives, in order of specificity. Each
	// letter run may be followed by ` letter-run` repeats so multi-word
	// book names like "Song of Solomon" or "1 Samuel" stay intact.
	//   1. digit + optional whitespace + letters (+ more words)  →  "1 John", "1john", "1 Samuel"
	//   2. letters (+ more words)                                 →  "John", "rom", "Song of Solomon"
	//   3. digit                                                  →  "1" (book id)
	bookRe := regexp.MustCompile(`(?i)^(\d+\s*[a-z]+(?:\s+[a-z]+)*|[a-z]+(?:\s+[a-z]+)*|\d+)\s*(.*)$`)
	m := bookRe.FindStringSubmatch(ref)
	if m == nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid reference: %s", ref)
	}
	bookPart := strings.TrimSpace(m[1])
	rest := m[2]

	// Resolve the book identifier.
	if id, perr := strconv.Atoi(bookPart); perr == nil {
		book = id
	} else if len(books) > 0 {
		id, _, found := fuzzyMatchBook(bookPart, books)
		if !found {
			return 0, 0, 0, 0, fmt.Errorf("book not found: %s", bookPart)
		}
		book = id
	} else {
		return 0, 0, 0, 0, fmt.Errorf("no books loaded")
	}

	// Default chapter when none was supplied.
	chapter = 1

	if rest == "" {
		return book, chapter, 0, 0, nil
	}

	// Split into before/after a hyphen so verse-range parsing is robust
	// even with arbitrary separators around it ("3:16-17", "3 16 - 17",
	// "3 16-17").
	beforeDash, afterDash := rest, ""
	if i := strings.IndexByte(rest, '-'); i >= 0 {
		beforeDash = rest[:i]
		afterDash = rest[i+1:]
	}

	numRe := regexp.MustCompile(`\d+`)
	beforeNums := numRe.FindAllString(beforeDash, -1)
	afterNums := numRe.FindAllString(afterDash, -1)

	if len(beforeNums) >= 1 {
		chapter, _ = strconv.Atoi(beforeNums[0])
	}
	if len(beforeNums) >= 2 {
		verseStart, _ = strconv.Atoi(beforeNums[1])
		verseEnd = verseStart
	}
	if len(afterNums) >= 1 {
		verseEnd, _ = strconv.Atoi(afterNums[0])
		if verseStart == 0 && len(beforeNums) >= 2 {
			verseStart, _ = strconv.Atoi(beforeNums[1])
		}
	}

	return book, chapter, verseStart, verseEnd, nil
}

func (m Model) renderAbout() string {
	bg := m.currentTheme.Background

	width := 64
	if m.width-leftPaneOuterWidth-6 < width {
		width = m.width - leftPaneOuterWidth - 6
		if width < 40 {
			width = 40
		}
	}

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(width).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Background(bg)
	labelStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Secondary).Background(bg).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Background(bg)
	linkStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Underline(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render("sword-tui") + "\n")
	content.WriteString(sectionStyle.Render("A terminal-based Bible application") + "\n\n")

	content.WriteString(labelStyle.Render("Version: ") + valueStyle.Render(version.Version) + "\n")
	content.WriteString(labelStyle.Render("Build:   ") + valueStyle.Render(version.BuildNumber) + "\n\n")

	content.WriteString(labelStyle.Render("Repo:    ") + linkStyle.Render("github.com/kmf/sword-tui") + "\n")
	content.WriteString(labelStyle.Render("API:     ") + valueStyle.Render("bolls.life") + "\n")
	content.WriteString(labelStyle.Render("License: ") + valueStyle.Render("GPL-2.0-or-later") + "\n\n")

	content.WriteString(titleStyle.Render("Shortcuts") + "\n\n")
	shortcuts := []struct{ key, desc string }{
		{"tab", "switch focused pane"},
		{"⏎", "open book / submit"},
		{"n / p", "next / prev chapter"},
		{"/", "go to verse"},
		{"s", "search Bible"},
		{"c", "compare translations"},
		{"t", "select translation"},
		{"T", "select theme"},
		{"d", "download translations"},
		{"y", "yank current verse"},
		{"?", "about"},
		{"q", "quit"},
	}
	for _, s := range shortcuts {
		content.WriteString(labelStyle.Render(fmt.Sprintf("%-8s", s.key)) + sectionStyle.Render(s.desc) + "\n")
	}

	return containerStyle.Render(content.String())
}

func (m Model) renderWordSearch() string {
	bg := m.currentTheme.Background

	// Panel sizes from the available right-pane area, capped at 100 cells
	// and floored at 40 so it stays usable on narrow terminals.
	maxAvail := m.width - leftPaneOuterWidth - 8
	width := maxAvail
	if width > 100 {
		width = 100
	}
	if width < 40 {
		width = 40
	}
	// Inner content width: panel - border(2) - padding(2*2)
	innerW := width - 6

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(width).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)

	selectedStyle := lipgloss.NewStyle().
		Foreground(bg).
		Background(m.currentTheme.Accent).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(bg)

	bookNameStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(bg).
		Bold(true)

	mutedStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Search Bible") + "\n\n")

	if m.wordSearchResults == nil && !m.wordSearchLoading {
		ti := m.wordSearchInput
		ti.SetStyles(m.themedInputStyles())
		ti.SetWidth(innerW - 2) // leave a couple of cells of breathing room
		content.WriteString(ti.View() + "\n\n")
		content.WriteString(mutedStyle.Render("Type a word or phrase, then ⏎"))
	} else if m.wordSearchLoading {
		content.WriteString(mutedStyle.Render("Searching…"))
	} else if len(m.wordSearchResults) == 0 {
		content.WriteString(normalStyle.Render(fmt.Sprintf("No results for \"%s\"", m.wordSearchQuery)) + "\n\n")
		content.WriteString(mutedStyle.Render("esc to close"))
	} else {
		content.WriteString(mutedStyle.Render(fmt.Sprintf("%d results for \"%s\" — showing %d",
			m.wordSearchTotal, m.wordSearchQuery, len(m.wordSearchResults))) + "\n\n")

		// Row-based virtual scrolling: each result may wrap to multiple
		// lines, so we budget by rendered row count instead of by item.
		// Total panel rows we can afford = m.height - top header bar (3)
		// - status bar (3) - 2 cells of breathing room. Inside the panel
		// we lose: border (2) + padding (2) + title (1) + blank (1) +
		// "X results for…" (1) + blank (1) + the "↓ N more" trailer (1)
		// = 9 rows of chrome. Anything left is for the wrapped items.
		availRows := m.height - 3 - 3 - 2 - 9
		if availRows < 4 {
			availRows = 4
		}

		// Pre-render each result with wrapping so we know its row cost.
		type item struct {
			lines   []string
			isBook  bool
			isSel   bool
			origIdx int
		}
		var items []item

		bodyPrefixW := 2 // "▸ " or "  "
		refTemplate := "999:999 "
		textWidth := innerW - bodyPrefixW - len(refTemplate)
		if textWidth < 20 {
			textWidth = 20
		}

		currentBook := -1
		for i, result := range m.wordSearchResults {
			if result.Book != currentBook {
				currentBook = result.Book
				bookName := m.getBookName(result.Book)
				items = append(items, item{
					lines:  []string{"", bookNameStyle.Render(bookName)},
					isBook: true,
				})
			}
			ref := fmt.Sprintf("%-7s", fmt.Sprintf("%d:%d", result.Chapter, result.Verse))
			verseText := stripHTMLTags(result.Text)
			wrapped := wrapTextWithIndent(verseText, textWidth, 2+len(refTemplate))
			wrappedLines := strings.Split(wrapped, "\n")

			lines := make([]string, len(wrappedLines))
			for j, ln := range wrappedLines {
				var row string
				if j == 0 {
					row = ref + " " + ln
				} else {
					row = ln // continuation already has indent inside wrappedText
				}
				lines[j] = row
			}
			items = append(items, item{
				lines:   lines,
				isSel:   i == m.wordSearchSelected,
				origIdx: i,
			})
		}

		// Find the index in `items` of the selected result so we can
		// center the window around it.
		selItemIdx := -1
		for i, it := range items {
			if !it.isBook && it.origIdx == m.wordSearchSelected {
				selItemIdx = i
				break
			}
		}

		// Walk items expanding rows around the selection until we fill
		// availRows. Symmetric expansion keeps the cursor roughly centered.
		startIdx, endIdx := selItemIdx, selItemIdx+1
		if selItemIdx < 0 {
			startIdx, endIdx = 0, 0
		}
		usedRows := 0
		if selItemIdx >= 0 {
			usedRows = len(items[selItemIdx].lines)
		}
		for usedRows < availRows && (startIdx > 0 || endIdx < len(items)) {
			// Prefer expanding downward first to mimic typical list scroll.
			if endIdx < len(items) && usedRows+len(items[endIdx].lines) <= availRows {
				usedRows += len(items[endIdx].lines)
				endIdx++
			} else if startIdx > 0 && usedRows+len(items[startIdx-1].lines) <= availRows {
				startIdx--
				usedRows += len(items[startIdx].lines)
			} else {
				break
			}
		}
		if startIdx < 0 {
			startIdx = 0
		}
		if endIdx > len(items) {
			endIdx = len(items)
		}

		// Count above/below results (excluding book headers).
		var above, below int
		for i := 0; i < startIdx; i++ {
			if !items[i].isBook {
				above++
			}
		}
		for i := endIdx; i < len(items); i++ {
			if !items[i].isBook {
				below++
			}
		}

		if above > 0 {
			content.WriteString(mutedStyle.Render(fmt.Sprintf("  ↑ %d more", above)) + "\n")
		}

		for i := startIdx; i < endIdx; i++ {
			it := items[i]
			if it.isBook {
				for _, ln := range it.lines {
					content.WriteString(ln + "\n")
				}
				continue
			}
			for j, ln := range it.lines {
				var styled string
				if it.isSel {
					if j == 0 {
						styled = selectedStyle.Render("▸ " + ln)
					} else {
						styled = selectedStyle.Render("  " + ln)
					}
				} else {
					if j == 0 {
						styled = normalStyle.Render("  " + ln)
					} else {
						styled = normalStyle.Render("  " + ln)
					}
				}
				content.WriteString(styled + "\n")
			}
		}

		if below > 0 {
			content.WriteString(mutedStyle.Render(fmt.Sprintf("  ↓ %d more", below)))
		}
	}

	return containerStyle.Render(content.String())
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

