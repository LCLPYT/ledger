package assetrepo

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

type AssetRepo struct {
	Id     string `yaml:"id"`
	Path   string `yaml:"path"`
	Public bool   `yaml:"public"`
}

func (r *AssetRepo) ReadPermission() string {
	return "assets.read:" + r.Id
}

func (r *AssetRepo) FileSystem() (http.FileSystem, error) {
	err := r.CheckPath()

	if err != nil {
		return nil, err
	}

	return dotFileHidingFS{http.Dir(r.Path)}, nil
}

func (r *AssetRepo) CheckPath() error {
	if info, err := os.Stat(r.Path); err != nil {
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
