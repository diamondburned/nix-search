package main

import (
	"context"
	"fmt"
	"go/doc"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
	"libdb.so/nix-search/cmd/internal/commoncmd"
	"libdb.so/nix-search/search"
	"libdb.so/nix-search/search/searchers/blugesearcher"
)

var opts = search.DefaultIndexPackageOpts

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
				Name:    "index",
				Aliases: []string{"i"},
				Usage:   "update the index before searching",
				Value:   false,
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
	indexPath := c.String("index-path")

	if c.Bool("index") {
		if c.IsSet("flake") {
			if c.IsSet("channel") {
				return errors.New("cannot set both --channel and --flake")
			}

			opts.Nixpkgs = c.String("flake")
		}

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

	out := io.WriteCloser(os.Stdout)

	if !c.Bool("no-pager") && termWidth() > 0 {
		pager := os.Getenv("PAGER")
		if pager == "" {
			pager = "less"
		}

		pagerCmd := exec.CommandContext(ctx, pager)
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
	}

	defer out.Close()

	pkgsCh, err := searcher.SearchPackages(ctx, query, search.Opts{
		Highlight: search.HighlightStyleANSI{},
	})
	if err != nil {
		return errors.Wrap(err, "failed to search packages")
	}

	for pkg := range pkgsCh {
		fmt.Fprintf(out, "- %s (%s)\n", pkg.Path, pkg.Version)
		fmt.Fprintf(out, "%s\n", wrap(pkg.Description, "  "))
	}

	return ctx.Err()
}

func wrap(text, indent string) string {
	width := termWidth()
	if width == 0 {
		return indent + text
	}

	if width > 80 {
		width = 80
	}

	var out strings.Builder
	doc.ToText(&out, text, indent, "", width)
	return out.String()
}

var width = -1

func termWidth() int {
	if width == -1 {
		width, _, _ = term.GetSize(int(os.Stdout.Fd()))
	}
	return width
}
