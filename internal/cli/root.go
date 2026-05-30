// Package cli wires up glyph's command-line interface with Cobra.
package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joshuademarco/glyph/internal/store"
	"github.com/joshuademarco/glyph/internal/tui"
	"github.com/spf13/cobra"
)

// version is set via -ldflags at build time.
var version = "dev"

// SetVersion lets main inject the build version.
func SetVersion(v string) {
	if v != "" {
		version = v
	}
}

// Execute runs the root command.
func Execute() error {
	return newRoot().Execute()
}

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "glyph",
		Short: "glyph — a TUI & CLI store for code snippets and shell commands",
		Long: "glyph keeps your snippets and one-liners in one place: a fast vim-style TUI " +
			"for browsing, and a pipe-friendly CLI for capturing and reusing them.\n\n" +
			"Run `glyph` with no arguments to open the TUI.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Default action (no subcommand) launches the TUI.
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI()
		},
	}

	root.AddCommand(
		addCmd(),
		listCmd(),
		getCmd(),
		copyCmd(),
		runCmd(),
		searchCmd(),
		rmCmd(),
		editCmd(),
		exportCmd(),
		importCmd(),
		tuiCmd(),
		syncCmd(),
		whereCmd(),
	)
	return root
}

func openStore() (*store.Store, error) {
	return store.Open()
}

func tuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the interactive TUI (default when no command is given)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI()
		},
	}
}

func runTUI() error {
	st, err := openStore()
	if err != nil {
		return err
	}
	p := tea.NewProgram(tui.New(st), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}

// fail prints an error to stderr and returns exit code 1.
func fail(format string, a ...any) error {
	fmt.Fprintf(os.Stderr, "glyph: "+format+"\n", a...)
	return errExit
}

var errExit = fmt.Errorf("")
