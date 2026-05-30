package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/joshuademarco/glyph/internal/model"
	"github.com/spf13/cobra"
)

// exportCmd writes snippets as a JSON library document to stdout or a file.
func exportCmd() *cobra.Command {
	var folder, tag, out string
	c := &cobra.Command{
		Use:   "export",
		Short: "Export snippets as a JSON library (to stdout or --out file)",
		Example: "  glyph export > backup.json\n" +
			"  glyph export -f Docker --out docker-snippets.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			items := st.Filter(folder, tag)
			lib := &model.Library{
				Schema:    model.SchemaVersion,
				Snippets:  items,
				UpdatedAt: nowUTC(),
			}
			b, err := json.MarshalIndent(lib, "", "  ")
			if err != nil {
				return err
			}
			if out != "" {
				if err := os.WriteFile(out, b, 0o600); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "exported %d snippet(s) → %s\n", len(items), out)
				return nil
			}
			_, err = os.Stdout.Write(append(b, '\n'))
			return err
		},
	}
	f := c.Flags()
	f.StringVarP(&folder, "folder", "f", "", "only export a folder (prefix match)")
	f.StringVarP(&tag, "tag", "T", "", "only export snippets with this tag")
	f.StringVarP(&out, "out", "o", "", "write to a file instead of stdout")
	return c
}

// importCmd merges a JSON library (file or stdin) into the local store.
func importCmd() *cobra.Command {
	var replace bool
	c := &cobra.Command{
		Use:   "import [file]",
		Short: "Import snippets from a JSON library (file or stdin), merging by id",
		Long: "Import merges snippets from a JSON document into your library.\n\n" +
			"The document may be a full library (as produced by `glyph export`) or a\n" +
			"bare array of snippets. Snippets are matched by id: a newer incoming copy\n" +
			"updates the local one, unknown ids are added. Use --replace to always take\n" +
			"the incoming copy.",
		Args: cobra.MaximumNArgs(1),
		Example: "  glyph import backup.json\n" +
			"  cat shared.json | glyph import",
		RunE: func(cmd *cobra.Command, args []string) error {
			var data []byte
			var err error
			if len(args) == 1 && args[0] != "-" {
				data, err = os.ReadFile(args[0])
			} else {
				data, err = io.ReadAll(os.Stdin)
			}
			if err != nil {
				return err
			}
			incoming, err := decodeLibrary(data)
			if err != nil {
				return fail("%v", err)
			}

			st, err := openStore()
			if err != nil {
				return err
			}
			added, updated := 0, 0
			for _, in := range incoming {
				if in == nil {
					continue
				}
				if in.ID == "" {
					in.ID = model.NewID()
				}
				if cur := st.Lib.Find(in.ID); cur != nil {
					if replace || in.UpdatedAt.After(cur.UpdatedAt) {
						*cur = *in
						updated++
					}
				} else {
					st.Lib.Snippets = append(st.Lib.Snippets, in)
					added++
				}
			}
			st.Lib.UpdatedAt = nowUTC()
			if err := st.Save(); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "imported: %d added, %d updated\n", added, updated)
			return nil
		},
	}
	c.Flags().BoolVar(&replace, "replace", false, "always take the incoming copy on id conflicts")
	return c
}

// decodeLibrary accepts either a library object or a bare snippet array.
func decodeLibrary(data []byte) ([]*model.Snippet, error) {
	var lib model.Library
	if err := json.Unmarshal(data, &lib); err == nil && lib.Snippets != nil {
		return lib.Snippets, nil
	}
	var arr []*model.Snippet
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr, nil
	}
	return nil, fmt.Errorf("could not parse JSON: expected a glyph library or a snippet array")
}
