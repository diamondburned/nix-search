package commoncmd

import (
	"context"
	"errors"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/urfave/cli/v3"
)

var Verbosity = 0

var Flags = []cli.Flag{
	&cli.BoolFlag{
		Name:    "verbose",
		Aliases: []string{"v"},
		Usage:   "verbosity level; 0 is least verbose (warn), 3 is most verbose",
		Config: cli.BoolConfig{
			Count: &Verbosity,
		},
		Action: func(c *cli.Context, _ bool) error {
			if Verbosity > 3 {
				return cli.Exit("verbosity level must be between 0 and 3", 1)
			}

			log := hclog.FromContext(c.Context)
			if Verbosity > 0 {
				log.SetLevel(hclog.Warn - hclog.Level(Verbosity))
			} else {
				log.SetLevel(hclog.Warn)
			}

			return nil
		},
	},
}

func JoinFlags(flags ...[]cli.Flag) []cli.Flag {
	var all []cli.Flag
	for _, f := range flags {
		all = append(all, f...)
	}
	return all
}

func Run(ctx context.Context, app *cli.App) {
	if err := app.RunContext(ctx, os.Args); err != nil {
		code := 1

		var codeError cli.ExitCoder
		if errors.As(err, &codeError) {
			code = codeError.ExitCode()
		}

		log := hclog.FromContext(ctx)
		log.Error("error", "err", err)

		os.Exit(code)
	}
}
