package blugesearcher

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"strings"

	"github.com/blugelabs/bluge"
	"github.com/hashicorp/go-hclog"
	"libdb.so/nix-search/search"

	blugesearch "github.com/blugelabs/bluge/search"
	blugehighlight "github.com/blugelabs/bluge/search/highlight"
)

// PackagesSearcher implements search.PackagesSearcher.
type PackagesSearcher struct {
	reader *bluge.Reader
}

var _ search.PackagesSearcher = (*PackagesSearcher)(nil)

// Exists checks if the index exists.
func Exists(path string) bool {
	if path == "" {
		var err error

		path, err = defaultIndexPath()
		if err != nil {
			return false
		}
	}

	path = filepath.Join(path, lastIndexVersion)
	if _, err := os.Stat(path); err != nil {
		return false
	}

	return true
}

// Open opens a PackagesSearcher at the given path. If path is
// empty, the default path is used.
func Open(path string) (*PackagesSearcher, error) {
	if !Exists(path) {
		return nil, fmt.Errorf("index does not exist")
	}

	if path == "" {
		var err error

		path, err = defaultIndexPath()
		if err != nil {
			return nil, fmt.Errorf("cannot get user cache dir: %w", err)
		}
	}

	path = filepath.Join(path, lastIndexVersion)
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
func (s *PackagesSearcher) SearchPackages(ctx context.Context, query string, opts search.Opts) (iter.Seq[search.SearchedPackage], error) {
	var highlighter blugehighlight.Highlighter
	if opts.Highlight != nil {
		switch highlight := opts.Highlight.(type) {
		case search.HighlightStyleANSI:
			highlighter = newANSIHighlighterColor(highlight.ANSIEscapeWithDefault())
		case search.HighlightStyleHTML:
			highlighter = blugehighlight.NewHTMLHighlighterTags(highlight.OpenTag(), highlight.CloseTag())
		}
	}

	searchQuery := bluge.NewBooleanQuery()
	searchQuery.SetMinShould(1)

	if opts.Regex {
		searchQuery.AddShould(
			bluge.NewRegexpQuery(query).SetField("name").SetBoost(2),
			bluge.NewRegexpQuery(query).SetField("description"),
		)
	} else {
		searchQuery.AddShould(
			// For exact matches.
			bluge.NewTermQuery(query).SetField("path").SetBoost(16),
			bluge.NewTermQuery(query).SetField("name").SetBoost(8),
			// For full word matches.
			bluge.NewMatchQuery(query).SetField("path").SetBoost(6),
			bluge.NewMatchQuery(query).SetField("name").SetBoost(4),
			bluge.NewMatchQuery(query).SetField("description").SetBoost(2),
			// For partial substring matches.
			bluge.NewWildcardQuery("*"+query+"*").SetField("path").SetBoost(4),
			bluge.NewWildcardQuery("*"+query+"*").SetField("name").SetBoost(2),
			bluge.NewWildcardQuery("*"+query+"*").SetField("description"),
			// For fuzzy matches.
			bluge.NewFuzzyQuery(query).SetField("path").SetBoost(4),
			bluge.NewFuzzyQuery(query).SetField("name").SetBoost(2),
			bluge.NewFuzzyQuery(query).SetField("description"),
		)
	}

	log := hclog.FromContext(ctx)
	log.Debug("searching", "query", query)

	request := bluge.NewAllMatches(searchQuery).
		WithStandardAggregations().
		IncludeLocations()

	matchIter, err := s.reader.Search(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("cannot search: %w", err)
	}

	return func(yield func(p search.SearchedPackage) bool) {
		var locationBuf []blugesearch.Location

		for {
			match, err := matchIter.Next()
			if err != nil {
				log.Error("cannot iterate matches", "error", err)
				break
			}

			if match == nil {
				break
			}

			var path string
			var jsonData []byte
			err = match.VisitStoredFields(func(field string, value []byte) bool {
				switch field {
				case "_id": // ID has same length as .path but is more correct
					path = string(value)
				case "json":
					jsonData = value
				}
				return path == "" || len(jsonData) == 0
			})
			if err != nil {
				log.Error("cannot visit stored fields", "error", err)
				continue
			}

			var pkg search.Package
			if err := json.Unmarshal(jsonData, &pkg); err != nil {
				log.Error("cannot unmarshal package", "id", path, "error", err)
				continue
			}

			result := search.SearchedPackage{
				Package: pkg,
				Path:    path,
			}

			if highlighter != nil {
				locationBuf = match.Complete(locationBuf)
			}

			if opts.Exact {
				for i, possible := range []string{
					result.Path,
					result.Name,
					result.Description,
				} {
					start := strings.Index(possible, query)
					if start == -1 {
						continue
					}

					// Edit the highlighted location directly, if needed.
					if highlighter != nil {
						end := start + len(query)

						termMap := blugesearch.TermLocationMap{
							query: {
								&blugesearch.Location{
									Pos:   0,
									Start: start,
									End:   end,
								},
							},
						}

						switch i {
						case 0:
							match.Locations["path"] = termMap
						case 1:
							match.Locations["name"] = termMap
						case 2:
							match.Locations["description"] = termMap
						}
					}

					goto ok
				}

				continue
			ok:
			}

			if highlighter != nil {
				hresult := highlightPackage(match, highlighter, result)
				result.Highlighted = &hresult
			}

			if !yield(result) {
				return
			}
		}
	}, nil
}

func highlightPackage(match *blugesearch.DocumentMatch, highlighter blugehighlight.Highlighter, pkg search.SearchedPackage) search.SearchedPackage {
	highlighted := pkg
	highlighted.Name = highlighter.BestFragment(match.Locations["name"], []byte(pkg.Name))
	highlighted.Path = highlighter.BestFragment(match.Locations["path"], []byte(pkg.Path))
	highlighted.Description = highlighter.BestFragment(match.Locations["description"], []byte(pkg.Description))
	return highlighted
}

func newANSIHighlighterColor(color string) *blugehighlight.SimpleHighlighter {
	fragmenter := blugehighlight.NewSimpleFragmenterSized(256)
	formatter := blugehighlight.NewANSIFragmentFormatterColor(color)
	return blugehighlight.NewSimpleHighlighter(fragmenter, formatter, blugehighlight.DefaultSeparator)
}
