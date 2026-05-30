package sync

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/joshuademarco/glyph/internal/config"
	"github.com/joshuademarco/glyph/internal/model"
)

// Backend is a remote that can store and retrieve a single serialized library.
type Backend interface {
	Name() string
	// Pull fetches the remote library. A nil library with nil error means the remote exists but is empty / uninitialized.
	Pull() (*model.Library, error)
	// Push uploads the library, replacing the remote contents.
	Push(*model.Library) error
}

// New constructs the backend selected in the config, or an error
func New(cfg *config.Config) (Backend, error) {
	switch cfg.Sync {
	case "gist":
		if cfg.GistToken == "" {
			return nil, fmt.Errorf("gist sync needs a token: run `glyph sync setup gist`")
		}
		return &GistBackend{ID: cfg.GistID, Token: cfg.GistToken}, nil
	case "file":
		if cfg.FilePath == "" {
			return nil, fmt.Errorf("file sync needs a path: run `glyph sync setup file <path>`")
		}
		return &FileBackend{Path: cfg.FilePath}, nil
	case "":
		return nil, fmt.Errorf("sync is not configured: run `glyph sync setup`")
	default:
		return nil, fmt.Errorf("unknown sync backend %q", cfg.Sync)
	}
}

// Result reports what a Sync call changed.
type Result struct {
	Pulled  int
	Pushed  int
	Deleted int
}

// Sync performs a two-way merge between the local library and the backend, then
// writes the merged result both locally (via save) and remotely.
//
// Merge strategy: union by snippet ID, keeping whichever side has the newer
// UpdatedAt. This is last-write-wins per snippet — simple, predictable, and
// good enough for a single user across their own devices.
func Sync(local *model.Library, b Backend, save func(*model.Library) error) (*Result, error) {
	remote, err := b.Pull()
	if err != nil {
		return nil, fmt.Errorf("pull from %s: %w", b.Name(), err)
	}
	if remote == nil {
		remote = model.NewLibrary()
	}
	if remote.Schema > model.SchemaVersion {
		return nil, fmt.Errorf("remote library uses schema v%d; upgrade glyph", remote.Schema)
	}

	merged, res := merge(local, remote)
	merged.UpdatedAt = time.Now().UTC()

	if err := save(merged); err != nil {
		return nil, fmt.Errorf("save merged library: %w", err)
	}
	if err := b.Push(merged); err != nil {
		return nil, fmt.Errorf("push to %s: %w", b.Name(), err)
	}
	return res, nil
}

func merge(local, remote *model.Library) (*model.Library, *Result) {
	res := &Result{}

	// Union tombstones from both sides, keeping the latest deletion time per ID.
	tombs := map[string]time.Time{}
	for id, t := range local.Deleted {
		tombs[id] = t
	}
	for id, t := range remote.Deleted {
		if cur, ok := tombs[id]; !ok || t.After(cur) {
			tombs[id] = t
		}
	}

	// Union snippets by ID, last-write-wins on UpdatedAt.
	byID := map[string]*model.Snippet{}
	for _, s := range local.Snippets {
		byID[s.ID] = s
	}
	for _, r := range remote.Snippets {
		cur, ok := byID[r.ID]
		if !ok {
			byID[r.ID] = r
			res.Pulled++
			continue
		}
		if r.UpdatedAt.After(cur.UpdatedAt) {
			byID[r.ID] = r
			res.Pulled++
		} else if cur.UpdatedAt.After(r.UpdatedAt) {
			res.Pushed++
		}
	}

	// Apply tombstones.
	for id, t := range tombs {
		s, ok := byID[id]
		if ok && s.UpdatedAt.After(t) {
			delete(tombs, id)
			continue
		}
		if ok {
			delete(byID, id)
			res.Deleted++
		}
	}

	out := model.NewLibrary()
	out.Deleted = tombs
	for _, s := range byID {
		out.Snippets = append(out.Snippets, s)
	}
	sort.SliceStable(out.Snippets, func(i, j int) bool {
		return out.Snippets[i].UpdatedAt.After(out.Snippets[j].UpdatedAt)
	})
	return out, res
}

// marshal is shared by backends for consistent on-the-wire formatting.
func marshal(lib *model.Library) ([]byte, error) {
	return json.MarshalIndent(lib, "", "  ")
}

func unmarshal(b []byte) (*model.Library, error) {
	var lib model.Library
	if err := json.Unmarshal(b, &lib); err != nil {
		return nil, err
	}
	if lib.Snippets == nil {
		lib.Snippets = []*model.Snippet{}
	}
	return &lib, nil
}
