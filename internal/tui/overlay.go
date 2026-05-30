package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joshuademarco/glyph/internal/clipboard"
	"github.com/joshuademarco/glyph/internal/model"
)

// ---------- variable-fill overlay ----------

// openVarOverlay prepares the variable-fill overlay for a snippet, building one
// input per {{placeholder}}.
func (m *Model) openVarOverlay(s *model.Snippet, action varAction) {
	names := s.Variables()
	m.varSnippet = s
	m.varNames = names
	m.varAction = action
	m.varInputs = make([]textinput.Model, len(names))
	for i, n := range names {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = "value for " + n
		ti.TextStyle = stFg
		ti.PlaceholderStyle = stFaint
		m.varInputs[i] = ti
	}
	m.varIdx = 0
	if len(m.varInputs) > 0 {
		m.varInputs[0].Focus()
	}
	m.varsOpen = true
}

func (m Model) updateVars(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.closeVars()
		return m, nil
	case "tab", "down":
		if len(m.varInputs) > 0 {
			m.varInputs[m.varIdx].Blur()
			m.varIdx = (m.varIdx + 1) % len(m.varInputs)
			m.varInputs[m.varIdx].Focus()
		}
		return m, nil
	case "shift+tab", "up":
		if len(m.varInputs) > 0 {
			m.varInputs[m.varIdx].Blur()
			m.varIdx = (m.varIdx - 1 + len(m.varInputs)) % len(m.varInputs)
			m.varInputs[m.varIdx].Focus()
		}
		return m, nil
	case "ctrl+s":
		return m.submitVars()
	case "enter":
		if m.varIdx < len(m.varInputs)-1 {
			m.varInputs[m.varIdx].Blur()
			m.varIdx++
			m.varInputs[m.varIdx].Focus()
			return m, nil
		}
		return m.submitVars()
	}
	if len(m.varInputs) > 0 {
		var cmd tea.Cmd
		m.varInputs[m.varIdx], cmd = m.varInputs[m.varIdx].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) closeVars() {
	m.varsOpen = false
	m.varSnippet = nil
	m.varInputs = nil
	m.varNames = nil
	m.varIdx = 0
}

func (m Model) varValues() map[string]string {
	vals := map[string]string{}
	for i, n := range m.varNames {
		if i < len(m.varInputs) {
			vals[n] = m.varInputs[i].Value()
		}
	}
	return vals
}

func (m Model) submitVars() (tea.Model, tea.Cmd) {
	s := m.varSnippet
	if s == nil {
		m.closeVars()
		return m, nil
	}
	resolved := s.Resolve(m.varValues())
	action := m.varAction
	m.closeVars()
	switch action {
	case varRun:
		s.MarkUsed(time.Now().UTC())
		_ = m.st.Save()
		c := shellCommand(resolved)
		return m, tea.ExecProcess(c, func(err error) tea.Msg { return runFinishedMsg{err} })
	default: // varYank
		if err := clipboard.Copy(resolved); err != nil {
			m.flash("copy failed: %v", err)
			return m, nil
		}
		s.MarkUsed(time.Now().UTC())
		_ = m.st.Save()
		m.flash("yanked resolved %q", short(s.Title, 24))
		return m, nil
	}
}

func (m Model) viewVars() string {
	floatW := m.width * 6 / 10
	if floatW < 52 {
		floatW = min1(m.width-4, 52)
	}
	inner := floatW - 2

	title := "yank with variables"
	if m.varAction == varRun {
		title = "run with variables"
	}

	var lines []string
	lines = append(lines, stDim.Render(short(m.varSnippet.Title, inner)))
	lines = append(lines, "")
	for i, n := range m.varNames {
		mark := "  "
		if i == m.varIdx {
			mark = stBlue.Render("▸ ")
		}
		lines = append(lines, mark+stYell.Render(cell(n, 14))+" "+m.varInputs[i].View())
	}
	lines = append(lines, "")
	lines = append(lines, stFaint.Render("Resolved"))
	lines = append(lines, codeBlock(m.varSnippet.Resolve(m.varValues()), inner-2)...)

	box := pane(title, "", lines, inner, len(lines)+1, colGreen, true)
	placed := lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Top,
		lipgloss.NewStyle().MarginTop(3).Render(box))
	bar := statusBar("FILL", colYellow, [][2]string{{"⇥", "next"}, {"⏎", "submit"}, {"⌃S", "submit"}, {"esc", "cancel"}}, title, m.width)
	return placed + "\n" + bar
}

// ---------- single-line prompt overlay (move / tag) ----------

