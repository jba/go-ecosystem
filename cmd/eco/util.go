package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// RunCommand runs the command with the given arguments in a new process.
// It returns the standard output. If the command fails, the error includes
// the standard error.
func RunCommand(ctx context.Context, com string, args ...string) (out []byte, err error) {
	return RunCommandInDir(ctx, "", com, args...)
}

func RunCommandInDir(ctx context.Context, dir, com string, args ...string) (out []byte, err error) {
	cmd := exec.CommandContext(ctx, com, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err = cmd.Output()
	if err != nil {
		cs := com + " " + strings.Join(args, " ")
		return nil, fmt.Errorf("'%s' returned %s", cs, includeStderr(err))
	}
	return out, nil
}

// includeStderr includes the stderr with an *exec.ExitError.
// If err is not an *exec.ExitError, it returns err.Error().
func includeStderr(err error) string {
	var eerr *exec.ExitError
	if errors.As(err, &eerr) {
		return fmt.Sprintf("%v: %s", eerr, bytes.TrimSpace(eerr.Stderr))
	}
	return err.Error()
}

func GoEnv(envvar string) (string, error) {
	out, err := RunCommand(context.Background(), "go", "env", envvar)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
