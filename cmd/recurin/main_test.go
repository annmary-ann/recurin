package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "no args shows usage", args: nil, wantCode: exitUsage, wantStderr: "Usage:"},
		{name: "help lists the three roles", args: []string{"--help"}, wantCode: exitOK, wantStdout: "migrate"},
		{name: "version", args: []string{"version"}, wantCode: exitOK, wantStdout: "recurin dev"},
		{name: "unknown command", args: []string{"bill"}, wantCode: exitUsage, wantStderr: `unknown command "bill"`},
		{name: "roles are stubbed", args: []string{"api"}, wantCode: exitError, wantStderr: "not implemented"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)

			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestHelpListsEveryRole(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	run([]string{"help"}, &stdout, &bytes.Buffer{})

	for _, role := range []string{"api", "worker", "migrate"} {
		if !strings.Contains(stdout.String(), "  "+role+" ") {
			t.Errorf("help does not list the %q role", role)
		}
	}
}
