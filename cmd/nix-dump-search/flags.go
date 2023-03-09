package main

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
)

type levelFlag hclog.Level

func (f *levelFlag) String() string {
	return hclog.Level(*f).String()
}

func (f *levelFlag) Set(s string) error {
	verbosity := hclog.LevelFromString(s)
	if verbosity == hclog.NoLevel {
		return fmt.Errorf("invalid verbosity level %q", s)
	}
	*f = levelFlag(verbosity)
	return nil
}
