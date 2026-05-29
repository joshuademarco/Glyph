package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/joshuademarco/glyph/internal/model"
)

// --- browse screen ---

func (m Model) viewBrowse() string {
	g := browseLayout(m.width, m.height)
	folderW, rightW, contentH := g.folderW, g.rightW, g.contentH
	listBoxH, previewBoxH := g.listBoxH, g.previewBoxH

	folderBody := m.folderBody(folderW - 2)
	folderPane := pane("Folders", "1", folderBody, folderW-2, contentH-2, colBlue, m.focus == focusFolders)

	fi := ""
	if len(m.folders) > 0 {
		fi = m.folders[m.folderIdx].label
	}
	listBody := m.listBody(rightW - 2)
	listPane := pane("Snippets · "+fi, "2", listBody, rightW-2, listBoxH-2, colGreen, m.focus == focusList)

	prevBody := m.previewBody(rightW-2, previewBoxH-2)
	prevPane := pane("Preview", "3", prevBody, rightW-2, previewBoxH-2, colBlue, m.focus == focusPreview)

	right := lipgloss.JoinVertical(lipgloss.Left, listPane, "", prevPane)
	body := lipgloss.JoinHorizontal(lipgloss.Top, folderPane, " ", right)

	keys := [][2]string{{"j/k", "move"}, {"⏎", "yank"}, {"e", "edit"}, {"n", "new"}, {"x", "run"}, {"/", "find"}, {"dd", "del"}, {"?", "help"}}
	right2 := fmt.Sprintf("%d of %d", min1(m.listIdx+1, len(m.snippets)), len(m.snippets))
	if s := m.statusLine(); s != "" {
		right2 = s
	}
	bar := statusBar("NORMAL", colGreen, keys, right2, m.width)
	return body + "\n" + bar
}

func (m Model) statusLine() string {
	if m.status != "" && time.Since(m.statusTime) < 4*time.Second {
		return m.status
	}
	return ""
}

func (m Model) folderBody(w int) []string {
	var out []string
	isFolderish := func(k string) bool { return k == "group" || k == "folder" }
	for i, f := range m.folders {
		if isFolderish(f.kind) && (i == 0 || !isFolderish(m.folders[i-1].kind)) {
			out = append(out, divlabel("Folders", w))
		}
		sel := m.focus == focusFolders && i == m.folderIdx
		icon := iconFor(f)
		out = append(out, sideRow(icon, f.label, fmt.Sprintf("%d", f.count), w, sel, colBlue))
	}
	return out
}

func iconFor(f folderItem) string {
	switch f.kind {
	case "fav":
		return stYell.Render("★")
	case "recent":
		return stGreen.Render("◷")
	case "danger":
		return stRed.Render("!")
	case "all":
		return stBlue.Render("∗")
	case "group":
		if f.collapsed {
			return stBlue.Render("▸")
		}
		return stBlue.Render("▾")
	case "folder":
		return stDim.Render("·")
	default:
		return " "
	}
}

func (m Model) listBody(w int) []string {
	if len(m.snippets) == 0 {
		return []string{"", stFaint.Render("  no snippets — press n to create one")}
	}
	var out []string
	for i, s := range m.snippets {
		sel := m.focus == focusList && i == m.listIdx
		icon := stDim.Render("·")
		if s.Favorite {
			icon = stYell.Render("★")
		}
		if sel {
			icon = stGreen.Render("❯")
		}
		right := ""
		if s.Lang != "" {
			right += langStyle(s.Lang).Render(s.Lang) + " "
		}
		right += stFaint.Render(timeAgo(s.UpdatedAt))
		out = append(out, sideRow(icon, s.Title, right, w, sel, colGreen))
	}
	return out
}

func (m Model) previewBody(w, innerH int) []string {
	s := m.selected()
	if s == nil {
		return []string{stFaint.Render("nothing selected")}
	}
	var top []string
	head := stBlue.Bold(true).Render(short(s.Title, w-8))
	if s.Lang != "" {
		head += "  " + langStyle(s.Lang).Render(s.Lang)
	}
	top = append(top, head)
	if s.Notes != "" {
		for _, ln := range strings.Split(lipgloss.NewStyle().Width(w).Foreground(colFgDim).Render(s.Notes), "\n") {
			top = append(top, ln)
		}
	}
	top = append(top, "")
	top = append(top, codeBlock(s.Command, w)...)

	stats := fmt.Sprintf("used %d× · ", s.UseCount)
	if s.LastUsed != nil {
		stats += "last " + timeAgo(*s.LastUsed)
	} else {
		stats += "never used"
	}
	statsLine := stDim.Render(stats)
	if len(s.Tags) > 0 {
		ts := make([]string, len(s.Tags))
		for i, t := range s.Tags {
			ts[i] = "#" + t
		}
		statsLine += stFaint.Render("   " + strings.Join(ts, " "))
	}
	var bottom []string
	bottom = append(bottom, statsLine)
	if s.Source != "" {
		bottom = append(bottom, stFaint.Render("source "+s.Source))
	}

	pad := innerH - len(top) - len(bottom)
	if pad < 1 {
		pad = 1
	}
	out := top
	for i := 0; i < pad; i++ {
		out = append(out, "")
	}
	out = append(out, bottom...)
	return out
}

