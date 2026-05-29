package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// cell truncates/pads an (possibly styled) string to exactly w display columns.
func cell(s string, w int) string {
	if w <= 0 {
		return ""
	}
	dw := lipgloss.Width(s)
	if dw > w {
		return ansi.Truncate(s, w, "…")
	}
	if dw < w {
		return s + strings.Repeat(" ", w-dw)
	}
	return s
}

// pane draws a rounded box with a floating title spliced into the top border,
// matching the design's pane chrome. body lines are clipped/padded to innerW
// and the box is exactly innerH content rows tall.
func pane(title, num string, body []string, innerW, innerH int, accent lipgloss.Color, focused bool) string {
	bc := colBorder
	tc := colFgDim
	if focused {
		bc = accent
		tc = accent
	}
	bs := lipgloss.NewStyle().Foreground(bc)
	ts := lipgloss.NewStyle().Foreground(tc).Bold(true)

	label := ""
	if num != "" {
		badge := lipgloss.NewStyle().Foreground(colBg).Background(tc).Bold(true).Render(" " + num + " ")
		label = badge + " " + ts.Render(title)
	} else {
		label = ts.Render(title)
	}
	chip := bs.Render("─ ") + label + bs.Render(" ")
	chipW := lipgloss.Width(chip)
	total := innerW + 2 // include corners region width = innerW; plus the two side chars
	// top line: ╭ + chip + dashes + ╮
	dashes := innerW - (chipW - 0)
	if dashes < 0 {
		dashes = 0
		chip = ansi.Truncate(chip, innerW, "")
	}
	top := bs.Render("╭") + chip + bs.Render(strings.Repeat("─", dashes)) + bs.Render("╮")
	_ = total

	// --- body rows ---
	var b strings.Builder
	b.WriteString(top + "\n")
	for i := 0; i < innerH; i++ {
		line := ""
		if i < len(body) {
			line = body[i]
		}
		b.WriteString(bs.Render("│") + cell(line, innerW) + bs.Render("│") + "\n")
	}
	b.WriteString(bs.Render("╰" + strings.Repeat("─", innerW) + "╯"))
	return b.String()
}

// divlabel renders an uppercase section divider like the design's .divlabel.
func divlabel(s string, w int) string {
	label := stFaint.Render(strings.ToUpper(s) + " ")
	rest := w - lipgloss.Width(label)
	if rest < 0 {
		rest = 0
	}
	return label + lipgloss.NewStyle().Foreground(colBorderD).Render(strings.Repeat("─", rest))
}

// statusBar renders the bottom mode/keys/right bar like the design's .statusbar.
func statusBar(mode string, modeColor lipgloss.Color, keys [][2]string, right string, width int) string {
	modeBox := lipgloss.NewStyle().
		Foreground(colBg2).Background(modeColor).Bold(true).
		Padding(0, 1).Render(mode)

	var parts []string
	for _, k := range keys {
		parts = append(parts, stFg.Render(k[0])+" "+stDim.Render(k[1]))
	}
	mid := " " + strings.Join(parts, stFaint.Render(" · "))

	left := modeBox + mid
	r := stFaint.Render(right)
	gap := width - lipgloss.Width(left) - lipgloss.Width(r)
	if gap < 1 {
		gap = 1
		left = ansi.Truncate(left, width-lipgloss.Width(r)-1, "…")
		gap = width - lipgloss.Width(left) - lipgloss.Width(r)
		if gap < 0 {
			gap = 0
		}
	}
	bar := left + strings.Repeat(" ", gap) + r
	return lipgloss.NewStyle().Background(colBg2).Width(width).Render(bar)
}
