package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joshuademarco/glyph/internal/clipboard"
	"github.com/joshuademarco/glyph/internal/model"
	"github.com/joshuademarco/glyph/internal/store"
)

type screen int

const (
	screenBrowse screen = iota
	screenEditor
	screenHelp
)

type focusPane int

const (
	focusFolders focusPane = iota
	focusList
	focusPreview
)

// folderItem is a row in the left sidebar: a smart group, a user group section,
// or a folder. Groups are pure section headers and never contain snippets directly.
type folderItem struct {
	label     string
	icon      string
	kind      string // all | fav | recent | danger | group | folder
	folder    string // for "group": group name; for "folder": full "Group/Folder" path or single-segment
	count     int
	collapsed bool // for kind="group"
}

// Model is the root Bubble Tea model.
type Model struct {
	st     *store.Store
	width  int
	height int

	screen screen
	focus  focusPane

	folders   []folderItem
	folderIdx int
	collapsed map[string]bool // group name -> collapsed

	snippets []*model.Snippet
	listIdx  int

	// palette overlay
	paletteOpen bool
	palInput    textinput.Model
	palResults  []*model.Snippet
	palIdx      int

	editor editorModel

	status     string
	statusTime time.Time
	awaitDelD  bool // first 'd' of a 'dd' delete

	quitMsg string
}

// New constructs the root model over an open store.
func New(st *store.Store) Model {
	pi := textinput.New()
	pi.Prompt = ""
	pi.Placeholder = "fuzzy search…"
	pi.TextStyle = stFg
	pi.PlaceholderStyle = stFaint

	m := Model{
		st:        st,
		screen:    screenBrowse,
		focus:     focusFolders,
		palInput:  pi,
		editor:    newEditor(),
		collapsed: map[string]bool{},
	}
	m.rebuildFolders()
	m.refilter()
	return m
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

// rebuildFolders recomputes the sidebar list and its counts.
func (m *Model) rebuildFolders() {
	lib := m.st.Lib
	count := func(pred func(*model.Snippet) bool) int {
		n := 0
		for _, s := range lib.Snippets {
			if pred(s) {
				n++
			}
		}
		return n
	}
	items := []folderItem{
		{label: "All snippets", icon: "∗", kind: "all", count: len(lib.Snippets)},
		{label: "Favorites", icon: "★", kind: "fav", count: count(func(s *model.Snippet) bool { return s.Favorite })},
		{label: "Recently used", icon: "◷", kind: "recent", count: count(isRecent)},
		{label: "Dangerous", icon: "!", kind: "danger", count: count(func(s *model.Snippet) bool { return s.Dangerous })},
	}

	if m.collapsed == nil {
		m.collapsed = map[string]bool{}
	}

	// Partition folder paths into groups (with subfolders) and legacy
	// single-segment entries rendered at root.
	groupChildren := map[string]map[string]bool{}
	var ungrouped []string
	seenUngrouped := map[string]bool{}
	for _, s := range lib.Snippets {
		f := s.Folder
		if f == "" {
			continue
		}
		if i := strings.Index(f, "/"); i >= 0 {
			g, sub := f[:i], f[i+1:]
			if groupChildren[g] == nil {
				groupChildren[g] = map[string]bool{}
			}
			groupChildren[g][sub] = true
		} else if !seenUngrouped[f] {
			seenUngrouped[f] = true
			ungrouped = append(ungrouped, f)
		}
	}

	groupNames := make([]string, 0, len(groupChildren))
	for g := range groupChildren {
		groupNames = append(groupNames, g)
	}
	sort.Strings(groupNames)
	sort.Strings(ungrouped)

	for _, g := range groupNames {
		prefix := g + "/"
		gc := count(func(s *model.Snippet) bool {
			return s.Folder == g || strings.HasPrefix(s.Folder, prefix)
		})
		collapsed := m.collapsed[g]
		items = append(items, folderItem{
			label: g, kind: "group", folder: g, count: gc, collapsed: collapsed,
		})
		if collapsed {
			continue
		}
		subs := make([]string, 0, len(groupChildren[g]))
		for sub := range groupChildren[g] {
			subs = append(subs, sub)
		}
		sort.Strings(subs)
		for _, sub := range subs {
			full := g + "/" + sub
			cc := count(func(s *model.Snippet) bool { return s.Folder == full })
			items = append(items, folderItem{
				label: sub, kind: "folder", folder: full, count: cc,
			})
		}
	}
	for _, u := range ungrouped {
		c := count(func(s *model.Snippet) bool { return s.Folder == u })
		items = append(items, folderItem{label: u, kind: "folder", folder: u, count: c})
	}

	m.folders = items
	if m.folderIdx >= len(items) {
		m.folderIdx = len(items) - 1
	}
	if m.folderIdx < 0 {
		m.folderIdx = 0
	}
}

func isRecent(s *model.Snippet) bool {
	return s.LastUsed != nil && time.Since(*s.LastUsed) < 14*24*time.Hour
}

// refilter recomputes the snippet list for the selected sidebar item.
func (m *Model) refilter() {
	if len(m.folders) == 0 {
		m.snippets = nil
		return
	}
	fi := m.folders[m.folderIdx]
	var out []*model.Snippet
	for _, s := range m.st.Lib.Snippets {
		keep := false
		switch fi.kind {
		case "all":
			keep = true
		case "fav":
			keep = s.Favorite
		case "recent":
			keep = isRecent(s)
		case "danger":
			keep = s.Dangerous
		case "group":
			keep = s.Folder == fi.folder || strings.HasPrefix(s.Folder, fi.folder+"/")
		case "folder":
			keep = s.Folder == fi.folder
		}
		if keep {
			out = append(out, s)
		}
	}
	// recent first
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].UpdatedAt.After(out[i].UpdatedAt) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	m.snippets = out
	if m.listIdx >= len(out) {
		m.listIdx = len(out) - 1
	}
	if m.listIdx < 0 {
		m.listIdx = 0
	}
}

