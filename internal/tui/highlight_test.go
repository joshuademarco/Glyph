package tui

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// stripANSI returns the visible text of a styled string
func stripANSI(s string) string { return ansi.Strip(s) }

func TestHighlightCodeLineCount(t *testing.T) {
	cases := []struct {
		name, lang, code string
	}{
		{"shell", "sh", "echo hi\nls -la /tmp"},
		{"trailing newline", "sh", "echo hi\n"},
		{"sql multiline", "sql", "SELECT *\nFROM users\nWHERE id = 1;"},
		{"empty", "sh", ""},
		{"unknown lang", "made-up", "plain text here"},
		{"block comment spans lines", "go", "/* a\nb\nc */\nx := 1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := highlightCode(c.code, c.lang)
			want := strings.Count(strings.ReplaceAll(c.code, "\r\n", "\n"), "\n") + 1
			if len(got) != want {
				t.Fatalf("line count = %d, want %d", len(got), want)
			}
			// Visible text must survive highlighting unchanged.
			joined := stripANSI(strings.Join(got, "\n"))
			if normalized := strings.ReplaceAll(c.code, "\r\n", "\n"); joined != normalized {
				t.Fatalf("visible text changed:\n got %q\nwant %q", joined, normalized)
			}
		})
	}
}

// forceColor pins the renderer to truecolor for the duration of a test so style.Render emits ANSI
func forceColor(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

func TestStyleForMapping(t *testing.T) {
	cases := []struct {
		tok  chroma.TokenType
		want lipgloss.Style
		name string
	}{
		{chroma.Keyword, stHlKw, "keyword"},
		{chroma.KeywordReserved, stHlKw, "keyword sub"},
		{chroma.NameFunction, stHlFn, "function"},
		{chroma.NameBuiltin, stHlFn, "builtin"},
		{chroma.NameClass, stHlFn, "class"},
		{chroma.NameVariable, stHlVar, "variable"},
		{chroma.NameAttribute, stHlVar, "attribute"},
		{chroma.LiteralString, stHlStr, "string"},
		{chroma.LiteralStringDouble, stHlStr, "string sub"},
		{chroma.LiteralNumber, stHlNum, "number"},
		{chroma.LiteralNumberInteger, stHlNum, "number sub"},
		{chroma.Operator, stHlOp, "operator"},
		{chroma.Punctuation, stHlOp, "punctuation"},
		{chroma.CommentSingle, stKwCm, "comment"},
		{chroma.Text, stFg, "plain text"},
	}
	for _, c := range cases {
		if got := styleFor(c.tok); got.GetForeground() != c.want.GetForeground() {
			t.Errorf("%s: styleFor(%v) fg = %v, want %v", c.name, c.tok, got.GetForeground(), c.want.GetForeground())
		}
	}
}

func TestHighlightAppliesColor(t *testing.T) {
	forceColor(t)
	out := highlightCode(`echo "hello world"`, "sh")
	if len(out) != 1 {
		t.Fatalf("expected 1 line, got %d", len(out))
	}
	if out[0] == stripANSI(out[0]) {
		t.Fatal("expected ANSI styling in highlighted shell line, got none")
	}
}

func TestOverlayVarsColorsPlaceholders(t *testing.T) {
	forceColor(t)
	code := `curl https://api/{{host}}/users`
	out := highlightCode(code, "sh")
	if len(out) != 1 {
		t.Fatalf("expected 1 line, got %d", len(out))
	}
	if stripANSI(out[0]) != code {
		t.Fatalf("visible text changed: %q", stripANSI(out[0]))
	}
	varColored := stHlVar.Render("{{host}}")
	if !strings.Contains(out[0], varColored) {
		t.Fatalf("placeholder not styled with var color\n line: %q\n want substring: %q", out[0], varColored)
	}
}
