package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go/doc"
	"go/doc/comment"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
	"libdb.so/nix-search/cmd/internal/commoncmd"
	"libdb.so/nix-search/search"
	"libdb.so/nix-search/search/searchers/blugesearcher"
)

var (
	opts        = search.DefaultIndexPackageOpts
	searchExact = true
)

var app = cli.App{
	Name:      "nix-search",
	UsageText: `nix-search [options] [query]`,
	Usage:     "Search for packages in the Nix package index.",
	Flags: commoncmd.JoinFlags(
		commoncmd.Flags,
		[]cli.Flag{
			&cli.BoolFlag{
				Name:  "no-pager",
				Usage: `do not pipe output through a pager; this is the default if stdout is not a terminal`,
			},
			&cli.BoolFlag{
				Name:    "no-color",
				Usage:   `do not use color in output; this is the default if stdout is not a terminal or if the NO_COLOR environment variable is set`,
				EnvVars: []string{"NO_COLOR"},
			},
			&cli.BoolFlag{
				Name:  "json",
				Usage: "output results as JSON, implies --no-{pager,color}",
			},
			&cli.BoolFlag{
				Name:    "index",
				Aliases: []string{"i"},
				Usage:   "update the index before searching",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:        "exact",
				Aliases:     []string{"e"},
				Value:       searchExact,
				Destination: &searchExact,
			},
			&cli.StringFlag{
				Name:    "index-path",
				Usage:   "path to the index directory, defaults to a directory in $XDG_CACHE_HOME",
				EnvVars: []string{"NIX_SEARCH_INDEX_PATH"},
			},
			&cli.StringFlag{
				Name:        "channel",
				Aliases:     []string{"c"},
				Usage:       "channel path to index",
				Value:       opts.Nixpkgs,
				Destination: &opts.Nixpkgs,
				Action: func(ctx *cli.Context, v string) error {
					if !strings.HasPrefix(v, "<") || !strings.HasSuffix(v, ">") {
						return errors.Errorf("invalid channel %q", v)
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:  "flake",
				Usage: "flake to index unless channel is provided",
				Action: func(c *cli.Context, v string) error {
					path, err := search.ResolveNixPathFromFlake(c.Context, c.String("flake"))
					if err != nil {
						return errors.Wrap(err, "failed to resolve flake")
					}
					c.Set("flake", path)
					return nil
				},
			},
			&cli.IntFlag{
				Name:        "max-jobs",
				Aliases:     []string{"j"},
				Usage:       "max parallel jobs, only used with --index",
				Value:       opts.Parallelism,
				Destination: &opts.Parallelism,
			},
		},
	),
	Action: mainAction,
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	commoncmd.Run(ctx, &app)
}

func mainAction(c *cli.Context) error {
	ctx := c.Context
	log := hclog.FromContext(ctx)
	indexPath := c.String("index-path")

	if !blugesearcher.Exists(indexPath) {
		log.Info("first run or outdated index detected, will index packages")
		c.Set("index", "true")
	}

	if c.Bool("index") {
		if c.IsSet("flake") {
			if c.IsSet("channel") {
				return errors.New("cannot set both --channel and --flake")
			}

			opts.Nixpkgs = c.String("flake")
		}

		log.Info("indexing packages")

		pkgs, err := search.IndexPackages(ctx, opts)
		if err != nil {
			return errors.Wrap(err, "failed to get package index")
		}

		if err := blugesearcher.IndexPackages(ctx, indexPath, pkgs); err != nil {
			return errors.Wrap(err, "failed to store indexed packages")
		}
	}

	query := c.Args().First()
	if query == "" {
		return nil
	}
	searcher, err := blugesearcher.Open(indexPath)
	if err != nil {
		return errors.Wrap(err, "failed to create searcher (try running with --update)")
	}
	defer searcher.Close()

	searchOpts := search.Opts{
		Exact: searchExact,
	}

	if c.Bool("json") {
		c.Set("no-pager", "true")
		c.Set("no-color", "true")
	}

	out := io.WriteCloser(os.Stdout)
	if !c.Bool("no-pager") && termWidth() > 0 {
		pager := os.Getenv("PAGER")
		if pager == "" {
			pager = "less -r"
		}

		var pagerCmd *exec.Cmd

		psplit := strings.Split(pager, " ")
		if len(psplit) > 1 {
			pagerCmd = exec.CommandContext(ctx, psplit[0], psplit[1:]...)
		} else {
			pagerCmd = exec.CommandContext(ctx, pager)
		}

		pagerCmd.Stdout = os.Stdout
		pagerCmd.Stderr = os.Stderr

		pagerIn, err := pagerCmd.StdinPipe()
		if err != nil {
			return errors.Wrap(err, "failed to pipe output to pager")
		}
		out = pagerIn

		if err := pagerCmd.Start(); err != nil {
			return errors.Wrap(err, "failed to start pager")
		}

		defer func() {
			if err := pagerCmd.Wait(); err != nil {
				fmt.Fprintf(os.Stderr, "pager failed: %s\n", err)
			}
		}()
	} else {
		if !c.Bool("no-color") && !isatty.IsTerminal(os.Stdout.Fd()) {
			log.Debug("not a terminal, disabling color")
			c.Set("no-color", "true")
		}
	}
	defer out.Close()

	var styler textStyler
	if !c.Bool("no-color") {
		styler = styledText
		searchOpts.Highlight = search.HighlightStyleANSI{}
	}

	pkgsIter, err := searcher.SearchPackages(ctx, query, searchOpts)
	if err != nil {
		return errors.Wrap(err, "failed to search packages")
	}

	pkgs := slices.Collect(pkgsIter)

	if c.Bool("json") {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(pkgs)
	}

	absoluteMatches := make([]search.SearchedPackage, 0, 1)
	pkgs = slices.DeleteFunc(pkgs, func(p search.SearchedPackage) bool {
		dotq := "." + query
		if strings.HasSuffix(p.Path, dotq) {
			// Rehighlight the package with the query in the path.
			if p.Highlighted != nil {
				dotqIx := strings.LastIndex(p.Path, dotq)

				p.Highlighted.Path = "" +
					p.Path[:dotqIx] +
					"." + styler.style(query, search.DefaultANSIEscapeColor, "\x1b[0m") +
					p.Path[dotqIx+len(dotq):]
			}

			absoluteMatches = append(absoluteMatches, p)
			return true
		}

		return false
	})

	if len(absoluteMatches) > 0 {
		fmt.Fprintln(out, styler.bold("* Exact matches:"))
		fmt.Fprintln(out)
		printPackages(out, styler, absoluteMatches)

		fmt.Fprintln(out, styler.bold("* Other matches:"))
		fmt.Fprintln(out)
	}

	printPackages(out, styler, pkgs)

	return ctx.Err()
}

func printPackages(out io.Writer, styler textStyler, pkgs []search.SearchedPackage) {
	for i := range pkgs {
		printPackage(out, styler, &pkgs[i])
	}
}

func printPackage(out io.Writer, styler textStyler, pkg *search.SearchedPackage) {
	// Use the highlighted version of the package if available.
	if pkg.Highlighted != nil {
		pkg = pkg.Highlighted
	}

	path := pkg.Path
	// Fix red coloring when used with other attributes by replacing all
	// resets with the default color.
	path = strings.ReplaceAll(path, "\x1b[0m", "\x1b[39m")
	if pkg.Broken || pkg.UnsupportedPlatform {
		path = styler.strikethrough(path)
	}

	fmt.Fprint(out, "- ", path)
	fmt.Fprint(out, " ", styler.dim("("+pkg.Version+")"))
	if pkg.Unfree {
		fmt.Fprint(out, styler.dim(" (unfree)"))
	}
	if pkg.Broken {
		fmt.Fprint(out, styler.dim(" (broken)"))
	}
	if pkg.UnsupportedPlatform {
		fmt.Fprint(out, styler.dim(" (unsupported)"))
	}
	fmt.Fprint(out, "\n")

	fmt.Fprint(out, wrap(pkg.Description, "  "), "\n")

	if pkg.LongDescription != "" && pkg.Description != pkg.LongDescription {
		fmt.Fprint(out, styleLongDescription(styler, pkg.LongDescription), "\n")
	}
}

var (
	reFencedCodeBlock = regexp.MustCompile(`(?ms)\x60\x60\x60+\s*(.*?)\s*\x60\x60\x60+`)
	reInlineHyperlink = regexp.MustCompile(`(?m)\[(.*?)\]\n*\((http.*?)\)`)
	reInlineCode      = regexp.MustCompile(`(?m)\x60\x60?(.*?)\x60\x60?`)
)

// TODO: consider using goldmark?
func styleLongDescription(styler textStyler, text string) string {
	linkReplace := styler.bold("$1") + styler.with(dontEndStyle).dim(" ($2)")
	codeReplace := styler.bold("$1") + styler.with(dontEndStyle).dim("")

	for _, f := range []func(string) string{
		func(text string) string {
			var sb strings.Builder
			sb.Grow(len(text))

			var start int
			for _, is := range reFencedCodeBlock.FindAllStringSubmatchIndex(text, -1) {
				sb.WriteString(text[start:is[0]])
				start = is[1]
				for _, codeLine := range strings.Split(text[is[2]:is[3]], "\n") {
					sb.WriteString("\t")
					sb.WriteString(codeLine)
					sb.WriteString("\n")
				}
			}
			sb.WriteString(text[start:])

			return sb.String()
		},
		func(text string) string { return reInlineHyperlink.ReplaceAllString(text, linkReplace) },
		func(text string) string { return reInlineCode.ReplaceAllString(text, codeReplace) },
		func(text string) string { return wrap(text, "  ") },
		styler.dim,
	} {
		text = f(text)
	}

	return text
}

func wrap(text, indent string) string {
	width := min(termWidth(), 80)
	if width < 1 {
		return indent + text
	}

	d := new(doc.Package).Parser().Parse(text)
	pr := &comment.Printer{
		TextCodePrefix: indent + indent,
		TextPrefix:     indent,
		TextWidth:      width,
	}

	return string(pr.Text(d))
}

var termWidth = sync.OnceValue(func() int {
	termWidth, _, _ := term.GetSize(int(os.Stdout.Fd()))
	return termWidth
})
