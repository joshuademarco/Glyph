package model

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"time"
)

// SchemaVersion is bumped when the on-disk/synced format changes in a backwards-incompatible way.
const SchemaVersion = 1

// Snippet is a single stored command or code fragment.
type Snippet struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Command   string     `json:"command"`          // the body — command(s) or code
	Notes     string     `json:"notes,omitempty"`  // free-form description
	Folder    string     `json:"folder,omitempty"` // slash-separated, e.g. "Docker/Compose"
	Tags      []string   `json:"tags,omitempty"`   // without leading '#'
	Lang      string     `json:"lang,omitempty"`   // sh, yml, sql, py, ...
	Favorite  bool       `json:"favorite,omitempty"`
	Dangerous bool       `json:"dangerous,omitempty"`
	UseCount  int        `json:"useCount,omitempty"`
	Source    string     `json:"source,omitempty"` // where it came from, e.g. a file path
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	LastUsed  *time.Time `json:"lastUsed,omitempty"`
}

var varRe = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)

// Variables returns the unique {{placeholder}} names found in the command body, in first-seen order.
func (s *Snippet) Variables() []string {
	matches := varRe.FindAllStringSubmatch(s.Command, -1)
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// Resolve substitutes the provided variable values into the command body.
func (s *Snippet) Resolve(vals map[string]string) string {
	return varRe.ReplaceAllStringFunc(s.Command, func(match string) string {
		name := varRe.FindStringSubmatch(match)[1]
		if v, ok := vals[name]; ok {
			return v
		}
		return match
	})
}

// FolderParts splits the folder path into its segments.
func (s *Snippet) FolderParts() []string {
	if strings.TrimSpace(s.Folder) == "" {
		return nil
	}
	return strings.Split(s.Folder, "/")
}

// MarkUsed records a usage at the given time.
func (s *Snippet) MarkUsed(at time.Time) {
	s.UseCount++
	t := at
	s.LastUsed = &t
}

// dangerRe matches commands that are commonly destructive, used to flag a
var dangerRe = regexp.MustCompile(`(?i)(rm\s+-rf|mkfs|dd\s+if=|drop\s+(table|database)|:\s*\(\)\s*\{|\bprune\b|truncate\s+table|--force-delete|git\s+push\s+.*--force)`)

// LooksDangerous reports whether the command body matches a destructive pattern.
func (s *Snippet) LooksDangerous() bool {
	return dangerRe.MatchString(s.Command) || dangerRe.MatchString(s.Title)
}

// Library is the whole collection — the unit that is persisted and synced.
type Library struct {
	Schema    int        `json:"schema"`
	Snippets  []*Snippet `json:"snippets"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// NewLibrary returns an empty library at the current schema version.
func NewLibrary() *Library {
	return &Library{Schema: SchemaVersion, Snippets: []*Snippet{}, UpdatedAt: time.Now().UTC()}
}

// Find returns the snippet with the given ID, or nil.
func (l *Library) Find(id string) *Snippet {
	for _, s := range l.Snippets {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// Add appends a snippet, assigning an ID and timestamps if missing.
func (l *Library) Add(s *Snippet) {
	now := time.Now().UTC()
	if s.ID == "" {
		s.ID = NewID()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	s.UpdatedAt = now
	if s.LooksDangerous() {
		s.Dangerous = true
	}
	l.Snippets = append(l.Snippets, s)
	l.UpdatedAt = now
}

// Remove deletes the snippet with the given ID, reporting whether it existed.
func (l *Library) Remove(id string) bool {
	for i, s := range l.Snippets {
		if s.ID == id {
			l.Snippets = append(l.Snippets[:i], l.Snippets[i+1:]...)
			l.UpdatedAt = time.Now().UTC()
			return true
		}
	}
	return false
}

// Folders returns the distinct folder paths, sorted.
func (l *Library) Folders() []string {
	seen := map[string]bool{}
	for _, s := range l.Snippets {
		if s.Folder != "" {
			seen[s.Folder] = true
		}
	}
	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// FolderCandidates returns every folder path plus all of their ancestor
func (l *Library) FolderCandidates() []string {
	seen := map[string]bool{}
	for _, s := range l.Snippets {
		parts := s.FolderParts()
		for i := range parts {
			seen[strings.Join(parts[:i+1], "/")] = true
		}
	}
	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// Tags returns the distinct tags across the library, sorted.
func (l *Library) Tags() []string {
	seen := map[string]bool{}
	for _, s := range l.Snippets {
		for _, t := range s.Tags {
			seen[t] = true
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// NewID returns a short, collision-resistant identifier.
func NewID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
