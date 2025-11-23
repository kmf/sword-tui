package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const baseURL = "https://bolls.life"

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{},
	}
}

type Translation struct {
	ShortName string `json:"short_name"`
	FullName  string `json:"full_name"`
	Updated   string `json:"updated"`
	Dir       string `json:"dir,omitempty"`
}

type Book struct {
	BookID     int    `json:"bookid"`
	ChronOrder int    `json:"chronorder"`
	Name       string `json:"name"`
	Chapters   []int  `json:"chapters"`
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

	var translations []Translation
	if err := json.NewDecoder(resp.Body).Decode(&translations); err != nil {
		return nil, err
	}

	return translations, nil
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
