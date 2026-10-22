package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const modulePath = "github.com/cameo-lang/cameo"

// runCompiled compiles the script at path as `cameo build` does, builds the
// program with the go command in a temporary module and runs it, and
// returns its exit status: the program's own, or 1 when compiling or
// building it failed.
func runCompiled(path string, stdout, stderr io.Writer) int {
	dir, err := os.MkdirTemp("", "cameo-run-")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	program, err := buildProgram(path, dir, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cmd := exec.Command(program)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// buildProgram writes the compiled script into dir as a module of its own
// that uses the cameo source this binary was built from, builds it, and
// returns the path of the program. The go command's output goes to stderr.
func buildProgram(path, dir string, stderr io.Writer) (string, error) {
	root, err := sourceRoot()
	if err != nil {
		return "", err
	}
	if err := build(path, dir, "main", io.Discard); err != nil {
		return "", err
	}
	goVersion, err := goDirective(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	mod := fmt.Sprintf("module cameorun\n\ngo %s\n\nrequire %s v0.0.0\n\nreplace %s => %s\n", goVersion, modulePath, modulePath, root)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		return "", err
	}
	// cameo's go.sum covers the modules the program needs, so the go
	// command downloads nothing that building cameo did not.
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), sum, 0o644); err != nil {
		return "", err
	}
	goCmd, err := exec.LookPath("go")
	if err != nil {
		return "", errors.New("cameo run: the go command is not on PATH; cameo run builds the compiled script with it")
	}
	program := filepath.Join(dir, "program")
	if runtime.GOOS == "windows" {
		program += ".exe"
	}
	cmd := exec.Command(goCmd, "build", "-mod=mod", "-o", program, ".")
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, stderr, stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cameo run: go build of the compiled script failed: %w", err)
	}
	return program, nil
}

// sourceRoot returns the directory of the cameo module this binary was
// built from, found through the path the compiler recorded for this file:
// a checkout for `go build`, the module cache for `go install`. The
// compiled script must build against that source, since cameo build wrote
// it for that version.
func sourceRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if ok && filepath.IsAbs(file) {
		root := filepath.Join(filepath.Dir(file), "..", "..", "..")
		if module, err := moduleOf(filepath.Join(root, "go.mod")); err == nil && module == modulePath {
			return root, nil
		}
	}
	return "", errors.New("cameo run: the cameo source this binary was built from is gone; build cameo again from a checkout or with go install, without -trimpath")
}

// moduleOf returns the module path a go.mod file declares.
func moduleOf(gomod string) (string, error) {
	return directive(gomod, "module")
}

// goDirective returns the Go version a go.mod file declares.
func goDirective(gomod string) (string, error) {
	return directive(gomod, "go")
}

// directive returns the argument of the first line of a go.mod file that
// starts with name.
func directive(gomod, name string) (string, error) {
	f, err := os.Open(gomod)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), name+" "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s: no %s line", gomod, name)
}
