package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joshuademarco/glyph/internal/model"
)

type editorField int

const (
	fTitle editorField = iota
	fFolder
	fTags
	fLang
	fNotes
	fCommand
	fieldCount
)

// editorModel is the modal create/edit form from screen-editor.jsx.
type editorModel struct {
	editingID string // empty => creating new
	title     textinput.Model
	folder    textinput.Model
	tags      textinput.Model
	lang      textinput.Model
	notes     textinput.Model
	command   textarea.Model
	field     editorField
	width     int
	height    int

	folderCands []string // existing folders + ancestors, for autocomplete
}

func newEditor() editorModel {
	mk := func(ph string) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = ph
		ti.Prompt = ""
		ti.TextStyle = stFg
		ti.PlaceholderStyle = stFaint
		return ti
	}
	ta := textarea.New()
	ta.Placeholder = "command or code…  use {{name}} for variables"
	ta.ShowLineNumbers = true
	ta.Prompt = ""
	e := editorModel{
		title:   mk("Short, action-first title"),
		folder:  mk("Folder / Subfolder"),
		tags:    mk("comma,separated,tags"),
		lang:    mk("sh · yml · sql · py …"),
		notes:   mk("What it does, when to use it"),
		command: ta,
	}
	return e
}

// load populates the form from an existing snippet (or a blank for new).
func (e *editorModel) load(s *model.Snippet) {
	if s == nil {
		e.editingID = ""
		e.title.SetValue("")
		e.folder.SetValue("")
		e.tags.SetValue("")
		e.lang.SetValue("sh")
		e.notes.SetValue("")
		e.command.SetValue("")
	} else {
		e.editingID = s.ID
		e.title.SetValue(s.Title)
		e.folder.SetValue(s.Folder)
		e.tags.SetValue(strings.Join(s.Tags, ", "))
		e.lang.SetValue(s.Lang)
		e.notes.SetValue(s.Notes)
		e.command.SetValue(s.Command)
	}
	e.field = fTitle
	e.focus()
}

// snippet builds a snippet from the current form values, preserving identity
// and usage stats of the snippet being edited.
func (e *editorModel) snippet(prev *model.Snippet) *model.Snippet {
	var tags []string
	for _, t := range strings.Split(e.tags.Value(), ",") {
		t = strings.TrimSpace(strings.TrimPrefix(t, "#"))
		if t != "" {
			tags = append(tags, t)
		}
	}
	s := &model.Snippet{
		ID:      e.editingID,
		Title:   strings.TrimSpace(e.title.Value()),
		Folder:  strings.Trim(strings.TrimSpace(e.folder.Value()), "/"),
		Tags:    tags,
		Lang:    strings.TrimSpace(e.lang.Value()),
		Notes:   strings.TrimSpace(e.notes.Value()),
		Command: e.command.Value(),
	}
	if prev != nil {
		s.CreatedAt = prev.CreatedAt
		s.UseCount = prev.UseCount
		s.LastUsed = prev.LastUsed
		s.Favorite = prev.Favorite
		s.Source = prev.Source
	}
	s.UpdatedAt = time.Now().UTC()
	return s
}

func (e *editorModel) inputs() []*textinput.Model {
	return []*textinput.Model{&e.title, &e.folder, &e.tags, &e.lang, &e.notes}
}

// focus directs keyboard input to the active field.
func (e *editorModel) focus() {
	for _, in := range e.inputs() {
		in.Blur()
	}
	e.command.Blur()
	switch e.field {
	case fCommand:
		e.command.Focus()
	default:
		e.inputs()[int(e.field)].Focus()
	}
}

func (e *editorModel) next(delta int) {
	e.field = editorField((int(e.field) + delta + int(fieldCount)) % int(fieldCount))
	e.focus()
}

func (e editorModel) update(msg tea.Msg) (editorModel, tea.Cmd) {
	var cmd tea.Cmd
	if e.field == fCommand {
		e.command, cmd = e.command.Update(msg)
		return e, cmd
	}
	in := e.inputs()[int(e.field)]
	*in, cmd = in.Update(msg)
	return e, cmd
}

