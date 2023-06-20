package query

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

// TODO: add support for some sort of query object to filter things based on
// various attributes such as title, artist, path, etc. Ideally an existing
// well-known language should be used instead of creating something new

// TODO: write my own fuzzy algorithm so I can tweak the details

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
	} else if i.Mode().IsRegular() {
		return []string{path}, nil
	}

	paths := []string{}
	err = fs.WalkDir(os.DirFS(musicDir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() != "." && d.Name()[0] == '.' {
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
	if err != nil {
		return nil, err
	}

	ranks := fuzzy.RankFindNormalizedFold(query, paths)
	sort.Sort(ranks)
	paths = paths[:len(ranks)]
	for i, rank := range ranks {
		paths[i] = filepath.Join(musicDir, rank.Target)
	}
	return paths, nil
}
