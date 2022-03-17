package utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/palantir/stacktrace"
)

// Version returns doriath version
func Version() string {
	return "1.5.1"
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

// Info .
func Info(msg string, args ...interface{}) {
	color.New(color.FgBlue).Printf(appendNewLine(msg), args...)
}

// Info2 .
func Info2(msg string, args ...interface{}) {
	color.New(color.FgCyan).Printf(appendNewLine(msg), args...)
}

// Warn .
func Warn(msg string, args ...interface{}) {
	color.New(color.FgYellow).Printf(appendNewLine(msg), args...)
}

// Success .
func Success(msg string, args ...interface{}) {
	color.New(color.FgGreen).Printf(appendNewLine(msg), args...)
}

// Error .
func Error(err error) {
	debug := os.Getenv("DEBUG")
	if debug == "true" || debug == "1" {
		color.New(color.FgRed).Fprintln(os.Stderr, err)
	} else {
		color.New(color.FgRed).Fprintln(os.Stderr, stacktrace.RootCause(err))
	}
}

// Fatal .
func Fatal(err error) {
	Error(err)
	os.Exit(1)
}

func appendNewLine(msg string) string {
	if strings.HasSuffix(msg, "\n") {
		return msg
	}
	return msg + "\n"
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

// RetryWithFixedDelay .
func RetryWithFixedDelay(delay time.Duration, retries int, f func() error) error {
	var err error
	for i := 0; i < retries; i++ {
		err = f()
		if err == nil {
			return nil
		}
		time.Sleep(delay)
	}
	return err
}
