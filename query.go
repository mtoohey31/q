package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

func Query(musicDir, query string) ([]string, error) {
	i, err := os.Stat(query)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		if i.IsDir() {
			paths := []string{}

			err := filepath.Walk(query, func(path string, info fs.FileInfo, err error) error {
				if info.Mode().IsRegular() {
					paths = append(paths, path)
				}

				return err
			})
			return paths, err
		} else if i.Mode().IsRegular() {
			return []string{query}, nil
		}
	}

	return nil, nil
}
