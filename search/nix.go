package search

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	_ "embed"

	"github.com/hashicorp/go-hclog"
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
func dumpPackages(ctx context.Context, channel string, attrs []string) (packagesDump, error) {
	stdout, err := execCommandWriter(ctx,
		"nix-instantiate", "--eval", "--json", "--strict",
		"-E", nixExprDumpPackages,
		"--arg", "channel", channel,
		"--arg", "attrs", toNixArray(attrs))
	if err != nil {
		return nil, errors.Wrap(err, "failed to dump packages")
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

// CommandError is an error that is returned when a command fails.
type CommandError struct {
	Stderr []byte
	err    error
}

func (err *CommandError) Error() string {
	return err.err.Error()
}

func (err *CommandError) Unwrap() error {
	return err.err
}

func execCommand(ctx context.Context, arg0 string, argv ...string) (string, error) {
	logger := hclog.FromContext(ctx)
	logger.Trace("executing command", "command", arg0, "args", argv)

	var stderr bytes.Buffer
	var stdout bytes.Buffer

	cmd := exec.CommandContext(ctx, arg0, argv...)
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", &CommandError{
			Stderr: stderr.Bytes(),
			err:    errors.Wrapf(err, "failed to run command: %q", arg0),
		}
	}

	return stdout.String(), nil
}

func execCommandWriter(ctx context.Context, arg0 string, argv ...string) (io.ReadCloser, error) {
	logger := hclog.FromContext(ctx)
	logger.Trace("executing command", "command", arg0, "args", argv)

	var stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, arg0, argv...)
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get stdout pipe")
	}

	startedAt := time.Now()
	onDone := func() {
		endedAt := time.Now()
		logger.Trace(
			"command finished",
			"command", arg0,
			"args", argv,
			"duration", endedAt.Sub(startedAt))
	}

	if err := cmd.Start(); err != nil {
		return nil, &CommandError{
			Stderr: stderr.Bytes(),
			err:    errors.Wrapf(err, "failed to run command: %q", arg0),
		}
	}

	return &cmdWriter{stdout, cmd, onDone}, nil
}

type cmdWriter struct {
	io.ReadCloser
	cmd    *exec.Cmd
	onDone func()
}

func (c *cmdWriter) Close() error {
	defer func() {
		if c.onDone != nil {
			c.onDone()
			c.onDone = nil
		}
	}()

	err := c.ReadCloser.Close()
	if err != nil {
		c.cmd.Process.Kill()
		return err
	}

	return c.cmd.Wait()
}
