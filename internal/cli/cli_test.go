package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/zewelor/feedway/internal/config"
)

func TestRunUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing command"},
		{name: "unknown command", args: []string{"unknown"}},
		{name: "too many arguments", args: []string{"serve", "extra"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			exitCode := Run(
				test.args,
				validEnvironment,
				&stderr,
				func(config.Config) error { return nil },
				func(config.Config) error { return nil },
			)

			if exitCode != 2 {
				t.Fatalf("Run() = %d, want 2", exitCode)
			}
			if stderr.String() != usage {
				t.Fatalf("stderr = %q, want %q", stderr.String(), usage)
			}
		})
	}
}

func TestRunDispatchesCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
	}{
		{name: "serve", command: "serve"},
		{name: "migrate", command: "migrate"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var called string
			exitCode := Run(
				[]string{test.command},
				validEnvironment,
				&bytes.Buffer{},
				func(config.Config) error {
					called = "serve"
					return nil
				},
				func(config.Config) error {
					called = "migrate"
					return nil
				},
			)

			if exitCode != 0 {
				t.Fatalf("Run() = %d, want 0", exitCode)
			}
			if called != test.command {
				t.Fatalf("called = %q, want %q", called, test.command)
			}
		})
	}
}

func TestRunReportsConfigurationError(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	exitCode := Run(
		[]string{"serve"},
		func(string) (string, bool) { return "", false },
		&stderr,
		func(config.Config) error { return nil },
		func(config.Config) error { return nil },
	)

	if exitCode != 1 {
		t.Fatalf("Run() = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "DATABASE_URL is required") {
		t.Fatalf("stderr = %q, want configuration error", stderr.String())
	}
}

func TestRunReportsCommandError(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	exitCode := Run(
		[]string{"serve"},
		validEnvironment,
		&stderr,
		func(config.Config) error { return errors.New("serve failed") },
		func(config.Config) error { return nil },
	)

	if exitCode != 1 {
		t.Fatalf("Run() = %d, want 1", exitCode)
	}
	if stderr.String() != "feedway: serve failed\n" {
		t.Fatalf("stderr = %q, want command error", stderr.String())
	}
}

func validEnvironment(name string) (string, bool) {
	environment := map[string]string{
		"DATABASE_URL": "postgres://feedway:secret@postgres/feedway",
		"API_TOKEN":    strings.Repeat("a", 32),
	}
	value, exists := environment[name]
	return value, exists
}
