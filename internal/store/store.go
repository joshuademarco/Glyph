// Package store handles loading, querying and persisting the snippet library.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/joshuademarco/glyph/internal/config"
	"github.com/joshuademarco/glyph/internal/model"
	"github.com/sahilm/fuzzy"
)

// Store wraps an in-memory library backed by a JSON file on disk.
type Store struct {
	path string
	Lib  *model.Library
}

// Open loads the library from the default location
func Open() (*Store, error) {
	p, err := config.LibraryPath()
	if err != nil {
		return nil, err
	}
	return OpenAt(p)
}

// OpenAt loads the library from a specific path.
func OpenAt(path string) (*Store, error) {
	s := &Store{path: path}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		s.Lib = model.NewLibrary()
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var lib model.Library
	if err := json.Unmarshal(b, &lib); err != nil {
		return nil, fmt.Errorf("parse library %s: %w", path, err)
	}
	if lib.Snippets == nil {
		lib.Snippets = []*model.Snippet{}
	}
	s.Lib = &lib
	return s, nil
}

// Save writes the library to disk atomically.
func (s *Store) Save() error {
	b, err := json.MarshalIndent(s.Lib, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Path returns the file path backing this store.
func (s *Store) Path() string { return s.path }

// Filter narrows snippets by folder (prefix match) and tag.
func (s *Store) Filter(folder, tag string) []*model.Snippet {
	var out []*model.Snippet
	for _, sn := range s.Lib.Snippets {
		if folder != "" && !folderMatch(sn.Folder, folder) {
			continue
		}
		if tag != "" && !hasTag(sn, tag) {
			continue
		}
		out = append(out, sn)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

// Search fuzzy-matches a query against title, folder and tags, best match first.
func (s *Store) Search(query string) []*model.Snippet {
	all := s.Lib.Snippets
	if strings.TrimSpace(query) == "" {
		out := append([]*model.Snippet(nil), all...)
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		})
		return out
	}
	haystack := make([]string, len(all))
	for i, sn := range all {
		haystack[i] = sn.Title + " " + sn.Folder + " " + strings.Join(sn.Tags, " ")
	}
	matches := fuzzy.Find(query, haystack)
	out := make([]*model.Snippet, 0, len(matches))
	for _, m := range matches {
		out = append(out, all[m.Index])
	}
	return out
}

// Resolve finds a snippet by exact ID, then by unique ID prefix, then by exact
func (s *Store) Resolve(ref string) (*model.Snippet, error) {
	if sn := s.Lib.Find(ref); sn != nil {
		return sn, nil
	}
	// unique id prefix
	var byPrefix []*model.Snippet
	for _, sn := range s.Lib.Snippets {
		if strings.HasPrefix(sn.ID, ref) {
			byPrefix = append(byPrefix, sn)
		}
	}
	if len(byPrefix) == 1 {
		return byPrefix[0], nil
	}
	// exact title
	for _, sn := range s.Lib.Snippets {
		if strings.EqualFold(sn.Title, ref) {
			return sn, nil
		}
	}
	// fuzzy fallback
	if hits := s.Search(ref); len(hits) == 1 {
		return hits[0], nil
	} else if len(hits) > 1 {
		return nil, fmt.Errorf("%q is ambiguous: %d matches (try a more specific term or an id)", ref, len(hits))
	}
	return nil, fmt.Errorf("no snippet matching %q", ref)
}

func folderMatch(have, want string) bool {
	have = strings.ToLower(have)
	want = strings.ToLower(want)
	return have == want || strings.HasPrefix(have, want+"/")
}

func hasTag(sn *model.Snippet, tag string) bool {
	tag = strings.TrimPrefix(strings.ToLower(tag), "#")
	for _, t := range sn.Tags {
		if strings.ToLower(t) == tag {
			return true
		}
	}
	return false
}