// view renders the editor screen body (without the status bar) at the given
// content size.
func (e editorModel) view(width, contentH int) string {
	leftW := width - 30
	if leftW < 30 {
		leftW = width
	}

	field := func(label string, in textinput.Model, active bool) string {
		l := stDim.Render(cell(label, 8))
		v := in.View()
		mark := "  "
		if active {
			mark = stBlue.Render("▸ ")
		}
		return mark + l + " " + v
	}

	// folder field gets an inline ghost completion appended after the input.
	folderField := func() string {
		mark := "  "
		if e.field == fFolder {
			mark = stBlue.Render("▸ ")
		}
		l := stDim.Render(cell("folder", 8))
		v := e.folder.View()
		if e.field == fFolder {
			if sug := e.folderSuggestion(); sug != "" {
				cur := e.folder.Value()
				if len(sug) > len(cur) {
					v += stFaint.Render(sug[len(cur):])
				}
			}
		}
		return mark + l + " " + v
	}

	// Metadata pane
	metaInner := []string{
		field("title", e.title, e.field == fTitle),
		folderField(),
		field("tags", e.tags, e.field == fTags),
		field("lang", e.lang, e.field == fLang),
	}
	if e.field == fFolder {
		if matches := e.folderMatches(5); len(matches) > 0 {
			metaInner = append(metaInner,
				stFaint.Render("           ⇥ or → to complete:"),
				stDim.Render("           "+strings.Join(matches, "  ")))
		}
	}
	meta := pane("Metadata", "", metaInner, leftW-2, len(metaInner)+1, colBlue, e.field <= fLang)

	// Notes pane
	notesInner := []string{
		(func() string {
			mark := "  "
			if e.field == fNotes {
				mark = stBlue.Render("▸ ")
			}
			return mark + e.notes.View()
		})(),
	}
	notes := pane("Notes", "", notesInner, leftW-2, 2, colBlue, e.field == fNotes)

	// Command editor pane (fills remaining height)
	usedH := lipgloss.Height(meta) + lipgloss.Height(notes) + 1
	cmdBoxH := contentH - usedH
	if cmdBoxH < 4 {
		cmdBoxH = 4
	}
	e.command.SetWidth(leftW - 4)
	e.command.SetHeight(cmdBoxH - 2)
	cmd := pane("Command", "↵", strings.Split(e.command.View(), "\n"), leftW-2, cmdBoxH-2, colBlue, e.field == fCommand)

	left := lipgloss.JoinVertical(lipgloss.Left, meta, notes, cmd)

	// Right column: Variables + Resolved preview
	rightW := width - leftW - 1
	if rightW < 12 {
		return left
	}
	vars := e.commandVars()
	varBody := []string{stFaint.Render("Detected {{ }} placeholders")}
	if len(vars) == 0 {
		varBody = append(varBody, stFaint.Render("none"))
	}
	for _, v := range vars {
		varBody = append(varBody, stYell.Render(cell(v, 14)))
	}
	varsPane := pane("Variables", "", varBody, rightW-2, len(varBody)+1, colBorder, false)

	resInner := codeBlock(e.command.Value(), rightW-4)
	resH := contentH - lipgloss.Height(varsPane) - 1
	if resH < 3 {
		resH = 3
	}
	res := pane("Resolved", "", resInner, rightW-2, resH-2, colBorder, false)
	right := lipgloss.JoinVertical(lipgloss.Left, varsPane, res)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

// hitField maps a click in the editor screen to the field whose row was hit.
// Returns false when the click lands on chrome (borders, gaps) or the
// non-interactive right column.
func (e editorModel) hitField(x, y int) (editorField, bool) {
	leftW := e.width - 30
	if leftW < 30 {
		leftW = e.width
	}
	if x < 0 || x >= leftW {
		return 0, false
	}

	metaInnerLen := 4
	if e.field == fFolder {
		if len(e.folderMatches(5)) > 0 {
			metaInnerLen += 2
		}
	}
	metaH := metaInnerLen + 1 + 2 // pane: innerH (= len+1) + top + bottom borders
	notesH := 4                   // pane: innerH=2 + 2 borders

	// meta content rows: y=1..4 → title, folder, tags, lang
	if y >= 1 && y <= 4 {
		return editorField(y - 1), true
	}
	// notes content row sits just below the meta pane's top border
	if y == metaH+1 {
		return fNotes, true
	}
	// anywhere inside the command pane (skipping its top border)
	if y >= metaH+notesH+1 {
		return fCommand, true
	}
	return 0, false
}

func (e editorModel) commandVars() []string {
	s := &model.Snippet{Command: e.command.Value()}
	return s.Variables()
}

// folderMatches returns folder candidates that start with the current input
// (case-insensitive), or the top-level folders when the input is empty.
func (e editorModel) folderMatches(limit int) []string {
	v := strings.TrimSpace(e.folder.Value())
	lv := strings.ToLower(v)
	var out []string
	for _, c := range e.folderCands {
		lc := strings.ToLower(c)
		if v == "" {
			if !strings.Contains(c, "/") {
				out = append(out, c)
			}
		} else if lc != lv && strings.HasPrefix(lc, lv) {
			out = append(out, c)
		}
		if len(out) >= limit {
			break
		}
	}
	return out
}

// folderSuggestion returns the best inline completion, or "".
func (e editorModel) folderSuggestion() string {
	m := e.folderMatches(1)
	if len(m) == 0 {
		return ""
	}
	return m[0]
}

// acceptFolderSuggestion fills the folder field with the best completion.
// Reports whether a completion was applied.
func (e *editorModel) acceptFolderSuggestion() bool {
	if e.field != fFolder {
		return false
	}
	sug := e.folderSuggestion()
	if sug == "" {
		return false
	}
	e.folder.SetValue(sug)
	e.folder.CursorEnd()
	return true
}
