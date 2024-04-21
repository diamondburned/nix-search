package search

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
)

// CommandError is an error that is returned when a command fails.
type CommandError struct {
	Stderr []byte
	err    error
}

func (err *CommandError) Error() string {
	var exitErr *exec.ExitError
	if errors.As(err.err, &exitErr) {
		if len(err.Stderr) > 0 {
			return strings.TrimRight(string(err.Stderr), "\n")
		}
		return exitErr.Error()
	}
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
			"duration", endedAt.Sub(startedAt),
			"stderr", stderr.String(),
		)
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