// toggleGroup flips the collapsed state of a group and rebuilds the sidebar,
// keeping selection anchored on the same group row.
func (m *Model) toggleGroup(group string) {
	if m.collapsed == nil {
		m.collapsed = map[string]bool{}
	}
	m.collapsed[group] = !m.collapsed[group]
	m.rebuildFolders()
	for i, f := range m.folders {
		if f.kind == "group" && f.folder == group {
			m.folderIdx = i
			break
		}
	}
	m.refilter()
}

func (m *Model) selected() *model.Snippet {
	if m.listIdx >= 0 && m.listIdx < len(m.snippets) {
		return m.snippets[m.listIdx]
	}
	return nil
}

func (m *Model) flash(format string, a ...any) {
	m.status = fmt.Sprintf(format, a...)
	m.statusTime = time.Now()
}

// Update is the Bubble Tea event loop.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.editor.width, m.editor.height = msg.Width, msg.Height
		return m, nil

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			return m.handleClick(msg.X, msg.Y), nil
		}
		return m, nil

	case runFinishedMsg:
		if msg.err != nil {
			m.flash("run failed: %v", msg.err)
		} else {
			m.flash("ran ✓")
		}
		return m, nil

	case tea.KeyMsg:
		if m.paletteOpen {
			return m.updatePalette(msg)
		}
		switch m.screen {
		case screenEditor:
			return m.updateEditor(msg)
		case screenHelp:
			if msg.String() == "?" || msg.String() == "esc" || msg.String() == "q" {
				m.screen = screenBrowse
			}
			return m, nil
		default:
			return m.updateBrowse(msg)
		}
	}
	return m, nil
}

func (m Model) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	// 'dd' delete sequence
	if m.awaitDelD {
		m.awaitDelD = false
		if k == "d" {
			return m.deleteSelected()
		}
	}

	switch k {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "?":
		m.screen = screenHelp
		return m, nil
	case "/", "ctrl+p":
		m.openPalette()
		return m, nil
	case "tab", "l", "right":
		m.focus = (m.focus + 1) % 3
		return m, nil
	case "shift+tab", "h", "left":
		m.focus = (m.focus + 2) % 3
		return m, nil
	case "j", "down":
		m.move(1)
		return m, nil
	case "k", "up":
		m.move(-1)
		return m, nil
	case "g":
		m.set(0)
		return m, nil
	case "G":
		m.set(1 << 30)
		return m, nil
	case "enter":
		if m.focus == focusFolders {
			m.focus = focusList
			return m, nil
		}
		return m.yankSelected()
	case " ":
		if m.focus == focusFolders && len(m.folders) > 0 {
			cur := m.folders[m.folderIdx]
			if cur.kind == "group" {
				m.toggleGroup(cur.folder)
			}
		}
		return m, nil
	case "y":
		return m.yankSelected()
	case "e":
		if s := m.selected(); s != nil {
			m.openEditor(s)
		}
		return m, nil
	case "n":
		m.openEditor(nil)
		return m, nil
	case "f":
		if s := m.selected(); s != nil {
			s.Favorite = !s.Favorite
			s.UpdatedAt = time.Now().UTC()
			_ = m.st.Save()
			m.rebuildFolders()
			m.refilter()
			m.flash("favorite: %v", s.Favorite)
		}
		return m, nil
	case "x":
		return m.runSelected()
	case "d":
		m.awaitDelD = true
		return m, nil
	}
	return m, nil
}

