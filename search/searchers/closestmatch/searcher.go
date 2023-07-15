package closestmatch

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"libdb.so/closestmatch"
	"libdb.so/nix-search/search"
)

// PackagesSearcher implements searcher.PackagesSearcher.
type PackagesSearcher struct {
	cm closestmatch.ClosestMatch[matchData]
}

// New reads a PackagesSearcher from the given path. If path is empty, the
// default path is used.
func New(path string) (*PackagesSearcher, error) {
	if path == "" {
		p, err := defaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}

	path = filepath.Join(path, "closestmatch-index.json.gz")

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	gzf, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("cannot create gzip reader: %w", err)
	}
	defer gzf.Close()

	var p PackagesSearcher
	if err := json.NewDecoder(gzf).Decode(&p.cm); err != nil {
		return nil, fmt.Errorf("cannot decode search: %w", err)
	}

	if err := gzf.Close(); err != nil {
		return nil, fmt.Errorf("cannot close gzip reader: %w", err)
	}

	return &p, nil
}

// SearchPackages searches for packages matching the given query. The returned
// channel is closed when the search is complete. The given context can be used
// to cancel the search.
func (s *PackagesSearcher) SearchPackages(ctx context.Context, query string, opts search.Opts) (<-chan search.SearchedPackage, error) {
	const threshold = 3

	matches := s.cm.ClosestN(query, 300)

	ch := make(chan search.SearchedPackage, len(matches))
	go func() {
		defer close(ch)

		for _, match := range matches {
			result := search.SearchedPackage{
				Path:    match.Data.Path,
				Package: match.Data.Package,
			}
			select {
			case <-ctx.Done():
				return
			case ch <- result:
				// ok
			}
		}
	}()

	return ch, nil
}
