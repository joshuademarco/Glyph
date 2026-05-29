package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds user-tunable settings, persisted alongside the library.
type Config struct {
	// Sync selects the backend: "" (off), "gist", or "file".
	Sync string `json:"sync,omitempty"`

	// Gist backend settings.
	GistID    string `json:"gistId,omitempty"`
	GistToken string `json:"gistToken,omitempty"` // a GitHub PAT with the "gist" scope

	// File backend settings: a path inside a synced folder
	FilePath string `json:"filePath,omitempty"`

	// Editor overrides $EDITOR for the `glyph edit`/external-edit flows.
	Editor string `json:"editor,omitempty"`
}

// Dir returns the glyph configuration directory, honouring $GLYPH_HOME first. then the OS convention (XDG / %AppData% / ~/Library).
func Dir() (string, error) {
	if h := os.Getenv("GLYPH_HOME"); h != "" {
		return h, ensure(h)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	dir := filepath.Join(base, "glyph")
	return dir, ensure(dir)
}

// LibraryPath is the path to the local snippet library file.
func LibraryPath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "snippets.json"), nil
}

// configPath is the path to the settings file.
func configPath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

// Load reads the config file, returning a zero Config if none exists yet.
func Load() (*Config, error) {
	p, err := configPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes the config file atomically.
func (c *Config) Save() error {
	p, err := configPath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func ensure(dir string) error {
	return os.MkdirAll(dir, 0o755)
}