// sideRow renders an icon + grow-label + right-aligned trailing segment.
func sideRow(icon, label, right string, w int, selected bool, accent lipgloss.Color) string {
	rw := lipgloss.Width(right)
	labelW := w - 2 - 1 - rw - 1
	if labelW < 1 {
		labelW = 1
	}
	lstyle := stFg
	if selected {
		lstyle = lipgloss.NewStyle().Foreground(colFg).Bold(true)
	}
	lab := lstyle.Render(short(label, labelW))
	line := icon + " " + cell(lab, labelW) + " " + right
	if selected {
		return lipgloss.NewStyle().Foreground(accent).Render("▎") + cell(line, w-1)
	}
	return " " + cell(line, w-1)
}

// --- palette overlay ---

func (m Model) viewPalette() string {
	floatW := m.width * 7 / 10
	if floatW < 48 {
		floatW = min1(m.width-4, 48)
	}
	inner := floatW - 2

	input := stBlue.Render("❯") + " " + m.palInput.View()
	scope := stFaint.Render("all folders · fuzzy")
	header := cell(input, inner-lipgloss.Width(scope)-1) + scope

	var rows []string
	maxRows := 10
	for i, s := range m.palResults {
		if i >= maxRows {
			break
		}
		sel := i == m.palIdx
		icon := stDim.Render("·")
		if sel {
			icon = stBlue.Render("❯")
		}
		folder := ""
		if s.Folder != "" {
			folder = stFaint.Render(s.Folder + "/ ")
		}
		right := ""
		if s.Lang != "" {
			right = langStyle(s.Lang).Render(s.Lang)
		}
		label := folder + s.Title
		rows = append(rows, sideRow(icon, label, right, inner, sel, colBlue))
	}
	if len(rows) == 0 {
		rows = append(rows, stFaint.Render("  no matches"))
		rows = append(rows, "")
		rows = append(rows, sideRow(stGreen.Render("+"), "Create snippet “"+m.palInput.Value()+"”", stFaint.Render("⌃N"), inner, true, colGreen))
	}

	footer := stFaint.Render(fmt.Sprintf("↑↓ move · ⏎ yank · ⌃N new · %d of %d", min1(m.palIdx+1, len(m.palResults)), len(m.palResults)))

	bodyLines := []string{header, divlabel("Snippets", inner)}
	bodyLines = append(bodyLines, rows...)
	bodyLines = append(bodyLines, "", footer)

	box := pane("Command palette", "", bodyLines, inner, len(bodyLines)+1, colBlue, true)

	placed := lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Top,
		lipgloss.NewStyle().MarginTop(3).Render(box))

	bar := statusBar("SEARCH", colYellow, [][2]string{{"esc", "cancel"}, {"⏎", "yank"}, {"⌃N", "new"}}, "fuzzy", m.width)
	return placed + "\n" + bar
}

// --- editor screen ---

func (m Model) viewEditor() string {
	contentH := m.height - 1
	body := m.editor.view(m.width, contentH)
	mode := "EDIT"
	if m.editor.editingID == "" {
		mode = "NEW"
	}
	keys := [][2]string{{"⇥", "next field"}, {"⌃S", "save"}, {"esc", "cancel"}}
	right := m.statusLine()
	if right == "" {
		right = fmt.Sprintf("%d vars", len(m.editor.commandVars()))
	}
	bar := statusBar(mode, colBlue, keys, right, m.width)
	// pad body to fill height
	bh := lipgloss.Height(body)
	pad := contentH - bh
	if pad > 0 {
		body += strings.Repeat("\n", pad)
	}
	return body + "\n" + bar
}

// --- help screen ---

func (m Model) viewHelp() string {
	contentH := m.height - 1
	lines := []string{
		stBlue.Bold(true).Render("glyph — keys"),
		"",
		stGreen.Render("Browse"),
		kv("j / k", "move within pane"),
		kv("h / l · tab", "switch pane"),
		kv("g / G", "top / bottom"),
		kv("space", "collapse / expand group"),
		kv("⏎ · y", "yank command to clipboard"),
		kv("x", "run command in shell"),
		kv("e / n", "edit / new snippet"),
		kv("f", "toggle favorite"),
		kv("dd", "delete snippet"),
		kv("/ · ⌃P", "fuzzy command palette"),
		kv("click", "focus / select a pane row"),
		"",
		stGreen.Render("Editor"),
		kv("⇥", "next field (or complete folder)"),
		kv("→", "accept folder suggestion"),
		kv("⌃S", "save"),
		kv("esc", "cancel"),
		"",
		stGreen.Render("Variables"),
		stDim.Render("  Use {{name}} in a command. glyph run prompts for values;"),
		stDim.Render("  yank copies the template as-is."),
		"",
		stGreen.Render("Sync"),
		stDim.Render("  glyph sync setup gist|file, then glyph sync."),
		"",
		stFaint.Render("press ? or esc to close"),
	}
	box := pane("Help", "", lines, m.width-4, len(lines)+1, colBlue, true)
	placed := lipgloss.Place(m.width, contentH, lipgloss.Center, lipgloss.Center, box)
	bar := statusBar("HELP", colBlue, [][2]string{{"?", "close"}}, "", m.width)
	return placed + "\n" + bar
}

func kv(k, v string) string {
	return "  " + stFg.Render(cell(k, 14)) + stDim.Render(v)
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/24/7))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/24/30))
	default:
		return fmt.Sprintf("%dy", int(d.Hours()/24/365))
	}
}

func min1(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ = model.SchemaVersion
