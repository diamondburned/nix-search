package search

import "context"

// TODO: make a search index for s p e e d, maybe use Bleve

// PackagesSearcher is a searcher for packages.
type PackagesSearcher interface {
	// SearchPackages returns a channel of packages that match the given query.
	// The channel is closed when there are no more results or ctx is canceled.
	SearchPackages(ctx context.Context, query string) (<-chan SearchedPackage, error)
	// SearchPackagesRe returns a channel of packages that match the given
	// regex.
	// The channel is closed when there are no more results or ctx is canceled.
	SearchPackagesRe(ctx context.Context, regex string) (<-chan SearchedPackage, error)
	// ReplacePackageIndex replaces the index of packages with the given name
	// with the given set.
	ReplacePackageIndex(TopLevelPackages) error
}

// SearchedPackage is a package that was searched for.
type SearchedPackage struct {
	Package
	// Path is the path to the derivation.
	Path Path
}
