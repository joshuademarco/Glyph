package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joshuademarco/glyph/internal/config"
	"github.com/joshuademarco/glyph/internal/model"
	gsync "github.com/joshuademarco/glyph/internal/sync"
)

type syncDoneMsg struct {
	res *gsync.Result
	err error
}

type shareDoneMsg struct {
	url string
	err error
}

type externalEditMsg struct {
	content string
	ok      bool
	err     error
}

// syncCmd runs a two-way sync against the configured backend off the UI thread.
func (m Model) syncCmd() tea.Cmd {
	st := m.st
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return syncDoneMsg{err: err}
		}
		b, err := gsync.New(cfg)
		if err != nil {
			return syncDoneMsg{err: err}
		}
		if gb, ok := b.(*gsync.GistBackend); ok {
			gb.OnCreate(func(id string) {
				cfg.GistID = id
				_ = cfg.Save()
			})
		}
		res, err := gsync.Sync(st.Lib, b, func(lib *model.Library) error {
			st.Lib = lib
			return st.Save()
		})
		return syncDoneMsg{res: res, err: err}
	}
}

// shareCmd publishes the snippet body to a one-off secret gist and returns its URL.
func (m Model) shareCmd(s *model.Snippet) tea.Cmd {
	command := s.Command
	title := s.Title
	lang := s.Lang
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return shareDoneMsg{err: err}
		}
		if cfg.GistToken == "" {
			return shareDoneMsg{err: fmt.Errorf("no gist token configured — run `glyph sync setup gist`")}
		}
		name := sanitizeFilename(title)
		if name == "" {
			name = "snippet"
		}
		url, err := gsync.ShareGist(cfg.GistToken, name+langSuffix(lang), command, title)
		return shareDoneMsg{url: url, err: err}
	}
}

// externalEditorCmd opens the user's $EDITOR on the command body, returning the
// edited content as an externalEditMsg.
func (m Model) externalEditorCmd(initial string) tea.Cmd {
	ed := resolveEditor()
	tmp, err := os.CreateTemp("", "glyph-*.txt")
	if err != nil {
		return func() tea.Msg { return externalEditMsg{err: err} }
	}
	path := tmp.Name()
	if initial != "" {
		_, _ = tmp.WriteString(initial)
	}
	_ = tmp.Close()

	c := exec.Command(ed, path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(path)
			return externalEditMsg{err: fmt.Errorf("editor %q: %w", ed, err)}
		}
		b, rerr := os.ReadFile(path)
		_ = os.Remove(path)
		if rerr != nil {
			return externalEditMsg{err: rerr}
		}
		return externalEditMsg{content: strings.TrimRight(string(b), "\r\n"), ok: true}
	})
}

func resolveEditor() string {
	if ed := os.Getenv("GLYPH_EDITOR"); ed != "" {
		return ed
	}
	if cfg, _ := config.Load(); cfg != nil && cfg.Editor != "" {
		return cfg.Editor
	}
	if ed := os.Getenv("EDITOR"); ed != "" {
		return ed
	}
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "vi"
}

// sanitizeFilename turns a title into a safe, lowercase gist filename stem.
func sanitizeFilename(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func langSuffix(lang string) string {
	switch strings.ToLower(lang) {
	case "sh", "bash", "zsh", "fish":
		return ".sh"
	case "py", "python":
		return ".py"
	case "sql":
		return ".sql"
	case "yml", "yaml":
		return ".yaml"
	case "json":
		return ".json"
	case "":
		return ".txt"
	default:
		return "." + strings.ToLower(lang)
	}
}
