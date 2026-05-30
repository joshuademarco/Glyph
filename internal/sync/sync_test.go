package sync

import (
	"testing"
	"time"

	"github.com/joshuademarco/glyph/internal/model"
)

func snip(id string, updated time.Time) *model.Snippet {
	return &model.Snippet{ID: id, Title: id, Command: "echo " + id, UpdatedAt: updated}
}

// A snippet deleted locally must not be re-pulled from a remote that still has it.
func TestMergeDeletionPropagates(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	delAt := t0.Add(time.Hour)

	local := model.NewLibrary()
	local.Deleted["a"] = delAt // deleted locally after t0

	remote := model.NewLibrary()
	remote.Snippets = []*model.Snippet{snip("a", t0)} // remote still has the old copy

	out, res := merge(local, remote)

	if got := len(out.Snippets); got != 0 {
		t.Fatalf("deleted snippet was reintroduced: got %d snippets, want 0", got)
	}
	if res.Deleted != 1 {
		t.Fatalf("Result.Deleted = %d, want 1", res.Deleted)
	}
	if _, ok := out.Deleted["a"]; !ok {
		t.Fatalf("tombstone for %q was dropped; it must persist to propagate", "a")
	}
}

// An edit newer than the deletion resurrects the snippet (last-write-wins).
func TestMergeEditAfterDeleteResurrects(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	delAt := t0.Add(time.Hour)
	editAt := delAt.Add(time.Hour)

	local := model.NewLibrary()
	local.Deleted["a"] = delAt

	remote := model.NewLibrary()
	remote.Snippets = []*model.Snippet{snip("a", editAt)} // edited after the deletion

	out, _ := merge(local, remote)

	if len(out.Snippets) != 1 {
		t.Fatalf("edit after delete should resurrect: got %d snippets, want 1", len(out.Snippets))
	}
	if _, ok := out.Deleted["a"]; ok {
		t.Fatalf("stale tombstone should be dropped once the snippet is resurrected")
	}
}

// Normal union still pulls genuinely new remote snippets.
func TestMergePullsNewRemote(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	local := model.NewLibrary()
	local.Snippets = []*model.Snippet{snip("a", t0)}

	remote := model.NewLibrary()
	remote.Snippets = []*model.Snippet{snip("a", t0), snip("b", t0)}

	out, res := merge(local, remote)

	if len(out.Snippets) != 2 {
		t.Fatalf("got %d snippets, want 2", len(out.Snippets))
	}
	if res.Pulled != 1 {
		t.Fatalf("Result.Pulled = %d, want 1", res.Pulled)
	}
}

// A remote tombstone deletes a snippet still present locally.
func TestMergeRemoteTombstoneDeletesLocal(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	delAt := t0.Add(time.Hour)

	local := model.NewLibrary()
	local.Snippets = []*model.Snippet{snip("a", t0)}

	remote := model.NewLibrary()
	remote.Deleted["a"] = delAt

	out, res := merge(local, remote)

	if len(out.Snippets) != 0 {
		t.Fatalf("remote deletion did not remove local snippet: got %d, want 0", len(out.Snippets))
	}
	if res.Deleted != 1 {
		t.Fatalf("Result.Deleted = %d, want 1", res.Deleted)
	}
}
