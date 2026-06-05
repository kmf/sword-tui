package cache

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sword-tui/internal/api"
)

const baseURL = "https://bolls.life/static/translations"

type Cache struct {
	cacheDir string

	mu       sync.Mutex
	progress float64 // [0, 1] for the current download, 0 if idle
	active   string  // translation short-name being downloaded, or ""
}

// progressReader wraps an io.Reader and reports the byte count consumed so far
// to a parent Cache via its setProgress method.
type progressReader struct {
	r     io.Reader
	cache *Cache
	read  int64
	total int64
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.read += int64(n)
		if p.total > 0 {
			p.cache.setProgress(float64(p.read) / float64(p.total))
		}
	}
	return n, err
}

func (c *Cache) setProgress(p float64) {
	c.mu.Lock()
	c.progress = p
	c.mu.Unlock()
}

// DownloadProgress returns the current download's byte progress as a value
// in [0, 1], plus the translation short-name being downloaded ("" if idle).
// Safe to call from any goroutine.
func (c *Cache) DownloadProgress() (float64, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.progress, c.active
}

func NewCache() (*Cache, error) {
	// Get user's cache directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cacheDir := filepath.Join(homeDir, ".cache", "sword-tui", "translations")

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}

	return &Cache{cacheDir: cacheDir}, nil
}

// IsCached checks if a translation is already downloaded
func (c *Cache) IsCached(translation string) bool {
	path := filepath.Join(c.cacheDir, translation+".json")
	_, err := os.Stat(path)
	return err == nil
}

// DownloadTranslation downloads and caches a translation. While the download
// runs the cache exposes byte-level progress via DownloadProgress().
func (c *Cache) DownloadTranslation(translation string) error {
	// Mark this download as active so the UI can poll for progress.
	c.mu.Lock()
	c.active = translation
	c.progress = 0
	c.mu.Unlock()
	// Idle-out when the download finishes (either branch).
	defer func() {
		c.mu.Lock()
		c.active = ""
		c.progress = 0
		c.mu.Unlock()
	}()

	url := fmt.Sprintf("%s/%s.zip", baseURL, translation)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", translation+"*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Wrap the HTTP body so we publish bytes-read as a fraction of
	// Content-Length. If the server doesn't send Content-Length the bar
	// stays at 0 — the bubbles/v2 progress renderer handles that as
	// "indeterminate"-looking.
	pr := &progressReader{r: resp.Body, cache: c, total: resp.ContentLength}
	if _, err := io.Copy(tmpFile, pr); err != nil {
		return err
	}

	// Treat unzip as the final stretch (97% → 100%).
	c.setProgress(0.97)
	if err := c.extractJSON(tmpFile.Name(), translation); err != nil {
		return err
	}
	c.setProgress(1.0)
	return nil
}

func (c *Cache) extractJSON(zipPath, translation string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// Find the JSON file in the ZIP
	for _, f := range r.File {
		if filepath.Ext(f.Name) == ".json" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			// Create output file
			outPath := filepath.Join(c.cacheDir, translation+".json")
			outFile, err := os.Create(outPath)
			if err != nil {
				return err
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, rc)
			return err
		}
	}

	return fmt.Errorf("no JSON file found in ZIP")
}

// GetChapter retrieves a chapter from cached data
func (c *Cache) GetChapter(translation string, book, chapter int) ([]api.Verse, error) {
	if !c.IsCached(translation) {
		return nil, fmt.Errorf("translation %s not cached", translation)
	}

	path := filepath.Join(c.cacheDir, translation+".json")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var allVerses []api.Verse
	decoder := json.NewDecoder(file)

	if err := decoder.Decode(&allVerses); err != nil {
		return nil, err
	}

	// Filter verses for the requested book and chapter
	var verses []api.Verse
	for _, v := range allVerses {
		if v.Book == book && v.Chapter == chapter {
			verses = append(verses, v)
		}
	}

	return verses, nil
}

// GetVerse retrieves a single verse from cached data
func (c *Cache) GetVerse(translation string, book, chapter, verse int) (*api.Verse, error) {
	verses, err := c.GetChapter(translation, book, chapter)
	if err != nil {
		return nil, err
	}

	for _, v := range verses {
		if v.Verse == verse {
			return &v, nil
		}
	}

	return nil, fmt.Errorf("verse not found")
}

// ListCached returns a list of cached translations
func (c *Cache) ListCached() ([]string, error) {
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return nil, err
	}

	var translations []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			name := entry.Name()
			translation := name[:len(name)-5] // Remove .json extension
			translations = append(translations, translation)
		}
	}

	return translations, nil
}

// ClearCache removes all cached translations
func (c *Cache) ClearCache() error {
	return os.RemoveAll(c.cacheDir)
}

// RemoveTranslation removes a specific cached translation
func (c *Cache) RemoveTranslation(translation string) error {
	path := filepath.Join(c.cacheDir, translation+".json")
	return os.Remove(path)
}

// GetCacheSize returns the total size of cached data in bytes
func (c *Cache) GetCacheSize() (int64, error) {
	var size int64
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			size += info.Size()
		}
	}

	return size, nil
}
