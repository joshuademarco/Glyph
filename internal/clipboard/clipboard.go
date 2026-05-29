// Package clipboard copies text to the system clipboard with a fallback
package clipboard

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/atotto/clipboard"
)

// Copy puts text on the clipboard.
func Copy(text string) error {
	if !clipboard.Unsupported {
		if err := clipboard.WriteAll(text); err == nil {
			return nil
		}
	}
	return osc52(text)
}

// osc52 writes the terminal OSC 52 "set clipboard" sequence to the controllingterminal. tmux and screen need the sequence wrapped
func osc52(text string) error {
	tty, err := os.OpenFile(ttyName(), os.O_WRONLY, 0)
	if err != nil {
		// Fall back to stderr so the sequence still reaches the terminal.
		tty = os.Stderr
	} else {
		defer tty.Close()
	}
	enc := base64.StdEncoding.EncodeToString([]byte(text))
	_, err = fmt.Fprintf(tty, "\x1b]52;c;%s\x07", enc)
	return err
}
