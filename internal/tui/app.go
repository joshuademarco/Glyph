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
	"github.com/joshuademarco/glyph/internal/config"
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

// sortMode controls the ordering of the snippet list.
type sortMode int

const (
	sortRecent sortMode = iota
	sortName
	sortUses
)

func (s sortMode) label() string {
	switch s {
	case sortName:
		return "name"
	case sortUses:
		return "uses"
	default:
		return "recent"
	}
}

// folderItem is a row in the left sidebar: a smart group, a user group section,
// a folder, or a tag. Groups are pure section headers and never contain snippets directly.
type folderItem struct {
	label     string
	icon      string
	kind      string // all | fav | recent | danger | group | folder | tag
	folder    string // for "group": group name; for "folder": full path; for "tag": tag name
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
	sortMode sortMode

	// multi-select: snippet ID -> marked
	marked map[string]bool

	// undo buffer for the most recent delete
	lastDeleted []*model.Snippet

	// palette overlay
	paletteOpen bool
	palScoped   bool
	palInput    textinput.Model
	palResults  []*model.Snippet
	palIdx      int

	// variable-fill overlay
	varsOpen   bool
	varSnippet *model.Snippet
	varNames   []string
	varInputs  []textinput.Model
	varIdx     int
	varAction  varAction

	// single-line prompt overlay (move folder / add tag)
	promptOpen   bool
	promptLabel  string
	promptInput  textinput.Model
	promptAction promptAction

	// confirmation overlay
	confirmOpen    bool
	confirmPrompt  string
	confirmAction  confirmAction
	confirmTargets []*model.Snippet
	confirmRun     *model.Snippet

	editor editorModel

	status     string
	statusTime time.Time
	busy       string // non-empty while an async action (sync/share) is in flight
	awaitDelD  bool   // first 'd' of a 'dd' delete

	quitMsg string
}

type varAction int

const (
	varYank varAction = iota
	varRun
)

type confirmAction int

const (
	confirmNone confirmAction = iota
	confirmDelete
	confirmBulkDelete
	confirmRunDanger
)

type promptAction int

const (
	promptNone promptAction = iota
	promptMoveFolder
	promptAddTag
)

