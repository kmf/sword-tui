package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const baseURL = "https://bolls.life"

type CacheInterface interface {
	IsCached(translation string) bool
	GetChapter(translation string, book, chapter int) ([]Verse, error)
	GetVerse(translation string, book, chapter, verse int) (*Verse, error)
}

type Client struct {
	httpClient *http.Client
	cache      CacheInterface
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{},
	}
}

func (c *Client) SetCache(cache CacheInterface) {
	c.cache = cache
}

type Translation struct {
	ShortName string `json:"short_name"`
	FullName  string `json:"full_name"`
	Updated   int64  `json:"updated"`
	Dir       string `json:"dir,omitempty"`
}

type LanguageGroup struct {
	Language     string        `json:"language"`
	Translations []Translation `json:"translations"`
}

type Book struct {
	BookID     int    `json:"bookid"`
	ChronOrder int    `json:"chronorder"`
	Name       string `json:"name"`
	Chapters   int    `json:"chapters"`
}

type Verse struct {
	PK          int    `json:"pk"`
	Verse       int    `json:"verse"`
	Text        string `json:"text"`
	Translation string `json:"translation,omitempty"`
	Book        int    `json:"book,omitempty"`
	Chapter     int    `json:"chapter,omitempty"`
}

type ParallelVerseRequest struct {
	Translations []string `json:"translations"`
	Verses       []int    `json:"verses"`
	Chapter      int      `json:"chapter"`
	Book         int      `json:"book"`
}

type SearchResponse struct {
	ExactMatches int     `json:"exact_matches"`
	Total        int     `json:"total"`
	Results      []Verse `json:"results"`
}

func (c *Client) GetTranslations() ([]Translation, error) {
	url := fmt.Sprintf("%s/static/bolls/app/views/languages.json", baseURL)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var languageGroups []LanguageGroup
	if err := json.NewDecoder(resp.Body).Decode(&languageGroups); err != nil {
		return nil, err
	}

	// Filter for English translations only
	var englishTranslations []Translation
	for _, group := range languageGroups {
		if group.Language == "English" {
			englishTranslations = group.Translations
			break
		}
	}

	return englishTranslations, nil
}

func (c *Client) GetBooks(translation string) ([]Book, error) {
	url := fmt.Sprintf("%s/get-books/%s/", baseURL, translation)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var books []Book
	if err := json.NewDecoder(resp.Body).Decode(&books); err != nil {
		return nil, err
	}

	return books, nil
}

func (c *Client) GetChapter(translation string, book, chapter int) ([]Verse, error) {
	// Try cache first if available
	if c.cache != nil && c.cache.IsCached(translation) {
		return c.cache.GetChapter(translation, book, chapter)
	}

	// Fall back to API
	url := fmt.Sprintf("%s/get-text/%s/%d/%d/", baseURL, translation, book, chapter)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var verses []Verse
	if err := json.NewDecoder(resp.Body).Decode(&verses); err != nil {
		return nil, err
	}

	return verses, nil
}

func (c *Client) GetVerse(translation string, book, chapter, verse int) (*Verse, error) {
	// Try cache first if available
	if c.cache != nil && c.cache.IsCached(translation) {
		return c.cache.GetVerse(translation, book, chapter, verse)
	}

	// Fall back to API
	url := fmt.Sprintf("%s/get-verse/%s/%d/%d/%d/", baseURL, translation, book, chapter, verse)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var v Verse
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, err
	}

	return &v, nil
}

func (c *Client) GetParallelVerses(req ParallelVerseRequest) (map[string][]Verse, error) {
	url := fmt.Sprintf("%s/get-parallel-verses/", baseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Post(url, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Response is a nested array structure
	var rawResponse [][]Verse
	if err := json.NewDecoder(resp.Body).Decode(&rawResponse); err != nil {
		return nil, err
	}

	// Convert to map for easier access
	result := make(map[string][]Verse)
	for i, translation := range req.Translations {
		if i < len(rawResponse) {
			result[translation] = rawResponse[i]
		}
	}

	return result, nil
}

func (c *Client) SearchVerses(translation, query string) (*SearchResponse, error) {
	// Build URL with query parameters
	searchURL := fmt.Sprintf("%s/v2/find/%s", baseURL, translation)
	params := url.Values{}
	params.Set("search", query)
	params.Set("limit", "500") // Get more results

	fullURL := searchURL + "?" + params.Encode()

	resp, err := c.httpClient.Get(fullURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	return &searchResp, nil
}
