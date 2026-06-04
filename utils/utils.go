package utils

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Run executes a command in dir (empty = current dir) with optional extra env,
// streams its output, and returns an error if it fails. Failures are
// propagated to the caller rather than swallowed.
func Run(dir, name string, args []string, envs ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(envs) > 0 {
		cmd.Env = append(os.Environ(), envs...)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// Make runs "make <target> -j<ncpu>" in src. target may contain several
// space-separated goals (e.g. "bzImage modules"). Returns an error on failure.
func Make(src, target string, envs ...string) error {
	fmt.Printf(">>> make %s (in %s)\n", target, src)
	args := append(strings.Fields(target), fmt.Sprintf("-j%d", runtime.NumCPU()))
	return Run(src, "make", args, envs...)
}

// ExtractTar extracts a tar archive into dst, creating dst if needed.
func ExtractTar(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("create dest dir %s: %w", dst, err)
	}
	return Run("", "tar", []string{"-xf", src, "-C", dst})
}
