package commoncmd

import (
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

			if Verbosity > 0 {
				log := hclog.FromContext(c.Context)
				log.SetLevel(hclog.Warn - hclog.Level(Verbosity))
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
