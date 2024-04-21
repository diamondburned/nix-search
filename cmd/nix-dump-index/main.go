package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
	"libdb.so/nix-search/cmd/internal/commoncmd"
	"libdb.so/nix-search/search"
)

var (
	opts  = search.DefaultIndexPackageOpts
	flake string
)

var app = cli.App{
	Name:  "nix-dump-index",
	Usage: "Dump a new index of packages to stdout.",
	Flags: commoncmd.JoinFlags(
		commoncmd.Flags,
		[]cli.Flag{
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
				Usage:       "max parallel jobs",
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

	if c.IsSet("flake") {
		if c.IsSet("channel") {
			return errors.New("cannot set both --channel and --flake")
		}

		opts.Nixpkgs = c.String("flake")
	}

	pkgs, err := search.IndexPackages(ctx, opts)
	if err != nil {
		return errors.Wrap(err, "failed to index packages")
	}

	if err := json.NewEncoder(os.Stdout).Encode(pkgs); err != nil {
		return errors.Wrap(err, "failed to encode packages into JSON")
	}

	return nil
}
