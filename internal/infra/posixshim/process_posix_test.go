//go:build linux || darwin

package posixshim

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestExecutableShimExecsBoundRuntimeAcrossAvailableShells(t *testing.T) {
	temporary := t.TempDir()
	runtimePath := filepath.Join(temporary, "runtime")
	writeExecutable(t, runtimePath, []byte("#!/bin/sh\nprintf 'runtime-pid=%s\\n' \"$$\"\nprintf 'runtime-stderr\\n' >&2\nexit 23\n"))
	material, err := Render(testBinding(t, runtimePath, filepath.Join(temporary, "bundle.json"), oneHelp()))
	if err != nil {
		t.Fatal(err)
	}
	shimPath := filepath.Join(temporary, "shim")
	writeExecutable(t, shimPath, material.Source)

	shells := []struct {
		path string
		args []string
	}{
		{path: "/bin/sh"},
		{path: "/bin/dash"},
		{path: "/bin/bash", args: []string{"--posix"}},
		{path: "/bin/zsh", args: []string{"--emulate", "sh"}},
	}
	for _, shell := range shells {
		if _, err := os.Stat(shell.path); err != nil {
			continue
		}
		t.Run(filepath.Base(shell.path), func(t *testing.T) {
			args := append(append([]string{}, shell.args...), shimPath, "issue", "list")
			command := exec.Command(shell.path, args...)
			stdout, err := command.Output()
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) || exitError.ExitCode() != 23 || string(exitError.Stderr) != "runtime-stderr\n" {
				t.Fatalf("runtime result = %v, stderr=%q", err, exitError.Stderr)
			}
			want := fmt.Sprintf("runtime-pid=%d\n", command.Process.Pid)
			if string(stdout) != want {
				t.Fatalf("runtime pid output = %q, want %q; shim shell was not replaced", stdout, want)
			}
		})
	}
}

func TestExecutableShimDeliversTerminationToBoundRuntimePID(t *testing.T) {
	temporary := t.TempDir()
	runtimePath := filepath.Join(temporary, "runtime")
	writeExecutable(t, runtimePath, []byte("#!/bin/sh\ntrap 'exit 42' TERM\nprintf '%s\\n' \"$$\"\nwhile :; do :; done\n"))
	material, err := Render(testBinding(t, runtimePath, filepath.Join(temporary, "bundle.json"), oneHelp()))
	if err != nil {
		t.Fatal(err)
	}
	shimPath := filepath.Join(temporary, "shim")
	writeExecutable(t, shimPath, material.Source)

	command := exec.Command("/bin/sh", shimPath, "issue", "list")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	finished := false
	defer func() {
		if !finished {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			_, _ = command.Process.Wait()
		}
	}()

	line := make(chan string, 1)
	readError := make(chan error, 1)
	go func() {
		value, readErr := bufio.NewReader(stdout).ReadString('\n')
		if readErr != nil {
			readError <- readErr
			return
		}
		line <- value
	}()
	var runtimePID int
	select {
	case value := <-line:
		runtimePID, err = strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			t.Fatalf("runtime pid %q: %v", value, err)
		}
	case err := <-readError:
		t.Fatalf("read runtime pid: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("runtime did not become ready")
	}
	if runtimePID != command.Process.Pid {
		t.Fatalf("runtime pid = %d, shim pid = %d; shim retained an intermediate process", runtimePID, command.Process.Pid)
	}
	if err := syscall.Kill(command.Process.Pid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	err = command.Wait()
	finished = true
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 42 {
		t.Fatalf("termination result = %v", err)
	}
}
