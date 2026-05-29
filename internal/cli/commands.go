package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joshuademarco/glyph/internal/clipboard"
	"github.com/joshuademarco/glyph/internal/config"
	"github.com/joshuademarco/glyph/internal/model"
	"github.com/spf13/cobra"
)

// addCmd captures a new snippet. The body comes from --command, piped stdin, or $EDITOR — in that order — making `history | tail -1 | glyph add`
func addCmd() *cobra.Command {
	var title, folder, lang, notes, command, source string
	var tags []string

	c := &cobra.Command{
		Use:   "add",
		Short: "Add a snippet (reads the body from --command, stdin, or $EDITOR)",
		Example: "  history | tail -1 | glyph add -t \"resize image\"\n" +
			"  cat deploy.sh | glyph add -t deploy -f Ops/Deploy -l sh\n" +
			"  glyph add -t \"k8s restart\" -c 'kubectl rollout restart deploy/{{name}}'",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := command
			if body == "" {
				if piped() {
					b, err := io.ReadAll(os.Stdin)
					if err != nil {
						return err
					}
					body = strings.TrimRight(string(b), "\n")
				} else {
					b, err := editorInput("")
					if err != nil {
						return err
					}
					body = b
				}
			}
			if strings.TrimSpace(body) == "" {
				return fail("nothing to add: empty command body")
			}
			if title == "" {
				title = firstLine(body)
			}

			st, err := openStore()
			if err != nil {
				return err
			}
			s := &model.Snippet{
				Title:   title,
				Folder:  strings.Trim(folder, "/"),
				Tags:    cleanTags(tags),
				Lang:    lang,
				Notes:   notes,
				Command: body,
				Source:  source,
			}
			st.Lib.Add(s)
			if err := st.Save(); err != nil {
				return err
			}
			fmt.Printf("added %s  %s\n", s.ID, s.Title)
			return nil
		},
	}
	f := c.Flags()
	f.StringVarP(&title, "title", "t", "", "snippet title (defaults to first line)")
	f.StringVarP(&folder, "folder", "f", "", "folder path, e.g. Docker/Compose")
	f.StringSliceVar(&tags, "tags", nil, "comma-separated tags")
	f.StringVarP(&lang, "lang", "l", "", "language: sh, yml, sql, py, …")
	f.StringVarP(&notes, "notes", "n", "", "free-form description")
	f.StringVarP(&command, "command", "c", "", "command body (skips stdin/$EDITOR)")
	f.StringVar(&source, "source", "", "where it came from (e.g. a file path)")
	return c
}

// listCmd lists snippets, optionally filtered and/or as JSON.
func listCmd() *cobra.Command {
	var folder, tag string
	var asJSON, quiet bool
	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List snippets",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			items := st.Filter(folder, tag)
			if asJSON {
				return json.NewEncoder(os.Stdout).Encode(items)
			}
			if quiet {
				for _, s := range items {
					fmt.Println(s.ID)
				}
				return nil
			}
			if len(items) == 0 {
				fmt.Fprintln(os.Stderr, "no snippets")
				return nil
			}
			for _, s := range items {
				loc := s.Folder
				if loc == "" {
					loc = "—"
				}
				fmt.Printf("%s  %-32s  %-18s  %s\n", s.ID, truncate(s.Title, 32), truncate(loc, 18), strings.Join(s.Tags, ","))
			}
			return nil
		},
	}
	f := c.Flags()
	f.StringVarP(&folder, "folder", "f", "", "filter by folder (prefix match)")
	f.StringVarP(&tag, "tag", "T", "", "filter by tag")
	f.BoolVar(&asJSON, "json", false, "output JSON")
	f.BoolVarP(&quiet, "quiet", "q", false, "print only ids")
	return c
}

// getCmd prints a snippet's raw command to stdout for piping into a shell.
func getCmd() *cobra.Command {
	var noTrack bool
	c := &cobra.Command{
		Use:     "get <ref>",
		Short:   "Print a snippet's command to stdout (pipe it: glyph get x | sh)",
		Args:    cobra.ExactArgs(1),
		Example: "  glyph get deploy | sh\n  ssh host \"$(glyph get 'disk usage')\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			s, err := st.Resolve(args[0])
			if err != nil {
				return fail("%v", err)
			}
			fmt.Println(s.Command)
			if !noTrack {
				s.MarkUsed(nowUTC())
				_ = st.Save()
			}
			return nil
		},
	}
	c.Flags().BoolVar(&noTrack, "no-track", false, "do not count this as a use")
	return c
}

// copyCmd copies a snippet to the clipboard.
func copyCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "copy <ref>",
		Aliases: []string{"yank", "cp"},
		Short:   "Copy a snippet's command to the clipboard",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			s, err := st.Resolve(args[0])
			if err != nil {
				return fail("%v", err)
			}
			if err := clipboard.Copy(s.Command); err != nil {
				return fail("clipboard: %v", err)
			}
			s.MarkUsed(nowUTC())
			_ = st.Save()
			fmt.Fprintf(os.Stderr, "copied %q\n", s.Title)
			return nil
		},
	}
	return c
}

