# TODO

Feature backlog for sword-tui, anchored against the v2 charm stack
(bubbletea v2.0.7, bubbles v2.1.0, lipgloss v2.0.3).

Roughly ordered by impact / effort. Each entry names the underlying v2
capability so we can verify the design lines up with what the library
actually provides.

## High-impact

- [ ] **Cross-reference hyperlinks** (lipgloss v2 hyperlinks)
  Render verse refs like `John 3:16` as ANSI hyperlinks. Click in
  supported terminals (Ghostty/iTerm2/WezTerm) jumps to the verse.
  Falls back to plain text elsewhere.

- [ ] **Declarative `View()` with cursor control** (bubbletea v2 View struct)
  Move from `(string)` to the `tea.View{Layer, Cursor}` shape so we can
  show the actual terminal cursor in word-search / verse-search inputs
  instead of the bubbletea-rendered block.

- [x] **Side-by-side comparison view**
  `formatParallelVerses` now renders N translations as columns:
  header row with translation labels, ─ separator under each, then
  one row per verse with `JoinHorizontal` pairing the columns by
  verse number. Column width = `(viewport.width - gutters) / N`,
  text wraps inside each column with a 4-cell verse-number indent,
  and rows are bg-painted to the column width so the grid stays
  clean on any theme. Hover popovers for cross-references not yet
  shipped (no cross-reference data source plumbed in).

- [ ] **Multi-line word-search input with auto-grow** (bubbles v2 textarea)
  Replace the single-line `textinput` for word search with a textarea
  using `WithDynamicHeight` so longer phrase searches don't truncate.

- [x] **Mouse hover highlights** (bubbletea v2 Motion mouse messages)
  Mouse mode switched to `MouseModeAllMotion`. Hover state is tracked
  via `mouseX/mouseY` and surfaced in two places: a subtle
  `theme.Highlight`-bg bar on the book row under the cursor in the
  left pane, and a `⊙ v. N` indicator appended to the right pane title
  when the cursor is over a verse in the viewport (suppressed when it
  would duplicate the existing scroll/highlight locator). Verses are
  resolved via a `verseAtMouseY` helper that mirrors `formatChapter`'s
  line-counting so the mapping is consistent with what's actually drawn.

## Medium

- [x] **Theme-aware light/dark auto-pick** (bubbletea v2 `RequestBackgroundColor`)
  At startup, queries the terminal's background color and switches to
  Catppuccin Latte (light) or Catppuccin Mocha (dark) when no theme is
  already saved in settings. Once the user picks a theme via the `T`
  picker it's marked pinned and the auto-pick won't override it.

- [x] **Progress bar for translation downloads** (bubbles v2 progress)
  Cache layer wraps the HTTP body in a counting `progressReader` that
  publishes byte-level progress via `Cache.DownloadProgress() (float64, string)`.
  The UI polls every 120ms via a `tea.Tick` while a download is
  running, updates `m.downloadProgress`, and renders a
  `bubbles/v2/progress` bar (default blend, 48 cells wide) inside the
  cache manager overlay just below the translation list. The bar
  disappears on `downloadCompleteMsg` / `downloadErrorMsg`.

- [x] **Verse range selection (shift+click and click+drag)**
  Plain left-click on a verse highlights it and sets a drag anchor.
  Click-and-drag with the left button held extends the highlight live
  from the anchor to whichever verse the cursor is over (works in any
  terminal — no modifier intercept). Shift+left-click is also wired
  and extends the existing highlight to the clicked verse (works in
  terminals that don't swallow shift+click for native selection).
  Both paths normalize the range so start ≤ end. Verse resolution
  uses the same `verseAtMouseY` helper as the hover indicator.

- [x] **Sticky chapter header on scroll**
  The right pane title is always pinned above the viewport. Once the
  reader scrolls past the top, the blank row directly below the title
  morphs into a styled sticky bar reading `↑ Book Chapter:Verse`,
  updating live as the wheel or `j`/`k` move the viewport. The bar
  disappears once the viewport is back at YOffset == 0.

- [ ] **Key-release detection for vim-style "gg" / "G"** (bubbletea v2 Kitty kbd protocol)
  Use the progressive keyboard enhancements to detect held vs.
  released chords cleanly. Lets us add `gg` (top of chapter) and `G`
  (end) without the current dead `hKeyPending`/`nKeyPending` hack.

- [x] **Theme preview side-by-side picker**
  `modeThemeSelect` now renders the theme list on the left and a live
  preview pane on the right showing book title, sample verse, the
  highlighted-verse rounded box, and another sample verse — all
  styled with the *focused* (not committed) theme, so arrow keys
  preview the look without applying it. Pressing Enter commits.

## Lower priority / nice-to-have

- [ ] **24-bit color downsampling for older terms** (bubbletea v2 colorprofile auto-detect)
  Audit the new Bru / Jozi palettes against 256-color and 16-color
  downsampling. v2 does this automatically, but we should verify the
  themes degrade legibly.

- [ ] **Inline footnote popups** (lipgloss v2 Z-ordered layers)
  When a verse has an embedded footnote marker, hover/click opens a
  small overlay panel. Currently we strip them.

- [ ] **Persistent bookmarks list** (out-of-band, but a v2 sidebar feature)
  Add a bookmarks sidebar (`b`). Persists via the existing
  `settings.json`. Mostly settings work but reuses the v2 list
  component cleanly.

- [ ] **Custom theme loader** (theme package extension)
  Read `~/.config/sword-tui/themes/*.json` (matching the
  `palette.json` shape used by Bru / Jozi) and surface as additional
  themes. Lets users drop in new schemes without a rebuild.

- [ ] **Status line / message log** (bubbletea v2 alternate renderer)
  Brief toast for "Highlight saved", "Bookmark added", etc.
  v2 renderer handles overlays without flicker.

