package search

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	_ "embed"

	"github.com/pkg/errors"
)

//go:embed nix/dump_packages.nix
var nixExprDumpPackages string

type packagesDump map[string]struct {
	Description string `json:"description"`
	Version     string `json:"version"`
	Broken      bool   `json:"broken"`
	HasMore     bool   `json:"hasMore"`
}

// dumpPackages returns a list of all packages in the given channel.
func dumpPackages(ctx context.Context, nixpkgs string, attrs []string) (packagesDump, error) {
	stdout, err := execCommandWriter(ctx,
		"nix-instantiate", "--eval", "--json", "--strict",
		"-E", nixExprDumpPackages,
		"--arg", "nixpkgs", nixpkgs,
		"--arg", "attrs", toNixArray(attrs))
	if err != nil {
		return nil, err
	}
	defer stdout.Close()

	var packages packagesDump
	if err := json.NewDecoder(stdout).Decode(&packages); err != nil {
		return nil, errors.Wrap(err, "failed to parse packages dump")
	}

	if err := stdout.Close(); err != nil {
		return nil, errors.Wrap(err, "failed to close process reader")
	}

	return packages, nil
}

func toNixArray(args []string) string {
	var b strings.Builder
	b.WriteString("[")
	for _, arg := range args {
		b.WriteString(strconv.Quote(arg))
		b.WriteByte(' ')
	}
	b.WriteString("]")
	return b.String()
}

// ResolveNixPathFromFlake returns the flake-locked Nix store path for the given flake.
// Using this path, one can directly do `import (path) { }` to evaluate the
// Nixpkgs instance like using a channel.
func ResolveNixPathFromFlake(ctx context.Context, flake string) (string, error) {
	stdout, err := execCommand(ctx, "nix", "flake", "metadata", flake, "--json")
	if err != nil {
		return "", err
	}

	var output struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		return "", errors.Wrap(err, "failed to parse flake metadata")
	}

	return output.Path, nil
}
