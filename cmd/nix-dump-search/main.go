package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"

	"github.com/diamondburned/nix-search/search"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/pflag"
)

var (
	channel     = search.DefaultIndexPackageOpts.Channel
	verbosity   = 0
	parallelism = search.DefaultIndexPackageOpts.Parallelism
)

func main() {
	pflag.StringVarP(&channel, "channel", "c", channel, "channel path to index")
	pflag.CountVarP(&verbosity, "verbose", "v", "verbosity level, default: info")
	pflag.IntVarP(&parallelism, "max-jobs", "j", parallelism, "max parallel jobs")
	pflag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	logLevel := hclog.Info

	if verbosity > int(logLevel) {
		verbosity = int(logLevel)
	}

	logLevel -= hclog.Level(verbosity)

	logger := hclog.Default()
	logger.SetLevel(logLevel)

	ctx = hclog.WithContext(ctx, logger)

	pkgs, err := search.IndexPackages(ctx, search.IndexPackagesOpts{
		Channel:     channel,
		Parallelism: parallelism,
	})
	if err != nil {
		log.Fatalln("Failed to index packages:", err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(pkgs); err != nil {
		log.Fatalln("Failed to encode packages into JSON:", err)
	}
}
