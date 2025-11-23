# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

sword-tui is a terminal-based Bible application built with Go using the Bubbletea TUI framework. It connects to the bolls.life API for Bible data.

## Build & Run

```bash
# Build the application
go build -o sword-tui cmd/sword-tui/main.go

# Run the application
./sword-tui
```

## Architecture

The application follows a clean three-layer architecture:

### 1. Entry Point (`cmd/sword-tui/main.go`)
- Initializes the Bubbletea program with alt screen and mouse support
- Creates and runs the UI model

### 2. API Layer (`internal/api/client.go`)
- Handles all HTTP communication with bolls.life API
- Key structs: `Translation`, `Book`, `Verse`, `LanguageGroup`, `ParallelVerseRequest`
- Important: `GetTranslations()` filters for English translations only
- Book IDs range from 1-66 (1-39 Old Testament, 40-66 New Testament)

### 3. UI Layer (`internal/ui/model.go`)
- Implements the Bubbletea Model-View-Update pattern
- State management through `viewMode` enum and boolean flags (`showSidebar`, `showTranslationList`)
- Uses bubbles components: `viewport` for scrollable content, `textinput` for search
- Lipgloss for styling

## Key UI Interactions

### Sidebar System
The app has two independent sidebars:
- **Book sidebar** (`[` key): Shows Old Testament and New Testament books
- **Translation sidebar** (`]` key): Shows English translations with virtual scrolling

**Critical implementation detail**: When opening sidebars, must correctly map current state to array indices:
- Book sidebar: Find index by searching `m.books` array for matching `BookID` (not `BookID - 1`)
- Translation sidebar: Find index by searching `m.translations` for matching `ShortName`

### Navigation
- `n`/`p`: Next/previous chapter
- `/`: Search for verse reference (format: "book chapter:verse" or "1 1:1")
- `c`: Comparison view (parallel verses across multiple translations)
- `↑`/`↓` or `j`/`k`: Navigate in sidebars
- `enter`: Select item in sidebar or execute search
- `esc`: Close sidebars or cancel search
- Mouse clicks work in sidebars

### Virtual Scrolling
Translation sidebar implements virtual scrolling to handle 40+ items:
- Calculates visible window around selected item
- Shows "... (N more above/below)" indicators
- Centers selected translation in view

## Data Flow

1. **Initialization**: `Init()` loads translations, books, and initial chapter in parallel
2. **Messages**: Async operations return typed messages (`translationsLoadedMsg`, `booksLoadedMsg`, `chapterLoadedMsg`, etc.)
3. **Update**: Model processes messages and returns commands for side effects
4. **View**: Renders based on current mode and flags

## Common Issues

- **JSON unmarshaling**: Ensure struct field types match API response (e.g., `BookID` is `int`, `Chapters` is `int`)
- **Array indexing**: BookID is not an array index; always search the books array
- **Sidebar conditions**: Sidebars toggle independently; don't block one with the other's state

## Git Conventions

Use conventional commits:
- `feat:` - New features
- `fix:` - Bug fixes
- `refactor:` - Code restructuring

All commits include Claude Code attribution footer.
