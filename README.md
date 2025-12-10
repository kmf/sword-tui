# sword-tui

A terminal-based Bible application built with Go, bubbletea, and lipgloss.

## Features

### Navigation
- **Miller Columns Interface**: Three-column navigation system (Books → Chapters → Verses)
- **Sidebar Toggle**: Show/hide sidebar with `[` key
- **Verse Picker**: Quick verse navigation with filtering capability
- **Keyboard-Driven**: Full keyboard navigation with vim-like bindings

### Bible Access
- **Multiple Translations**: Switch between different Bible translations on the fly
- **Translation Comparison**: View multiple translations side-by-side
- **Verse Lookup**: Jump directly to any book, chapter, and verse
- **Offline Cache**: Automatic caching system for offline reading

### User Interface
- **Modern Terminal UI**: Built with bubbletea and lipgloss
- **7 Theme Options**:
  - Catppuccin Mocha
  - Catppuccin Latte
  - Dracula
  - Rosé Pine Moon
  - Rosé Pine Dawn
  - Solarized Dark
  - Solarized Light
- **Viewport-Based Text Wrapping**: Prevents text from rendering off-screen
- **Visual Depth Effects**: Dimming and shadow effects for focused elements
- **Auto-Scroll**: Smooth scrolling through long passages
- **Status Bar**: Displays current version and build information

### Productivity
- **Copy/Yank Functionality**: Copy verses to clipboard
- **Search/Filter**: Filter verses with `/` key
- **Quick Navigation**: Jump between books, chapters, and verses efficiently

## Installation

### From Source

```bash
go build -o sword-tui cmd/sword-tui/main.go
```

### Arch Linux (AUR)

```bash
yay -S sword-tui
```

## Usage

```bash
./sword-tui
```

### Keyboard Shortcuts

- `[` - Toggle books sidebar
- `/` - Filter/search verses
- `y` - Yank/copy selected verse
- `j/k` - Navigate down/up
- `h/l` - Navigate left/right (between columns)
- `Enter` - Select item
- `t` - Switch theme
- `q` - Quit

## API

Uses the [bolls.life API](https://bolls.life/api/) for Bible data.

## License

GPL-2.0-or-later
