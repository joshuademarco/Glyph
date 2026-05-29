package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// highlightLine applies a lightweight shell-ish syntax highlight to a single
// line: comments, the leading command word, long/short flags, and {{vars}}.
func highlightLine(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]

	if strings.HasPrefix(trimmed, "#") {
		return indent + stKwCm.Render(trimmed)
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return line
	}

	var out strings.Builder
	out.WriteString(indent)
	for i, f := range fields {
		if i > 0 {
			out.WriteString(" ")
		}
		switch {
		case i == 0:
			out.WriteString(stKwFn.Render(f))
		case strings.HasPrefix(f, "-"):
			out.WriteString(stKwFl.Render(f))
		case strings.HasPrefix(f, "{{") && strings.HasSuffix(f, "}}"):
			out.WriteString(lipgloss.NewStyle().Foreground(colCyan).Render(f))
		default:
			out.WriteString(stFg.Render(highlightVars(f)))
		}
	}
	return out.String()
}

// highlightVars colors any {{var}} placeholders embedded inside a token.
func highlightVars(s string) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	var b strings.Builder
	for {
		i := strings.Index(s, "{{")
		if i < 0 {
			b.WriteString(s)
			break
		}
		j := strings.Index(s, "}}")
		if j < 0 || j < i {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:i])
		b.WriteString(stCyan.Render(s[i : j+2]))
		s = s[j+2:]
	}
	return b.String()
}

// codeBlock renders command text as numbered, highlighted lines within width w.
func codeBlock(code string, w int) []string {
	lines := strings.Split(strings.ReplaceAll(code, "\r\n", "\n"), "\n")
	var out []string
	for i, ln := range lines {
		num := stLn.Render(lpad(itoa(i+1), 3))
		out = append(out, num+"  "+highlightLine(ln))
	}
	return out
}

func lpad(s string, w int) string {
	for len(s) < w {
		s = " " + s
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
