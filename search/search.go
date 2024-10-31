package search

import (
	"context"
	"html"
	"iter"
	"strings"
)

// TODO: make a search index for s p e e d, maybe use Bleve

// PackagesSearcher is a searcher for packages.
type PackagesSearcher interface {
	// SearchPackages returns a channel of packages that match the given query.
	// The channel is closed when there are no more results or ctx is canceled.
	SearchPackages(ctx context.Context, query string, opts Opts) (iter.Seq[SearchedPackage], error)
}

// Opts are options for searching.
type Opts struct {
	// Highlight is an optional highlighter for this package.
	// If not nil, it can be used to prehighlight matching terms of all
	// packages.
	Highlight HighlightStyle
	// Regex is whether to use regex matching instead.
	// If unsupported, it should return an error.
	Regex bool
	// Exact is whether to match the package exactly according to the string.
	// Note that this filter is applied on top of Bluge's, meaning it narrows
	// down Bluge's results but does not expand them.
	Exact bool
}

// SearchedPackage is a package that was searched for.
type SearchedPackage struct {
	// Path is the path to the derivation.
	Path string `json:"path"`
	Package

	// Highlighted is the color-highlighted package, if any.
	// This is only used if Highlight is set in Opts.
	Highlighted *SearchedPackage `json:"unhighlighted"`
}

// HighlightStyle is a style of highlighting.
type HighlightStyle interface{ isHighlightStyle() }

// HighlightStyleHTML is a style of highlighting that uses HTML.
type HighlightStyleHTML struct {
	// Tag is the HTML tag to use for highlighting.
	// If empty, "mark" is used.
	Tag string
	// Attributes is a map of attributes to add to the tag.
	Attributes map[string]string
}

func (s HighlightStyleHTML) OpenTag() string {
	var b strings.Builder
	b.WriteByte('<')

	if s.Tag != "" {
		b.WriteString(s.Tag)
	} else {
		b.WriteString("mark")
	}

	for k, v := range s.Attributes {
		b.WriteByte(' ')
		b.WriteString(k + `="` + html.EscapeString(v) + `"`)
	}

	b.WriteByte('>')
	return b.String()
}

func (s HighlightStyleHTML) CloseTag() string {
	if s.Tag != "" {
		return "</" + s.Tag + ">"
	}
	return "</mark>"
}

// HighlightStyleANSI is a style of highlighting that uses ANSI escape
// sequences.
type HighlightStyleANSI struct {
	// ANSIEscape is the ANSI escape sequence to use for highlighting.
	// It should be a valid ANSI escape sequence with a prefix of "\x1b".
	// If empty, red is used.
	ANSIEscape string
}

func (s HighlightStyleANSI) ANSIEscapeWithDefault() string {
	if s.ANSIEscape != "" {
		return s.ANSIEscape
	}
	return DefaultANSIEscapeColor
}

// DefaultANSIEscapeColor is the default ANSI escape sequence to use for
// highlighting.
const DefaultANSIEscapeColor = "\x1b[31m" // FgRed

func (HighlightStyleHTML) isHighlightStyle() {}
func (HighlightStyleANSI) isHighlightStyle() {}

// Highlights is a list of highlights. Each highlight is a pair of start and end
// indices.
type Highlights [][2]int

// HighlightedMap maps an arbitrary value to a list of highlights. When
// printing, simply check if the value is in the map, and if it is, print the
// highlights.
type HighlightedMap map[string]Highlights
