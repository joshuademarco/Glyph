package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Palette — GitHub dark-dimmed, matching the design system in TUI Design/tui.css.
// These are package vars so a user theme (config.json "theme") can override them
// at startup via applyTheme.
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

	// syntax-highlight token colors — mapped from chroma token categories in
	// highlight.go. All derive from the col* palette so a user theme overrides
	// them for free.
	stHlKw  = lipgloss.NewStyle().Foreground(colRed)                  // keywords
	stHlFn  = lipgloss.NewStyle().Foreground(colPurple)               // function / builtin / class names
	stHlVar = lipgloss.NewStyle().Foreground(colCyan)                 // variables, attributes, {{placeholders}}
	stHlStr = lipgloss.NewStyle().Foreground(colGreen)                // string literals
	stHlNum = lipgloss.NewStyle().Foreground(colBlue)                 // numeric literals
	stHlOp  = lipgloss.NewStyle().Foreground(colFgDim)                // operators & punctuation
	stKwCm  = lipgloss.NewStyle().Foreground(colFgFaint).Italic(true) // comments
	stLn    = lipgloss.NewStyle().Foreground(colFgFaint)              // line numbers
)

// refreshStyles rebuilds every derived st* style from the current col* palette.
// Call after mutating any col* value (see applyTheme).
func refreshStyles() {
	stFg = lipgloss.NewStyle().Foreground(colFg)
	stDim = lipgloss.NewStyle().Foreground(colFgDim)
	stFaint = lipgloss.NewStyle().Foreground(colFgFaint)
	stBlue = lipgloss.NewStyle().Foreground(colBlue)
	stGreen = lipgloss.NewStyle().Foreground(colGreen)
	stYell = lipgloss.NewStyle().Foreground(colYellow)
	stRed = lipgloss.NewStyle().Foreground(colRed)
	stCyan = lipgloss.NewStyle().Foreground(colCyan)

	stSelBlue = lipgloss.NewStyle().Background(lipgloss.Color("#16273f")).Foreground(colFg)
	stSelGreen = lipgloss.NewStyle().Background(lipgloss.Color("#16331f")).Foreground(colFg)

	stTagBlue = lipgloss.NewStyle().Foreground(colBlue)
	stTagGreen = lipgloss.NewStyle().Foreground(colGreen)
	stTagYellow = lipgloss.NewStyle().Foreground(colYellow)
	stTagPurple = lipgloss.NewStyle().Foreground(colPurple)

	stHlKw = lipgloss.NewStyle().Foreground(colRed)
	stHlFn = lipgloss.NewStyle().Foreground(colPurple)
	stHlVar = lipgloss.NewStyle().Foreground(colCyan)
	stHlStr = lipgloss.NewStyle().Foreground(colGreen)
	stHlNum = lipgloss.NewStyle().Foreground(colBlue)
	stHlOp = lipgloss.NewStyle().Foreground(colFgDim)
	stKwCm = lipgloss.NewStyle().Foreground(colFgFaint).Italic(true)
	stLn = lipgloss.NewStyle().Foreground(colFgFaint)
}

// applyTheme overrides palette colors from a config map (name -> "#rrggbb").
// Unknown keys and blank values are ignored, so a partial theme is fine.
func applyTheme(theme map[string]string) {
	if len(theme) == 0 {
		return
	}
	set := func(dst *lipgloss.Color, key string) {
		if v, ok := theme[key]; ok {
			if t := strings.TrimSpace(v); t != "" {
				*dst = lipgloss.Color(t)
			}
		}
	}
	set(&colBg, "bg")
	set(&colBg2, "bg2")
	set(&colPanel, "panel")
	set(&colBorder, "border")
	set(&colBorderD, "borderD")
	set(&colFg, "fg")
	set(&colFgDim, "fgDim")
	set(&colFgFaint, "faint")
	set(&colBlue, "blue")
	set(&colGreen, "green")
	set(&colYellow, "yellow")
	set(&colRed, "red")
	set(&colPurple, "purple")
	set(&colCyan, "cyan")
	refreshStyles()
}

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
