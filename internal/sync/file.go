package sync

import (
	"os"
	"path/filepath"

	"github.com/joshuademarco/glyph/internal/model"
)

// FileBackend syncs through a plain file
type FileBackend struct {
	Path string
}

func (f *FileBackend) Name() string { return "file (" + f.Path + ")" }

func (f *FileBackend) Pull() (*model.Library, error) {
	b, err := os.ReadFile(f.Path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return unmarshal(b)
}

func (f *FileBackend) Push(lib *model.Library) error {
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
		return err
	}
	b, err := marshal(lib)
	if err != nil {
		return err
	}
	tmp := f.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f.Path)
}
