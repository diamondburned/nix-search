package closestmatch

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"libdb.so/closestmatch"
	"libdb.so/nix-search/search"
)

type matchData struct {
	Path    string
	Package search.Package
}

// IndexPackages indexes the given packages into the file at path. If path is
// empty, the default path is used.
func IndexPackages(ctx context.Context, path string, packages search.TopLevelPackages) error {
	if path == "" {
		p, err := defaultPath()
		if err != nil {
			return fmt.Errorf("cannot get default path: %w", err)
		}
		path = p
	}

	path = filepath.Join(path, "closestmatch-index.json.gz")

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("cannot create directory: %w", err)
	}

	dict := make(map[string]matchData, packages.Count())
	packages.Walk(func(path search.Path, pkg search.Package) bool {
		dict[buildSearchString(path, pkg)] = matchData{
			Path:    path.String(),
			Package: pkg,
		}
		return true
	})

	search := closestmatch.New[matchData](dict, []int{3, 4, 5})

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}
	defer f.Close()

	gzf := gzip.NewWriter(f)

	if err := json.NewEncoder(gzf).Encode(search); err != nil {
		return fmt.Errorf("cannot encode search: %w", err)
	}

	if err := gzf.Close(); err != nil {
		return fmt.Errorf("cannot close gzip writer: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("cannot close file: %w", err)
	}

	return nil
}

func defaultPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("cannot get user cache dir: %w", err)
	}

	cacheDir = filepath.Join(cacheDir, "nix-search")
	return cacheDir, nil
}

func buildSearchString(path search.Path, pkg search.Package) string {
	return strings.Join([]string{
		path.String(),
		pkg.Name,
		pkg.Description,
	}, " ")
}
