package bluge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/blugelabs/bluge"
	"github.com/hashicorp/go-hclog"
	"libdb.so/nix-search/search"
)

// PackagesSearcher implements search.PackagesSearcher.
type PackagesSearcher struct {
	reader *bluge.Reader
}

var _ search.PackagesSearcher = (*PackagesSearcher)(nil)

// OpenPackagesSearcher opens a PackagesSearcher at the given path. If path is
// empty, the default path is used.
func OpenPackagesSearcher(path string) (*PackagesSearcher, error) {
	if path == "" {
		var err error

		path, err = defaultPath()
		if err != nil {
			return nil, fmt.Errorf("cannot get user cache dir: %w", err)
		}
	}

	path = filepath.Join(path, "index")
	log.Println("opening index at", path)

	config := bluge.DefaultConfig(path)

	reader, err := bluge.OpenReader(config)
	if err != nil {
		return nil, fmt.Errorf("cannot open bluge reader: %w", err)
	}

	return &PackagesSearcher{reader}, nil
}

// Close closes the index.
func (s *PackagesSearcher) Close() error {
	return s.reader.Close()
}

// SearchPackages implements search.PackagesSearcher. The searching is done by
// fuzzy matching the query.
func (s *PackagesSearcher) SearchPackages(ctx context.Context, query string) (<-chan search.SearchedPackage, error) {
	searchQuery := bluge.NewBooleanQuery()
	searchQuery.SetMinShould(1)
	searchQuery.AddShould(
		bluge.NewFuzzyQuery(query).SetField("name").SetBoost(2).SetFuzziness(2),
		bluge.NewMatchQuery(query).SetField("name").SetBoost(2),
		bluge.NewFuzzyQuery(query).SetField("description").SetFuzziness(2),
		bluge.NewMatchQuery(query).SetField("description"),
	)

	request := bluge.NewTopNSearch(10, searchQuery).
		WithStandardAggregations()

	matchIter, err := s.reader.Search(context.Background(), request)
	if err != nil {
		return nil, fmt.Errorf("cannot search: %w", err)
	}

	results := make(chan search.SearchedPackage)
	go func() {
		defer close(results)

		match, err := matchIter.Next()
		for err == nil && match != nil {
			var id string
			var jsonData []byte
			match.VisitStoredFields(func(field string, value []byte) bool {
				log := hclog.FromContext(ctx)
				log.Debug("stored field", "field", field, "value", string(value))
				switch field {
				case "_id":
					id = string(value)
				case "json":
					jsonData = value
				}
				return id == "" || len(jsonData) == 0
			})

			pkg := search.SearchedPackage{
				Path: search.Path(strings.Split(id, ".")),
			}
			if err := json.Unmarshal(jsonData, &pkg.Package); err != nil {
				log := hclog.FromContext(ctx)
				log.Error("cannot unmarshal package", "id", id, "error", err)
				continue
			}

			select {
			case results <- pkg:
				match, err = matchIter.Next()
			case <-ctx.Done():
				return
			}
		}

		if err != nil {
			log := hclog.FromContext(ctx)
			log.Error("cannot iterate matches", "error", err)
		}
	}()

	return results, nil
}

func (s *PackagesSearcher) SearchPackagesRe(ctx context.Context, regex string) (<-chan search.SearchedPackage, error) {
	panic("not implemented")
}
