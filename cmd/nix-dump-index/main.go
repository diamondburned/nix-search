package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
	"libdb.so/nix-search/cmd/internal/commoncmd"
	"libdb.so/nix-search/search"
)

var opts = search.DefaultIndexPackageOpts

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
				Value:       opts.Channel,
				Destination: &opts.Channel,
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

	if err := app.RunContext(ctx, os.Args); err != nil {
		cli.HandleExitCoder(err)
	}
}

func mainAction(c *cli.Context) error {
	ctx := c.Context

	pkgs, err := search.IndexPackages(ctx, search.IndexPackagesOpts{
		Channel:     opts.Channel,
		Parallelism: opts.Parallelism,
	})
	if err != nil {
		return errors.Wrap(err, "failed to index packages")
	}

	if err := json.NewEncoder(os.Stdout).Encode(pkgs); err != nil {
		return errors.Wrap(err, "failed to encode packages into JSON")
	}

	return nil
}
