package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joshuademarco/glyph/internal/model"
	"github.com/joshuademarco/glyph/internal/store"
)

func seededModel(t *testing.T) Model {
	t.Helper()
	st := &store.Store{Lib: model.NewLibrary()}
	st.Lib.Add(&model.Snippet{
		Title: "Recreate one service without deps", Folder: "Docker/Compose", Lang: "sh",
		Tags: []string{"docker", "compose"}, Notes: "Force-recreate a single compose service.",
		Command: "docker compose up -d --no-deps --force-recreate {{service}}",
	})
	st.Lib.Add(&model.Snippet{
		Title: "Rollout restart a deployment", Folder: "Kubernetes/Workloads", Lang: "sh",
		Tags: []string{"k8s", "restart"}, Favorite: true,
		Command: "kubectl rollout restart deployment/{{name}} -n {{ns}}",
	})
	m := New(st)
	tm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	return tm.(Model)
}

func sendKey(m Model, s string) Model {
	var k tea.KeyMsg
	switch s {
	case "enter":
		k = tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		k = tea.KeyMsg{Type: tea.KeyTab}
	case "esc":
		k = tea.KeyMsg{Type: tea.KeyEsc}
	default:
		k = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	tm, _ := m.Update(k)
	return tm.(Model)
}

func TestBrowseRenders(t *testing.T) {
	m := seededModel(t)
	out := m.View()
	if !strings.Contains(out, "Folders") || !strings.Contains(out, "Preview") {
		t.Fatalf("browse view missing panes:\n%s", out)
	}
	if !strings.Contains(out, "NORMAL") {
		t.Fatalf("browse view missing status bar mode")
	}
}

func TestNavigationAndScreens(t *testing.T) {
	m := seededModel(t)

	// move focus to the list and select second snippet
	m = sendKey(m, "l") // folders -> list
	if m.focus != focusList {
		t.Fatalf("expected focus on list, got %v", m.focus)
	}
	m = sendKey(m, "j")

	// open palette
	m = sendKey(m, "/")
	if !m.paletteOpen {
		t.Fatal("palette did not open")
	}
	if out := m.View(); !strings.Contains(out, "SEARCH") {
		t.Fatalf("palette view missing SEARCH mode:\n%s", out)
	}
	m = sendKey(m, "esc")
	if m.paletteOpen {
		t.Fatal("palette did not close")
	}

	// open editor (new)
	m = sendKey(m, "n")
	if m.screen != screenEditor {
		t.Fatal("editor did not open on 'n'")
	}
	if out := m.View(); !strings.Contains(out, "Metadata") || !strings.Contains(out, "Command") {
		t.Fatalf("editor view missing panes:\n%s", out)
	}
	m = sendKey(m, "esc")
	if m.screen != screenBrowse {
		t.Fatal("esc did not return to browse")
	}

	// help
	m = sendKey(m, "?")
	if m.screen != screenHelp {
		t.Fatal("help did not open")
	}
	if out := m.View(); !strings.Contains(out, "yank") {
		t.Fatalf("help view missing content:\n%s", out)
	}
}

func TestEditorSavesNewSnippet(t *testing.T) {
	m := seededModel(t)
	before := len(m.st.Lib.Snippets)

	m = sendKey(m, "n") // new
	m.editor.title.SetValue("My new snippet")
	m.editor.command.SetValue("echo hello")
	tm, _ := m.saveEditor()
	m = tm.(Model)

	if got := len(m.st.Lib.Snippets); got != before+1 {
		t.Fatalf("expected %d snippets after save, got %d", before+1, got)
	}
	if m.screen != screenBrowse {
		t.Fatal("expected to return to browse after save")
	}
}

func TestDeleteSequence(t *testing.T) {
	m := seededModel(t)
	before := len(m.st.Lib.Snippets)
	m = sendKey(m, "l") // focus list
	m = sendKey(m, "d") // first d
	if !m.awaitDelD {
		t.Fatal("expected to await second d")
	}
	m = sendKey(m, "d") // second d -> delete
	if got := len(m.st.Lib.Snippets); got != before-1 {
		t.Fatalf("expected %d snippets after dd, got %d", before-1, got)
	}
}

func TestClickMovesFocusAndSelects(t *testing.T) {
	m := seededModel(t)
	g := browseLayout(m.width, m.height)

	// click inside the snippets list pane (right column, second row)
	m = m.handleClick(g.folderW+2, 2)
	if m.focus != focusList {
		t.Fatalf("click on list pane did not focus list, got %v", m.focus)
	}
	if m.listIdx != 1 {
		t.Fatalf("expected list selection 1 from click, got %d", m.listIdx)
	}

	// click the preview pane
	m = m.handleClick(g.folderW+2, g.listBoxH+2)
	if m.focus != focusPreview {
		t.Fatalf("click on preview did not focus preview, got %v", m.focus)
	}

	// click a folder row: rows are [All,Fav,Recent,Danger, divider, <group>, <folder>…]
	// body line 5 (y=6) is the first group; line 6 (y=7) is its first subfolder.
	m = m.handleClick(1, 6)
	if m.focus != focusFolders {
		t.Fatalf("click on folders did not focus folders, got %v", m.focus)
	}
	if got := m.folders[m.folderIdx].kind; got != "group" {
		t.Fatalf("expected a group selected, got kind %q", got)
	}
	m = m.handleClick(1, 7)
	cur := m.folders[m.folderIdx]
	if cur.kind != "folder" {
		t.Fatalf("expected a folder selected, got kind=%q", cur.kind)
	}
}

func TestFolderAutocomplete(t *testing.T) {
	m := seededModel(t)
	m.openEditor(nil)
	m.editor.field = fFolder // user tabs to the folder field
	m.editor.focus()

	m.editor.folder.SetValue("Doc")
	if got := m.editor.folderSuggestion(); got != "Docker" {
		t.Fatalf("expected suggestion Docker, got %q", got)
	}
	if !m.editor.acceptFolderSuggestion() || m.editor.folder.Value() != "Docker" {
		t.Fatalf("accept did not complete to Docker, got %q", m.editor.folder.Value())
	}

	m.editor.folder.SetValue("Docker/")
	if got := m.editor.folderSuggestion(); got != "Docker/Compose" {
		t.Fatalf("expected suggestion Docker/Compose, got %q", got)
	}

	// no spurious suggestion once the value is already a full candidate
	m.editor.folder.SetValue("Docker/Compose")
	if got := m.editor.folderSuggestion(); got != "" {
		t.Fatalf("expected no suggestion for exact match, got %q", got)
	}
}

func TestVariablesDetected(t *testing.T) {
	s := &model.Snippet{Command: "kubectl rollout restart deployment/{{name}} -n {{ns}}"}
	vars := s.Variables()
	if len(vars) != 2 || vars[0] != "name" || vars[1] != "ns" {
		t.Fatalf("unexpected vars: %v", vars)
	}
	got := s.Resolve(map[string]string{"name": "api", "ns": "prod"})
	if got != "kubectl rollout restart deployment/api -n prod" {
		t.Fatalf("resolve failed: %q", got)
	}
	_ = time.Now
}
