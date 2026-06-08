# sword-tui

A terminal-based Bible application built with Go, bubbletea, and lipgloss.

## Features

### Navigation
- **Multi-Pane Shell**: Permanent two-pane layout with rounded borders and drop-shadow overlays
- **Books & Translations Sidebars**: Independent pickers on `[` and `]`
- **Smart Verse References**: Parses `rom8:8`, `rom 8 8`, `1 john 3 16`, and similar
- **Mouse Support**: Click, drag, hover, and wheel for navigation and verse selection
- **Keyboard-Driven**: Full keyboard navigation with vim-like bindings

### Bible Access
- **Multiple Translations**: Switch between English translations on the fly
- **Side-by-Side Comparison**: Per-column translation pickers for parallel reading
- **Verse Lookup**: Jump directly to any book, chapter, and verse
- **Offline Cache**: Automatic caching with a real byte-level progress bar for downloads
- **Persistent State**: Theme and last-read position survive restarts

### User Interface
- **Modern Terminal UI**: Built on the charm v2 stack (bubbletea, lipgloss)
- **12 Themes** across dark and light variants:
  - Catppuccin Mocha / Latte
  - Dracula
  - Rosé Pine Moon / Dawn
  - Solarized Dark / Light
  - Bru Espresso / Latte
  - Jozi Nights / Morning / Midnight
- **Auto Light/Dark Detection**: Picks a sensible default based on terminal background
- **Live Theme Preview**: See a preview card while choosing a theme
- **Sticky Chapter Header**: Morphs into a scroll indicator as you read
- **Viewport-Based Text Wrapping**: Prevents text from rendering off-screen
- **Visual Depth Effects**: Dimming and shadow effects for focused elements
- **Status Bar**: Displays current version and build information

### Productivity
- **Copy/Yank**: Copy selected verse(s) to clipboard
- **Click-and-Drag Range Selection**: Select multi-verse ranges with the mouse
- **Search/Filter**: Filter verses with `/`
- **Quick Navigation**: `n`/`p` step between chapters; sidebars jump between books and translations

## Installation

### Homebrew (macOS and Linux)

```bash
# Install from the tap
brew tap kmf/sword-tui
brew install sword-tui
```

### From Source

```bash
go build -o sword-tui cmd/sword-tui/main.go
```

### Arch Linux (AUR)

```bash
yay -S sword-tui
```

### Fedora / RHEL (COPR)

Enable the [kmf/sword-tui](https://copr.fedorainfracloud.org/coprs/kmf/sword-tui/) COPR repo and install:

```bash
sudo dnf copr enable kmf/sword-tui
sudo dnf install sword-tui
```

Supported chroots: Fedora (current and previous release), RHEL/CentOS Stream 9 (`epel-9`), RHEL/CentOS Stream 10 (`epel-10`).

## Usage

```bash
./sword-tui
```

### Keyboard Shortcuts

- `[` / `]` - Focus books pane / content pane
- `n` / `p` - Next / previous chapter
- `j` / `k`, `↓` / `↑` - Navigate down / up
- `h` / `l`, `←` / `→` - Navigate left / right between panes
- `tab` / `shift+tab` - Cycle focus between panes
- `PgUp` / `PgDn` - Scroll a page at a time
- `/` - Search by verse reference
- `s` - Word search
- `v` - Toggle Miller-columns picker (Books → Chapters → Verses)
- `c` - Comparison view (side-by-side translations)
- `t` - Translation picker
- `T` - Theme picker
- `d` - Cache manager (`x` deletes a cached translation here)
- `r` - Return to reader from any overlay
- `y` - Yank/copy selected verse
- `?` - About
- `Enter` - Select item
- `esc` - Close overlay / cancel
- `q`, `Ctrl-C` - Quit

## API

Uses the [bolls.life API](https://bolls.life/api/) for Bible data.

## License

GPL-2.0-or-later
