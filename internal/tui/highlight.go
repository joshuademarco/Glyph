package tui

import (
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"
)

// hlVarRe matches Glyph {{...}} placeholders; highlighted over lexer output.
var hlVarRe = regexp.MustCompile(`\{\{\s*[a-zA-Z_][a-zA-Z0-9_]*\s*\}\}`)

// langAliases maps Glyph short codes to chroma lexer names for gaps.
var langAliases = map[string]string{
	"yml":        "yaml",
	"ps1":        "powershell",
	"dockerfile": "docker",
	"jsonc":      "json",
	"plaintext":  "plaintext",
	"text":       "plaintext",
}

// lexerFor returns a coalesced chroma lexer for lang, falling back to plaintext.
func lexerFor(lang string) chroma.Lexer {
	name := strings.ToLower(strings.TrimSpace(lang))
	if alias, ok := langAliases[name]; ok {
		name = alias
	}
	var lx chroma.Lexer
	if name != "" {
		lx = lexers.Get(name)
	}
	if lx == nil {
		lx = lexers.Fallback
	}
	return chroma.Coalesce(lx)
}

// styleFor maps a chroma token type to a Glyph style.
func styleFor(t chroma.TokenType) lipgloss.Style {
	switch t.Category() {
	case chroma.Keyword:
		return stHlKw
	case chroma.Name:
		switch t.SubCategory() {
		case chroma.NameBuiltin, chroma.NameFunction:
			return stHlFn
		case chroma.NameVariable:
			return stHlVar
		}
		switch t {
		case chroma.NameClass, chroma.NameNamespace, chroma.NameException, chroma.NameDecorator:
			return stHlFn
		case chroma.NameAttribute, chroma.NameTag, chroma.NameLabel:
			return stHlVar
		}
		return stFg
	case chroma.Literal:
		switch t.SubCategory() {
		case chroma.LiteralString:
			return stHlStr
		case chroma.LiteralNumber:
			return stHlNum
		}
		return stFg
	case chroma.Operator, chroma.Punctuation:
		return stHlOp
	case chroma.Comment:
		return stKwCm
	}
	return stFg
}

// hlRun is a styled fragment in a rendered line.
type hlRun struct {
	text  string
	style lipgloss.Style
}

// highlightCode tokenizes code and returns one styled string per source line.
func highlightCode(code, lang string) []string {
	code = strings.ReplaceAll(code, "\r\n", "\n")
	want := strings.Count(code, "\n") + 1

	it, err := lexerFor(lang).Tokenise(nil, code)
	if err != nil {
		return plainLines(code)
	}

	// Split tokens into lines (tokens may contain '\n').
	var lines [][]hlRun
	var cur []hlRun
	for _, tok := range it.Tokens() {
		st := styleFor(tok.Type)
		for i, part := range strings.Split(tok.Value, "\n") {
			if i > 0 {
				lines = append(lines, cur)
				cur = nil
			}
			if part != "" {
				cur = append(cur, hlRun{part, st})
			}
		}
	}
	lines = append(lines, cur)

	// Ensure result has original line count.
	for len(lines) < want {
		lines = append(lines, nil)
	}
	lines = lines[:want]

	out := make([]string, len(lines))
	for i, runs := range lines {
		out[i] = renderRuns(overlayVars(runs))
	}
	return out
}

// plainLines renders lines with plain foreground style.
func plainLines(code string) []string {
	lines := strings.Split(code, "\n")
	for i, ln := range lines {
		lines[i] = stFg.Render(ln)
	}
	return lines
}

// renderRuns concatenates runs, merging adjacent runs with the same style.
func renderRuns(runs []hlRun) string {
	var b strings.Builder
	for i := 0; i < len(runs); {
		j := i + 1
		for j < len(runs) && sameStyle(runs[j].style, runs[i].style) {
			j++
		}
		var text strings.Builder
		for k := i; k < j; k++ {
			text.WriteString(runs[k].text)
		}
		b.WriteString(runs[i].style.Render(text.String()))
		i = j
	}
	return b.String()
}

// sameStyle reports whether two styles render identically.
func sameStyle(a, b lipgloss.Style) bool {
	return a.GetForeground() == b.GetForeground() &&
		a.GetItalic() == b.GetItalic() &&
		a.GetBold() == b.GetBold()
}

// overlayVars restyles {{...}} spans as variable style.
func overlayVars(runs []hlRun) []hlRun {
	if len(runs) == 0 {
		return runs
	}
	var raw strings.Builder
	for _, r := range runs {
		raw.WriteString(r.text)
	}
	locs := hlVarRe.FindAllStringIndex(raw.String(), -1)
	if len(locs) == 0 {
		return runs
	}
	inVar := make([]bool, raw.Len())
	for _, lo := range locs {
		for i := lo[0]; i < lo[1]; i++ {
			inVar[i] = true
		}
	}

	var out []hlRun
	pos := 0
	for _, r := range runs {
		b := r.text
		start := 0
		for i := 1; i <= len(b); i++ {
			if i == len(b) || inVar[pos+i] != inVar[pos+start] {
				st := r.style
				if inVar[pos+start] {
					st = stHlVar
				}
				out = append(out, hlRun{b[start:i], st})
				start = i
			}
		}
		pos += len(b)
	}
	return out
}

// codeBlock returns numbered, highlighted lines; width reserved by caller.
func codeBlock(code, lang string, w int) []string {
	hl := highlightCode(code, lang)
	out := make([]string, len(hl))
	for i, ln := range hl {
		num := stLn.Render(lpad(itoa(i+1), 3))
		out[i] = num + "  " + ln
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