func (m *Model) move(d int) {
	switch m.focus {
	case focusFolders:
		m.folderIdx = clamp(m.folderIdx+d, 0, len(m.folders)-1)
		m.listIdx = 0
		m.refilter()
	case focusList:
		m.listIdx = clamp(m.listIdx+d, 0, len(m.snippets)-1)
	}
}

func (m *Model) set(idx int) {
	switch m.focus {
	case focusFolders:
		m.folderIdx = clamp(idx, 0, len(m.folders)-1)
		m.refilter()
	case focusList:
		m.listIdx = clamp(idx, 0, len(m.snippets)-1)
	}
}

func (m Model) yankSelected() (tea.Model, tea.Cmd) {
	s := m.selected()
	if s == nil {
		return m, nil
	}
	if err := clipboard.Copy(s.Command); err != nil {
		m.flash("copy failed: %v", err)
		return m, nil
	}
	s.MarkUsed(time.Now().UTC())
	_ = m.st.Save()
	m.flash("yanked %q to clipboard", short(s.Title, 28))
	return m, nil
}

func (m Model) deleteSelected() (tea.Model, tea.Cmd) {
	s := m.selected()
	if s == nil {
		return m, nil
	}
	m.st.Lib.Remove(s.ID)
	_ = m.st.Save()
	m.rebuildFolders()
	m.refilter()
	m.flash("deleted %q", short(s.Title, 28))
	return m, nil
}

type runFinishedMsg struct{ err error }

func (m Model) runSelected() (tea.Model, tea.Cmd) {
	s := m.selected()
	if s == nil {
		return m, nil
	}
	if len(s.Variables()) > 0 {
		m.flash("has variables — use `glyph run` in the CLI to fill them")
		return m, nil
	}
	s.MarkUsed(time.Now().UTC())
	_ = m.st.Save()
	c := shellCommand(s.Command)
	return m, tea.ExecProcess(c, func(err error) tea.Msg { return runFinishedMsg{err} })
}

func shellCommand(body string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", body)
	}
	sh := "/bin/sh"
	return exec.Command(sh, "-c", body)
}

// --- palette ---

func (m *Model) openPalette() {
	m.paletteOpen = true
	m.palInput.SetValue("")
	m.palInput.Focus()
	m.palResults = m.st.Search("")
	m.palIdx = 0
}

func (m Model) updatePalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.paletteOpen = false
		m.palInput.Blur()
		return m, nil
	case "up", "ctrl+k":
		m.palIdx = clamp(m.palIdx-1, 0, len(m.palResults)-1)
		return m, nil
	case "down", "ctrl+j":
		m.palIdx = clamp(m.palIdx+1, 0, len(m.palResults)-1)
		return m, nil
	case "ctrl+n":
		m.paletteOpen = false
		m.palInput.Blur()
		m.openEditor(nil)
		m.editor.title.SetValue(m.palInput.Value())
		return m, nil
	case "enter", "tab":
		if m.palIdx >= 0 && m.palIdx < len(m.palResults) {
			s := m.palResults[m.palIdx]
			_ = clipboard.Copy(s.Command)
			s.MarkUsed(time.Now().UTC())
			_ = m.st.Save()
			m.paletteOpen = false
			m.palInput.Blur()
			m.flash("yanked %q", short(s.Title, 28))
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.palInput, cmd = m.palInput.Update(msg)
	m.palResults = m.st.Search(m.palInput.Value())
	m.palIdx = clamp(m.palIdx, 0, max(0, len(m.palResults)-1))
	return m, cmd
}

// --- editor delegation ---

