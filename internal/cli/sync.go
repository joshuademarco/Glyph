package cli

import (
	"fmt"
	"os"

	"github.com/joshuademarco/glyph/internal/config"
	"github.com/joshuademarco/glyph/internal/model"
	gsync "github.com/joshuademarco/glyph/internal/sync"
	"github.com/spf13/cobra"
)

func syncCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "sync",
		Short: "Sync snippets across devices (two-way merge)",
		Long: "Sync merges your local library with a remote backend so the same snippets " +
			"are available on every device.\n\n" +
			"Backends:\n" +
			"  gist  — a private GitHub Gist (needs only a token; works anywhere)\n" +
			"  file  — a file in a folder you already sync (Dropbox/iCloud/OneDrive/Syncthing)\n\n" +
			"Configure once with `glyph sync setup ...`, then run `glyph sync`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			b, err := gsync.New(cfg)
			if err != nil {
				return fail("%v", err)
			}
			// Let the gist backend persist a newly created id.
			if gb, ok := b.(*gsync.GistBackend); ok {
				gb.OnCreate(func(id string) {
					cfg.GistID = id
					_ = cfg.Save()
				})
			}

			st, err := openStore()
			if err != nil {
				return err
			}
			res, err := gsync.Sync(st.Lib, b, func(lib *model.Library) error {
				st.Lib = lib
				return st.Save()
			})
			if err != nil {
				return fail("%v", err)
			}
			fmt.Printf("synced with %s: %d pulled, %d pushed\n", b.Name(), res.Pulled, res.Pushed)
			return nil
		},
	}
	c.AddCommand(syncSetupCmd(), syncStatusCmd())
	return c
}

func syncSetupCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "setup <gist|file> [path]",
		Short: "Configure a sync backend",
		Long: "Configure how glyph syncs.\n\n" +
			"  glyph sync setup gist            # then paste a token when prompted\n" +
			"  glyph sync setup gist <id>       # reuse an existing gist id\n" +
			"  glyph sync setup file <path>     # e.g. ~/Dropbox/glyph/snippets.json",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			switch args[0] {
			case "gist":
				cfg.Sync = "gist"
				if len(args) > 1 {
					cfg.GistID = args[1]
				}
				fmt.Fprintln(os.Stderr, "Create a token at https://github.com/settings/tokens with the \"gist\" scope.")
				tok := prompt("GitHub token: ")
				if tok == "" && cfg.GistToken == "" {
					return fail("a token is required for gist sync")
				}
				if tok != "" {
					cfg.GistToken = tok
				}
			case "file":
				if len(args) < 2 {
					return fail("file backend needs a path: glyph sync setup file <path>")
				}
				cfg.Sync = "file"
				cfg.FilePath = args[1]
			default:
				return fail("unknown backend %q (use gist or file)", args[0])
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Printf("sync configured: %s\n", cfg.Sync)
			fmt.Fprintln(os.Stderr, "run `glyph sync` to push your snippets.")
			return nil
		},
	}
	return c
}

func syncStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the configured sync backend",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if cfg.Sync == "" {
				fmt.Println("sync: not configured (run `glyph sync setup`)")
				return nil
			}
			fmt.Println("sync:", cfg.Sync)
			switch cfg.Sync {
			case "gist":
				id := cfg.GistID
				if id == "" {
					id = "(will be created on first sync)"
				}
				fmt.Println("gist id:", id)
				fmt.Println("token:", masked(cfg.GistToken))
			case "file":
				fmt.Println("path:", cfg.FilePath)
			}
			return nil
		},
	}
}

func masked(s string) string {
	if s == "" {
		return "(none)"
	}
	if len(s) <= 4 {
		return "****"
	}
	return "****" + s[len(s)-4:]
}
