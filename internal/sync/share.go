package sync

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// shareResponse captures the fields we need from a created gist.
type shareResponse struct {
	HTMLURL string `json:"html_url"`
}

// ShareGist publishes content as a single-file secret gist and returns its
// human-facing URL. It reuses the gist payload shapes from the sync backend.
func ShareGist(token, filename, content, description string) (string, error) {
	payload := gistPayload{
		Description: description,
		Public:      false,
		Files:       map[string]gistFile{filename: {Content: content}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.github.com/gists", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", apiError(resp)
	}
	var sr shareResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", err
	}
	return sr.HTMLURL, nil
}
