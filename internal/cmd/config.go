package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

func LoadGlobalsConfig() ([]string, error) {
	path, err := xdg.ConfigFile(filepath.Join("q", "globals.conf"))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve constants config path: %w", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// return no error when the file doesn't exist
			return nil, nil
		}

		return nil, fmt.Errorf("failed to read constants config file: %w", err)
	}

	return strings.Fields(string(b)), nil
}