// New constructs the root model over an open store.
func New(st *store.Store) Model {
	if cfg, err := config.Load(); err == nil && cfg != nil {
		applyTheme(cfg.Theme)
	}

	pi := textinput.New()
	pi.Prompt = ""
	pi.Placeholder = "fuzzy search…"
	pi.TextStyle = stFg
	pi.PlaceholderStyle = stFaint

	pr := textinput.New()
	pr.Prompt = ""
	pr.TextStyle = stFg
	pr.PlaceholderStyle = stFaint

	m := Model{
		st:          st,
		screen:      screenBrowse,
		focus:       focusFolders,
		palInput:    pi,
		promptInput: pr,
		editor:      newEditor(),
		collapsed:   map[string]bool{},
		marked:      map[string]bool{},
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

	// Tags section, mirroring the folder tree as a flat list.
	for _, tag := range lib.Tags() {
		t := tag
		c := count(func(s *model.Snippet) bool { return snippetHasTag(s, t) })
		items = append(items, folderItem{label: t, kind: "tag", folder: t, count: c})
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

func snippetHasTag(s *model.Snippet, tag string) bool {
	tag = strings.ToLower(strings.TrimPrefix(tag, "#"))
	for _, t := range s.Tags {
		if strings.ToLower(t) == tag {
			return true
		}
	}
	return false
}

// currentScopePred returns a predicate matching the snippets under the selected
// sidebar item. Shared by refilter and the scoped palette.
func (m Model) currentScopePred() func(*model.Snippet) bool {
	if len(m.folders) == 0 {
		return func(*model.Snippet) bool { return true }
	}
	fi := m.folders[m.folderIdx]
	switch fi.kind {
	case "fav":
		return func(s *model.Snippet) bool { return s.Favorite }
	case "recent":
		return isRecent
	case "danger":
		return func(s *model.Snippet) bool { return s.Dangerous }
	case "group":
		return func(s *model.Snippet) bool {
			return s.Folder == fi.folder || strings.HasPrefix(s.Folder, fi.folder+"/")
		}
	case "folder":
		return func(s *model.Snippet) bool { return s.Folder == fi.folder }
	case "tag":
		return func(s *model.Snippet) bool { return snippetHasTag(s, fi.folder) }
	default: // all
		return func(*model.Snippet) bool { return true }
	}
}

// refilter recomputes the snippet list for the selected sidebar item.
func (m *Model) refilter() {
	pred := m.currentScopePred()
	var out []*model.Snippet
	for _, s := range m.st.Lib.Snippets {
		if pred(s) {
			out = append(out, s)
		}
	}
	m.sortSnippets(out)
	m.snippets = out
	if m.listIdx >= len(out) {
		m.listIdx = len(out) - 1
	}
	if m.listIdx < 0 {
		m.listIdx = 0
	}
}

func (m *Model) sortSnippets(out []*model.Snippet) {
	switch m.sortMode {
	case sortName:
		sort.SliceStable(out, func(i, j int) bool {
			return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
		})
	case sortUses:
		sort.SliceStable(out, func(i, j int) bool { return out[i].UseCount > out[j].UseCount })
	default:
		sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
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

// targets returns the marked snippets, or the single selected one when nothing
// is marked.
func (m Model) targets() []*model.Snippet {
	if len(m.marked) > 0 {
		var out []*model.Snippet
		for _, s := range m.st.Lib.Snippets {
			if m.marked[s.ID] {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	if s := m.selected(); s != nil {
		return []*model.Snippet{s}
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

	case syncDoneMsg:
		m.busy = ""
		if msg.err != nil {
			m.flash("sync failed: %v", msg.err)
		} else {
			m.rebuildFolders()
			m.refilter()
			if msg.res != nil {
				m.flash("synced: %d pulled, %d pushed", msg.res.Pulled, msg.res.Pushed)
			} else {
				m.flash("synced ✓")
			}
		}
		return m, nil

	case shareDoneMsg:
		m.busy = ""
		if msg.err != nil {
			m.flash("share failed: %v", msg.err)
		} else {
			_ = clipboard.Copy(msg.url)
			m.flash("shared → %s (url copied)", msg.url)
		}
		return m, nil

	case externalEditMsg:
		if msg.err != nil {
			m.flash("editor: %v", msg.err)
		} else if msg.ok {
			m.editor.command.SetValue(msg.content)
			m.flash("loaded from external editor")
		}
		return m, nil

	case tea.KeyMsg:
		if m.paletteOpen {
			return m.updatePalette(msg)
		}
		if m.varsOpen {
			return m.updateVars(msg)
		}
		if m.promptOpen {
			return m.updatePrompt(msg)
		}
		if m.confirmOpen {
			return m.updateConfirm(msg)
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
			return m, nil
		}
		if m.focus == focusList {
			if s := m.selected(); s != nil {
				if m.marked[s.ID] {
					delete(m.marked, s.ID)
				} else {
					m.marked[s.ID] = true
				}
				m.move(1)
			}
		}
		return m, nil
	case "esc":
		if len(m.marked) > 0 {
			m.marked = map[string]bool{}
			m.flash("selection cleared")
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
	case "c":
		m.duplicateSelected()
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
	case "s":
		m.sortMode = (m.sortMode + 1) % 3
		m.refilter()
		m.flash("sort: %s", m.sortMode.label())
		return m, nil
	case "m":
		if t := m.targets(); len(t) > 0 {
			init := ""
			if len(t) == 1 {
				init = t[0].Folder
			}
			m.openPrompt(promptMoveFolder, "Move to folder: ", init)
		}
		return m, nil
	case "t":
		if len(m.targets()) > 0 {
			m.openPrompt(promptAddTag, "Add tag: ", "")
		}
		return m, nil
	case "u":
		m.undoDelete()
		return m, nil
	case "S":
		if m.busy != "" {
			return m, nil
		}
		m.busy = "syncing…"
		m.flash("syncing…")
		return m, m.syncCmd()
	case "P":
		s := m.selected()
		if s == nil {
			return m, nil
		}
		if m.busy != "" {
			return m, nil
		}
		m.busy = "sharing…"
		m.flash("sharing to gist…")
		return m, m.shareCmd(s)
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
	if len(s.Variables()) > 0 {
		(&m).openVarOverlay(s, varYank)
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

// duplicateSelected clones the selected snippet under a new id and "(copy)" title.
func (m *Model) duplicateSelected() {
	s := m.selected()
	if s == nil {
		return
	}
	c := *s
	c.ID = ""
	c.Title = s.Title + " (copy)"
	c.UseCount = 0
	c.LastUsed = nil
	c.Favorite = false
	c.CreatedAt = time.Time{}
	c.Tags = append([]string(nil), s.Tags...)
	m.st.Lib.Add(&c)
	_ = m.st.Save()
	m.rebuildFolders()
	m.refilter()
	for i, sn := range m.snippets {
		if sn.ID == c.ID {
			m.listIdx = i
			break
		}
	}
	m.flash("duplicated %q", short(s.Title, 24))
}

func (m *Model) undoDelete() {
	if len(m.lastDeleted) == 0 {
		m.flash("nothing to undo")
		return
	}
	now := time.Now().UTC()
	m.st.Lib.Snippets = append(m.st.Lib.Snippets, m.lastDeleted...)
	m.st.Lib.UpdatedAt = now
	n := len(m.lastDeleted)
	m.lastDeleted = nil
	_ = m.st.Save()
	m.rebuildFolders()
	m.refilter()
	m.flash("restored %d snippet(s)", n)
}

// deleteSelected opens a confirmation for the current target set (marked or
// selected). The actual removal happens in updateConfirm.
func (m Model) deleteSelected() (tea.Model, tea.Cmd) {
	t := m.targets()
	if len(t) == 0 {
		return m, nil
	}
	m.confirmTargets = t
	if len(t) == 1 {
		m.openConfirm(confirmDelete, fmt.Sprintf("delete %q?", short(t[0].Title, 28)))
	} else {
		m.openConfirm(confirmBulkDelete, fmt.Sprintf("delete %d snippets?", len(t)))
	}
	return m, nil
}

func (m *Model) performDelete(t []*model.Snippet) {
	m.lastDeleted = append([]*model.Snippet(nil), t...)
	for _, s := range t {
		m.st.Lib.Remove(s.ID)
	}
	m.marked = map[string]bool{}
	_ = m.st.Save()
	m.rebuildFolders()
	m.refilter()
	m.flash("deleted %d snippet(s) — u to undo", len(t))
}

type runFinishedMsg struct{ err error }

func (m Model) runSelected() (tea.Model, tea.Cmd) {
	s := m.selected()
	if s == nil {
		return m, nil
	}
	if s.Dangerous {
		m.confirmRun = s
		m.openConfirm(confirmRunDanger, fmt.Sprintf("⚠ %q looks destructive — run it?", short(s.Title, 24)))
		return m, nil
	}
	return m.doRun(s)
}

// doRun fills variables (if any) then executes the snippet body in the shell.
func (m Model) doRun(s *model.Snippet) (tea.Model, tea.Cmd) {
	if len(s.Variables()) > 0 {
		(&m).openVarOverlay(s, varRun)
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
	m.palResults = m.scopedSearch("")
	m.palIdx = 0
}

// scopedSearch runs the fuzzy search, optionally narrowed to the current
// sidebar scope.
func (m Model) scopedSearch(q string) []*model.Snippet {
	res := m.st.Search(q)
	if !m.palScoped {
		return res
	}
	pred := m.currentScopePred()
	out := make([]*model.Snippet, 0, len(res))
	for _, s := range res {
		if pred(s) {
			out = append(out, s)
		}
	}
	return out
}

func (m Model) updatePalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.paletteOpen = false
		m.palInput.Blur()
		return m, nil
	case "ctrl+f":
		m.palScoped = !m.palScoped
		m.palResults = m.scopedSearch(m.palInput.Value())
		m.palIdx = clamp(m.palIdx, 0, max(0, len(m.palResults)-1))
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
			m.paletteOpen = false
			m.palInput.Blur()
			if len(s.Variables()) > 0 {
				// jump to the list selection then fill variables
				m.selectByID(s.ID)
				(&m).openVarOverlay(s, varYank)
				return m, nil
			}
			_ = clipboard.Copy(s.Command)
			s.MarkUsed(time.Now().UTC())
			_ = m.st.Save()
			m.flash("yanked %q", short(s.Title, 28))
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.palInput, cmd = m.palInput.Update(msg)
	m.palResults = m.scopedSearch(m.palInput.Value())
	m.palIdx = clamp(m.palIdx, 0, max(0, len(m.palResults)-1))
	return m, cmd
}

// selectByID best-effort moves the list cursor to the snippet with the given id
// within the current filter (used after palette actions).
func (m *Model) selectByID(id string) {
	for i, s := range m.snippets {
		if s.ID == id {
			m.listIdx = i
			return
		}
	}
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
	case "ctrl+e":
		// shell out to $EDITOR on the command body
		return m, m.externalEditorCmd(m.editor.command.Value())
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
	if m.varsOpen {
		return m.viewVars()
	}
	if m.promptOpen {
		return m.viewPrompt()
	}
	if m.confirmOpen {
		return m.viewConfirm()
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

// dividerBefore returns the section-divider label to draw before sidebar row i,
// or "" for none. Shared by folderBody (render) and folderRowToIdx (clicks).
func (m Model) dividerBefore(i int) string {
	isFolderish := func(k string) bool { return k == "group" || k == "folder" }
	cur := m.folders[i].kind
	prev := ""
	if i > 0 {
		prev = m.folders[i-1].kind
	}
	if isFolderish(cur) && !isFolderish(prev) {
		return "Folders"
	}
	if cur == "tag" && prev != "tag" {
		return "Tags"
	}
	return ""
}

// folderRowToIdx maps each rendered sidebar body line to a folder index, or -1
// for a non-selectable divider — mirroring folderBody's layout.
func (m Model) folderRowToIdx() []int {
	var rows []int
	for i := range m.folders {
		if m.dividerBefore(i) != "" {
			rows = append(rows, -1)
		}
		rows = append(rows, i)
	}
	return rows
}

// handleClick moves focus (and selects a row) based on which pane was clicked.
func (m Model) handleClick(x, y int) Model {
	if m.paletteOpen || m.varsOpen || m.promptOpen || m.confirmOpen {
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
