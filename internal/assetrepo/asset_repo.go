package assetrepo

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

type AssetRepo struct {
	Id     string `config:"-"`
	Path   string `config:"path" validate:"required"`
	Public bool   `config:"public"`
}

func (r *AssetRepo) ReadPermission() string {
	return "assets.read:" + r.Id
}

func (r *AssetRepo) FileSystem() (http.FileSystem, error) {
	err := r.EnsurePath()

	if err != nil {
		return nil, err
	}

	return dotFileHidingFS{http.Dir(r.Path)}, nil
}

func (r *AssetRepo) EnsurePath() error {
	if info, err := os.Stat(r.Path); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(r.Path, 0o755); err != nil {
			return fmt.Errorf("asset_repo: failed to create repo %s directory at %s: %w", r.Id, r.Path, err)
		}
	} else if err != nil {
		return fmt.Errorf("asset_repo: failed to access repo %s at %s directory file: %w", r.Id, r.Path, err)
	} else if !info.IsDir() {
		return fmt.Errorf("asset_repo: root_dir %s is not a directory", r.Path)
	}

	return nil
}

type dotFileHidingFS struct {
	http.FileSystem
}

func (fsys dotFileHidingFS) Open(name string) (http.File, error) {
	for part := range strings.SplitSeq(name, "/") {
		if strings.HasPrefix(part, ".") {
			return nil, fs.ErrNotExist
		}
	}

	f, err := fsys.FileSystem.Open(name)

	if err != nil {
		return nil, err
	}

	return dotFileHidingFile{f}, nil
}

type dotFileHidingFile struct {
	http.File
}

func (f dotFileHidingFile) Readdir(n int) ([]fs.FileInfo, error) {
	entries, err := f.File.Readdir(n)
	visible := entries[:0]

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") {
			visible = append(visible, entry)
		}
	}

	return visible, err
}