func (m Model) updateEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	// → accepts the folder ghost completion when the cursor is at the end.
	if k == "right" && m.editor.field == fFolder &&
		m.editor.folder.Position() >= len([]rune(m.editor.folder.Value())) {
		if m.editor.acceptFolderSuggestion() {
			return m, nil
		}
	}

	switch k {
	case "esc":
		m.screen = screenBrowse
		return m, nil
	case "ctrl+s":
		return m.saveEditor()
	case "tab":
		// On the folder field, Tab first completes the suggestion; with nothing
		// left to complete it advances to the next field.
		if m.editor.acceptFolderSuggestion() {
			return m, nil
		}
		m.editor.next(1)
		return m, nil
	case "shift+tab":
		m.editor.next(-1)
		return m, nil
	}
	var cmd tea.Cmd
	m.editor, cmd = m.editor.update(msg)
	return m, cmd
}

// openEditor loads a snippet (nil = new) into the editor and refreshes the
// folder autocomplete candidates.
func (m *Model) openEditor(s *model.Snippet) {
	m.editor.folderCands = m.st.Lib.FolderCandidates()
	m.editor.load(s)
	m.screen = screenEditor
}

func (m Model) saveEditor() (tea.Model, tea.Cmd) {
	prev := m.st.Lib.Find(m.editor.editingID)
	s := m.editor.snippet(prev)
	if strings.TrimSpace(s.Title) == "" {
		m.flash("title is required")
		return m, nil
	}
	if prev != nil {
		*prev = *s
		if prev.LooksDangerous() {
			prev.Dangerous = true
		}
	} else {
		m.st.Lib.Add(s)
	}
	_ = m.st.Save()
	m.rebuildFolders()
	m.refilter()
	m.screen = screenBrowse
	m.flash("saved %q", short(s.Title, 28))
	return m, nil
}

// View renders the active screen.
func (m Model) View() string {
	if m.width == 0 {
		return "starting glyph…"
	}
	if m.paletteOpen {
		return m.viewPalette()
	}
	switch m.screen {
	case screenEditor:
		return m.viewEditor()
	case screenHelp:
		return m.viewHelp()
	default:
		return m.viewBrowse()
	}
}

// browseGeo captures the browse screen's pane geometry so the renderer and the
// mouse handler agree on where each pane is.
type browseGeo struct {
	folderW, rightW, contentH, listBoxH, previewBoxH int
}

func browseLayout(w, h int) browseGeo {
	contentH := h - 1
	if contentH < 6 {
		contentH = 6
	}
	folderW := 30
	if w < 96 {
		folderW = w / 4
	}
	if folderW < 20 {
		folderW = 20
	}
	rightW := w - folderW - 1
	listBoxH := (contentH - 1) / 2
	previewBoxH := contentH - 1 - listBoxH
	return browseGeo{folderW, rightW, contentH, listBoxH, previewBoxH}
}

// folderRowToIdx maps each rendered sidebar body line to a folder index, or -1
// for the non-selectable "Folders" divider — mirroring folderBody's layout.
func (m Model) folderRowToIdx() []int {
	var rows []int
	isFolderish := func(k string) bool { return k == "group" || k == "folder" }
	for i, f := range m.folders {
		if isFolderish(f.kind) && (i == 0 || !isFolderish(m.folders[i-1].kind)) {
			rows = append(rows, -1)
		}
		rows = append(rows, i)
	}
	return rows
}

// handleClick moves focus (and selects a row) based on which pane was clicked.
func (m Model) handleClick(x, y int) Model {
	if m.paletteOpen {
		return m
	}
	if m.screen == screenEditor {
		if f, ok := m.editor.hitField(x, y); ok {
			m.editor.field = f
			m.editor.focus()
		}
		return m
	}
	if m.screen != screenBrowse {
		return m
	}
	g := browseLayout(m.width, m.height)
	if y >= g.contentH {
		return m // status bar
	}

	// folders pane (left column)
	if x < g.folderW {
		m.focus = focusFolders
		row := y - 1 // skip top border
		rmap := m.folderRowToIdx()
		if row >= 0 && row < len(rmap) && rmap[row] >= 0 {
			m.folderIdx = rmap[row]
			m.listIdx = 0
			m.refilter()
		}
		return m
	}

	// right column
	if x >= g.folderW+1 {
		switch {
		case y < g.listBoxH: // snippets list pane
			m.focus = focusList
			row := y - 1
			if row >= 0 && row < len(m.snippets) {
				m.listIdx = row
			}
		case y >= g.listBoxH+1 && y < g.contentH: // preview pane
			m.focus = focusPreview
		}
	}
	return m
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func short(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

var _ = lipgloss.JoinHorizontal