func (m *Model) openPrompt(action promptAction, label, initial string) {
	m.promptOpen = true
	m.promptAction = action
	m.promptLabel = label
	pi := textinput.New()
	pi.Prompt = ""
	pi.TextStyle = stFg
	pi.PlaceholderStyle = stFaint
	pi.SetValue(initial)
	pi.CursorEnd()
	pi.Focus()
	m.promptInput = pi
}

func (m *Model) closePrompt() {
	m.promptOpen = false
	m.promptAction = promptNone
	m.promptInput.Blur()
}

func (m Model) updatePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.closePrompt()
		return m, nil
	case "enter":
		return m.submitPrompt()
	}
	var cmd tea.Cmd
	m.promptInput, cmd = m.promptInput.Update(msg)
	return m, cmd
}

func (m Model) submitPrompt() (tea.Model, tea.Cmd) {
	val := strings.TrimSpace(m.promptInput.Value())
	action := m.promptAction
	targets := m.targets()
	m.closePrompt()
	if len(targets) == 0 {
		return m, nil
	}
	now := time.Now().UTC()
	switch action {
	case promptMoveFolder:
		folder := strings.Trim(val, "/")
		for _, s := range targets {
			s.Folder = folder
			s.UpdatedAt = now
		}
		m.marked = map[string]bool{}
		_ = m.st.Save()
		m.rebuildFolders()
		m.refilter()
		dest := folder
		if dest == "" {
			dest = "(root)"
		}
		m.flash("moved %d → %s", len(targets), dest)
	case promptAddTag:
		tag := strings.TrimSpace(strings.TrimPrefix(val, "#"))
		if tag == "" {
			return m, nil
		}
		n := 0
		for _, s := range targets {
			if snippetHasTag(s, tag) {
				continue
			}
			s.Tags = append(s.Tags, tag)
			s.UpdatedAt = now
			n++
		}
		m.marked = map[string]bool{}
		_ = m.st.Save()
		m.rebuildFolders()
		m.refilter()
		m.flash("tagged %d with #%s", n, tag)
	}
	return m, nil
}

func (m Model) viewPrompt() string {
	floatW := m.width * 5 / 10
	if floatW < 44 {
		floatW = min1(m.width-4, 44)
	}
	inner := floatW - 2

	count := len(m.targets())
	scope := stFaint.Render(fmt.Sprintf("%d snippet(s)", count))
	input := stBlue.Render("❯") + " " + m.promptInput.View()

	lines := []string{
		stDim.Render(m.promptLabel) + " " + scope,
		"",
		input,
	}
	box := pane("Action", "", lines, inner, len(lines)+1, colBlue, true)
	placed := lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Top,
		lipgloss.NewStyle().MarginTop(4).Render(box))
	bar := statusBar("INPUT", colBlue, [][2]string{{"⏎", "apply"}, {"esc", "cancel"}}, m.promptLabel, m.width)
	return placed + "\n" + bar
}

// ---------- confirmation overlay ----------

func (m *Model) openConfirm(action confirmAction, prompt string) {
	m.confirmOpen = true
	m.confirmAction = action
	m.confirmPrompt = prompt
}

func (m *Model) closeConfirm() {
	m.confirmOpen = false
	m.confirmAction = confirmNone
	m.confirmPrompt = ""
	m.confirmTargets = nil
	m.confirmRun = nil
}

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		action := m.confirmAction
		targets := m.confirmTargets
		run := m.confirmRun
		m.closeConfirm()
		switch action {
		case confirmDelete, confirmBulkDelete:
			(&m).performDelete(targets)
			return m, nil
		case confirmRunDanger:
			if run != nil {
				return m.doRun(run)
			}
		}
		return m, nil
	case "n", "N", "esc", "ctrl+c":
		m.closeConfirm()
		m.flash("cancelled")
		return m, nil
	}
	return m, nil
}

func (m Model) viewConfirm() string {
	floatW := m.width * 5 / 10
	if floatW < 40 {
		floatW = min1(m.width-4, 40)
	}
	inner := floatW - 2

	accent := colRed
	lines := []string{
		"",
		stFg.Render(m.confirmPrompt),
		"",
		stFaint.Render("y to confirm · n / esc to cancel"),
	}
	box := pane("Confirm", "", lines, inner, len(lines)+1, accent, true)
	placed := lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, box)
	bar := statusBar("CONFIRM", colRed, [][2]string{{"y", "yes"}, {"n", "no"}}, "", m.width)
	return placed + "\n" + bar
}
