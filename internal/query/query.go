package query

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

func Query(musicDir, query string) ([]string, error) {
	path := query
	wasAbs := filepath.IsAbs(path)
	if !wasAbs {
		path = filepath.Join(musicDir, query)
	}
	i, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else if i.IsDir() && !wasAbs {
		paths := []string{}

		err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			switch d.Name() {
			case ".git", ".thumbnails":
				return fs.SkipDir
			}

			i, err := d.Info()
			if err != nil {
				return err
			}

			if i.Mode().IsRegular() {
				paths = append(paths, path)
			}

			return nil
		})
		return paths, err
	} else if i.Mode().IsRegular() {
		return []string{path}, nil
	}

	return nil, nil
}
