package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/palantir/stacktrace"
)

// Version returns doriath version
func Version() string {
	return "1.4.0"
}

// ResolveDir appends a path to a rootDir
func ResolveDir(rootDir, path string) string {
	if path == "provided" {
		return "provided"
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return filepath.Join(rootDir, path)
}

// PrintError prints additional stack trace if DEBUG is set to true
func PrintError(err error) {
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
	} else {
		fmt.Fprintln(os.Stderr, "ERROR:", stacktrace.RootCause(err))
	}
}

// DetectRequirement detects requirement for doriath
func DetectRequirement() error {
	return detectDocker()
}

func detectDocker() error {
	cmd := exec.Command("docker")
	return stacktrace.Propagate(cmd.Run(), "Cannot find docker command")
}

// StringSet is a set of string
type StringSet map[string]struct{}

// Add adds a value to the string set
func (s StringSet) Add(value string) {
	s[value] = struct{}{}
}

// Remove removes a value from the string set
func (s StringSet) Remove(value string) {
	delete(s, value)
}

// Exists checks if the string set contains a value or not
func (s StringSet) Exists(value string) bool {
	_, ok := s[value]
	return ok
}

// RunShellCommand runs a command under bash shell
func RunShellCommand(command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
