package tui

import "github.com/charmbracelet/lipgloss"

// Palette — GitHub dark-dimmed, matching the design system in TUI Design/tui.css.
var (
	colBg      = lipgloss.Color("#0d1117")
	colBg2     = lipgloss.Color("#010409")
	colPanel   = lipgloss.Color("#0f141b")
	colBorder  = lipgloss.Color("#30363d")
	colBorderD = lipgloss.Color("#21262d")
	colFg      = lipgloss.Color("#c9d1d9")
	colFgDim   = lipgloss.Color("#8b949e")
	colFgFaint = lipgloss.Color("#484f58")
	colBlue    = lipgloss.Color("#58a6ff")
	colGreen   = lipgloss.Color("#3fb950")
	colYellow  = lipgloss.Color("#d29922")
	colRed     = lipgloss.Color("#f85149")
	colPurple  = lipgloss.Color("#bc8cff")
	colCyan    = lipgloss.Color("#56d4dd")
)

var (
	stFg    = lipgloss.NewStyle().Foreground(colFg)
	stDim   = lipgloss.NewStyle().Foreground(colFgDim)
	stFaint = lipgloss.NewStyle().Foreground(colFgFaint)
	stBlue  = lipgloss.NewStyle().Foreground(colBlue)
	stGreen = lipgloss.NewStyle().Foreground(colGreen)
	stYell  = lipgloss.NewStyle().Foreground(colYellow)
	stRed   = lipgloss.NewStyle().Foreground(colRed)
	stCyan  = lipgloss.NewStyle().Foreground(colCyan)

	// selected row washes
	stSelBlue  = lipgloss.NewStyle().Background(lipgloss.Color("#16273f")).Foreground(colFg)
	stSelGreen = lipgloss.NewStyle().Background(lipgloss.Color("#16331f")).Foreground(colFg)

	// tag chip
	stTagBlue   = lipgloss.NewStyle().Foreground(colBlue)
	stTagGreen  = lipgloss.NewStyle().Foreground(colGreen)
	stTagYellow = lipgloss.NewStyle().Foreground(colYellow)
	stTagPurple = lipgloss.NewStyle().Foreground(colPurple)

	// code-preview token colors
	stKwFn  = lipgloss.NewStyle().Foreground(colBlue)
	stKwFl  = lipgloss.NewStyle().Foreground(colYellow)
	stKwCm  = lipgloss.NewStyle().Foreground(colFgFaint).Italic(true)
	stKwStr = lipgloss.NewStyle().Foreground(colCyan)
	stLn    = lipgloss.NewStyle().Foreground(colFgFaint)
)

// langStyle picks a chip color for a language tag, mirroring the design
func langStyle(lang string) lipgloss.Style {
	switch lang {
	case "yml", "yaml", "json", "toml":
		return stTagBlue
	case "sh", "bash", "zsh", "fish":
		return stTagGreen
	case "sql":
		return stTagPurple
	default:
		return stTagYellow
	}
}
