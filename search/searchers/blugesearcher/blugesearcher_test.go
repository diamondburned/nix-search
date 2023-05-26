package blugesearcher

import (
	"context"
	"os"
	"testing"

	"github.com/alecthomas/assert/v2"
	"github.com/hashicorp/go-hclog"
	"libdb.so/nix-search/search"
)

func TestMain(m *testing.M) {
	hclog.Default().SetLevel(hclog.Debug)

	result := m.Run()
	os.Exit(result)
}

func TestBluge(t *testing.T) {
	packages := search.TopLevelPackages{
		Channel: "nixpkgs",
		PackageSet: search.PackageSet{
			"nix-search": search.Package{
				Name:        "nix-search",
				Description: "Search for packages in Nixpkgs.",
			},
			"nix-index": search.Package{
				Name:        "nix-index",
				Description: "Index Nixpkgs.",
			},
			"firefox": search.Package{
				Name:        "firefox",
				Description: "Firefox is a free and open-source web browser developed by the Mozilla Foundation and its subsidiary, the Mozilla Corporation.",
			},
			"goPackages": search.PackageSet{
				"staticcheck": search.Package{
					Name:        "staticcheck",
					Description: "staticcheck is a go vet on steroids, applying a ton of static analysis checks you might be used to from tools like ReSharper for C#.",
				},
				"bluge": search.Package{
					Name:        "bluge",
					Description: "Bluge is a high-performance, high-level full-text search engine library written in Go.",
				},
			},
		},
	}

	var npackages int
	packages.Walk(func(path search.Path, pkg search.Package) bool {
		npackages++
		return true
	})

	tempIndex, err := os.MkdirTemp("", "bluge-test-*")
	assert.NoError(t, err, "cannot create temporary index")
	t.Logf("using temporary index: %s", tempIndex)

	// t.Cleanup(func() {
	// 	err := os.RemoveAll(tempIndex)
	// 	assert.NoError(t, err, "cannot remove temporary index")
	// })

	ctx := context.Background()

	t.Run("index", func(t *testing.T) {
		err := IndexPackages(ctx, tempIndex, packages)
		assert.NoError(t, err, "cannot index packages")
	})

	searcher, err := Open(tempIndex)
	assert.NoError(t, err, "cannot open searcher")

	t.Cleanup(func() {
		err := searcher.Close()
		assert.NoError(t, err, "cannot close searcher")
	})

	count, err := searcher.reader.Count()
	assert.NoError(t, err, "cannot count packages")
	assert.Equal(t, npackages, int(count), "wrong number of packages")

	t.Run("search", func(t *testing.T) {
		type expectSearch struct {
			query string
			want  []string
		}

		expectSearches := []expectSearch{
			{"nix-search", []string{"nix-search"}},
			{"fire", []string{"firefox"}},
			{"go", []string{"staticcheck", "bluge"}},
		}

		for _, expect := range expectSearches {
			wantSet := setFromList(expect.want)

			t.Run("search:"+expect.query, func(t *testing.T) {
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()

				results, err := searcher.SearchPackages(ctx, expect.query, search.Opts{
					// Highlight: search.HighlightStyleHTML{},
				})
				assert.NoError(t, err, "cannot search for", expect.query)

				for result := range results {
					t.Logf("%s: found result: %v", expect.query, result)
					if wantSet[result.Name] {
						delete(wantSet, result.Name)
					}
					if len(wantSet) == 0 {
						break
					}
				}

				if len(wantSet) > 0 {
					t.Fatalf("%s: not all results found: %v", expect.query, wantSet)
				}
			})
		}

		type unexpectedSearch struct {
			query string
		}

		unexpectedSearches := []unexpectedSearch{
			{"asldjkoasdjasjdasd"},
		}

		for _, unexpected := range unexpectedSearches {
			t.Run("unexpected-search:"+unexpected.query, func(t *testing.T) {
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()

				results, err := searcher.SearchPackages(ctx, unexpected.query, search.Opts{})
				assert.NoError(t, err, "cannot search for", unexpected.query)

				for result := range results {
					t.Errorf("%s: unexpected result: %v", unexpected.query, result)
				}
			})
		}
	})
}

func setFromList[T comparable](list []T) map[T]bool {
	set := make(map[T]bool, len(list))
	for _, item := range list {
		set[item] = true
	}
	return set

}