// runCmd resolves variables then executes the snippet in a shell.
func runCmd() *cobra.Command {
	var sets []string
	var dry bool
	c := &cobra.Command{
		Use:     "run <ref>",
		Short:   "Run a snippet, prompting for any {{variables}}",
		Args:    cobra.ExactArgs(1),
		Example: "  glyph run deploy --set name=api --set ns=prod",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			s, err := st.Resolve(args[0])
			if err != nil {
				return fail("%v", err)
			}
			vals := parseSets(sets)
			for _, v := range s.Variables() {
				if _, ok := vals[v]; ok {
					continue
				}
				vals[v] = prompt(fmt.Sprintf("%s = ", v))
			}
			resolved := s.Resolve(vals)
			if dry {
				fmt.Println(resolved)
				return nil
			}
			s.MarkUsed(nowUTC())
			_ = st.Save()
			sh := shellCmd(resolved)
			sh.Stdin, sh.Stdout, sh.Stderr = os.Stdin, os.Stdout, os.Stderr
			return sh.Run()
		},
	}
	c.Flags().StringArrayVar(&sets, "set", nil, "set a variable, e.g. --set name=api")
	c.Flags().BoolVar(&dry, "dry-run", false, "print the resolved command instead of running it")
	return c
}

// searchCmd fuzzy-searches snippets.
func searchCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:     "search <query>",
		Aliases: []string{"find"},
		Short:   "Fuzzy-search snippets by title, folder and tags",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			hits := st.Search(strings.Join(args, " "))
			if asJSON {
				return json.NewEncoder(os.Stdout).Encode(hits)
			}
			for _, s := range hits {
				fmt.Printf("%s  %-34s  %s\n", s.ID, truncate(s.Title, 34), s.Folder)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return c
}

// rmCmd deletes a snippet (with confirmation unless --yes).
func rmCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:     "rm <ref>",
		Aliases: []string{"delete"},
		Short:   "Delete a snippet",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			s, err := st.Resolve(args[0])
			if err != nil {
				return fail("%v", err)
			}
			if !yes {
				if !confirm(fmt.Sprintf("delete %q? [y/N] ", s.Title)) {
					fmt.Fprintln(os.Stderr, "aborted")
					return nil
				}
			}
			st.Lib.Remove(s.ID)
			if err := st.Save(); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "deleted %q\n", s.Title)
			return nil
		},
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	return c
}

// editCmd edits a snippet's command body in $EDITOR.
func editCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "edit <ref>",
		Short: "Edit a snippet's command body in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			s, err := st.Resolve(args[0])
			if err != nil {
				return fail("%v", err)
			}
			body, err := editorInput(s.Command)
			if err != nil {
				return err
			}
			s.Command = body
			s.UpdatedAt = nowUTC()
			if s.LooksDangerous() {
				s.Dangerous = true
			}
			if err := st.Save(); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "updated %q\n", s.Title)
			return nil
		},
	}
	return c
}

// whereCmd prints glyph's file locations.
func whereCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "where",
		Short: "Print the paths glyph uses",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := config.Dir()
			lib, _ := config.LibraryPath()
			fmt.Println("config dir:", dir)
			fmt.Println("library:   ", lib)
			fmt.Println("config:    ", filepath.Join(dir, "config.json"))
			return nil
		},
	}
}

// --- helpers ---

func piped() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

func editorInput(initial string) (string, error) {
	ed := os.Getenv("GLYPH_EDITOR")
	if ed == "" {
		if cfg, _ := config.Load(); cfg != nil && cfg.Editor != "" {
			ed = cfg.Editor
		}
	}
	if ed == "" {
		ed = os.Getenv("EDITOR")
	}
	if ed == "" {
		ed = defaultEditor()
	}
	tmp, err := os.CreateTemp("", "glyph-*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if initial != "" {
		_, _ = tmp.WriteString(initial)
	}
	tmp.Close()

	cmd := exec.Command(ed, tmp.Name())
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor %q: %w", ed, err)
	}
	b, err := os.ReadFile(tmp.Name())
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), "\n"), nil
}

// stdinReader is shared across prompts
var stdinReader = bufio.NewReader(os.Stdin)

func prompt(label string) string {
	fmt.Fprint(os.Stderr, label)
	line, _ := stdinReader.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}

func confirm(label string) bool {
	ans := strings.ToLower(strings.TrimSpace(prompt(label)))
	return ans == "y" || ans == "yes"
}

func parseSets(sets []string) map[string]string {
	out := map[string]string{}
	for _, kv := range sets {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			out[kv[:i]] = kv[i+1:]
		}
	}
	return out
}

func cleanTags(tags []string) []string {
	var out []string
	for _, t := range tags {
		t = strings.TrimSpace(strings.TrimPrefix(t, "#"))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
