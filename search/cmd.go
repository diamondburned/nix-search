package search

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
)

// CommandError is an error that is returned when a command fails.
type CommandError struct {
	cmd *exec.Cmd
	err error
}

func (err *CommandError) Error() string {
	return fmt.Sprintf("%s: %v", err.cmd.Args[0], err.err)
}

func (err *CommandError) Unwrap() error {
	return err.err
}

func execCommand(ctx context.Context, arg0 string, argv ...string) (string, error) {
	logger := hclog.FromContext(ctx)
	logger.Trace("executing command", "command", arg0, "args", argv)

	var stdout bytes.Buffer

	cmd := exec.CommandContext(ctx, arg0, argv...)
	cmd.Stderr = newStderrLogger(ctx, cmd)
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", &CommandError{
			cmd: cmd,
			err: err,
		}
	}

	cmd.Stderr.(*stderrLogger).Flush()
	return stdout.String(), nil
}

func execCommandWriter(ctx context.Context, arg0 string, argv ...string) (io.ReadCloser, error) {
	logger := hclog.FromContext(ctx)
	logger.Trace("executing command", "command", arg0, "args", argv)

	cmd := exec.CommandContext(ctx, arg0, argv...)
	cmd.Stderr = newStderrLogger(ctx, cmd)

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
		)
	}

	if err := cmd.Start(); err != nil {
		return nil, &CommandError{
			cmd: cmd,
			err: err,
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

	if err := c.ReadCloser.Close(); err != nil {
		c.cmd.Process.Kill()
		return err
	}

	if err := c.cmd.Wait(); err != nil {
		return &CommandError{
			cmd: c.cmd,
			err: err,
		}
	}

	c.cmd.Stderr.(*stderrLogger).Flush()
	return nil
}

type stderrLogger struct {
	buffer bytes.Buffer
	logger hclog.Logger
}

func newStderrLogger(ctx context.Context, cmd *exec.Cmd) *stderrLogger {
	return &stderrLogger{
		logger: hclog.FromContext(ctx).Named(cmd.Args[0]),
	}
}

func (l *stderrLogger) Write(p []byte) (n int, err error) {
	l.buffer.Write(p)
	return len(p), l.Flush()
}

func (l *stderrLogger) Flush() error {
	for bytes.Contains(l.buffer.Bytes(), []byte{'\n'}) {
		line, err := l.buffer.ReadString('\n')
		if err != nil {
			panic(fmt.Sprintf("cannot read line: %v", err))
		}
		l.logger.Debug(strings.TrimRight(line, "\n"))
	}
	return nil
}
