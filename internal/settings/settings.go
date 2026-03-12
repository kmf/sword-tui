package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Highlight struct {
	VersePK int    `json:"verse_pk"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Color   string `json:"color"`
}

type Reference struct {
	BookID      int    `json:"book_id"`
	BookName    string `json:"book_name"`
	Chapter     int    `json:"chapter"`
	Verse       int    `json:"verse"`
	Translation string `json:"translation"`
}

type Note struct {
	Translation string      `json:"translation"`
	VersePK     int         `json:"verse_pk"`
	WordIndex   int         `json:"word_index"` // Index of the word in the verse
	Symbol      string      `json:"symbol"`     // The symbol or number displayed
	Text        string      `json:"text"`       // The actual note content
	References  []Reference `json:"references"`
}

type Settings struct {
	SelectedLanguage       string      `json:"selected_language"`
	SelectedTranslation    string      `json:"selected_translation"`
	CurrentBook            int         `json:"current_book"`
	CurrentChapter         int         `json:"current_chapter"`
	CurrentTheme           string      `json:"current_theme"` // theme display name
	Highlights             []Highlight `json:"highlights"`
	SelectedHighlightColor string      `json:"selected_highlight_color"`
	Notes                  []Note      `json:"notes"`
	SelectedSymbolStyle    string      `json:"selected_symbol_style"` // "numbers", "symbols", etc.
}

func configPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(configDir, "sword-tui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.json"), nil
}

func Load() (Settings, error) {
	var s Settings

	path, err := configPath()
	if err != nil {
		return s, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// No config = just return zero value, no error
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}

	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, err
	}

	return s, nil
}

func Save(s Settings) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
