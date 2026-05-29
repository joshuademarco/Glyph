package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/joshuademarco/glyph/internal/model"
)

const gistFilename = "glyph-snippets.json"

// GistBackend syncs through a private GitHub Gist.
type GistBackend struct {
	ID    string
	Token string

	onCreate func(id string)
}

// registers a callback fired when a new gist is created during Push.
func (g *GistBackend) OnCreate(fn func(id string)) { g.onCreate = fn }

func (g *GistBackend) Name() string {
	if g.ID == "" {
		return "gist (new)"
	}
	return "gist (" + g.ID + ")"
}

type gistFile struct {
	Content string `json:"content"`
}

type gistPayload struct {
	Description string              `json:"description,omitempty"`
	Public      bool                `json:"public"`
	Files       map[string]gistFile `json:"files"`
}

type gistResponse struct {
	ID    string `json:"id"`
	Files map[string]struct {
		Content  string `json:"content"`
		Truncate bool   `json:"truncated"`
		RawURL   string `json:"raw_url"`
	} `json:"files"`
}

func (g *GistBackend) client() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func (g *GistBackend) do(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+g.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return g.client().Do(req)
}

func (g *GistBackend) Pull() (*model.Library, error) {
	if g.ID == "" {
		return nil, nil
	}
	resp, err := g.do(http.MethodGet, "https://api.github.com/gists/"+g.ID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}
	var gr gistResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, err
	}
	file, ok := gr.Files[gistFilename]
	if !ok {
		return nil, nil
	}
	content := file.Content
	if file.Truncate && file.RawURL != "" {
		// Large gists are truncated in the API response; fetch the raw file.
		raw, err := g.client().Get(file.RawURL)
		if err != nil {
			return nil, err
		}
		defer raw.Body.Close()
		b, err := io.ReadAll(raw.Body)
		if err != nil {
			return nil, err
		}
		content = string(b)
	}
	return unmarshal([]byte(content))
}

func (g *GistBackend) Push(lib *model.Library) error {
	b, err := marshal(lib)
	if err != nil {
		return err
	}
	payload := gistPayload{
		Description: "glyph snippet library",
		Public:      false,
		Files:       map[string]gistFile{gistFilename: {Content: string(b)}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	method, url := http.MethodPost, "https://api.github.com/gists"
	if g.ID != "" {
		method, url = http.MethodPatch, "https://api.github.com/gists/"+g.ID
	}
	resp, err := g.do(method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return apiError(resp)
	}
	var gr gistResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return err
	}
	if g.ID == "" && gr.ID != "" {
		g.ID = gr.ID
		if g.onCreate != nil {
			g.onCreate(gr.ID)
		}
	}
	return nil
}

func apiError(resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("github gist API %s: %s", resp.Status, bytes.TrimSpace(b))
}
