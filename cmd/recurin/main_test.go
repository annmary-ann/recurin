package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func noEnv(string) string { return "" }

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
		{name: "help lists the roles", args: []string{"--help"}, wantCode: exitOK, wantStdout: "migrate"},
		{name: "version", args: []string{"version"}, wantCode: exitOK, wantStdout: "recurin dev"},
		{name: "unknown command", args: []string{"bill"}, wantCode: exitUsage, wantStderr: `unknown command "bill"`},
		{name: "worker is stubbed", args: []string{"worker"}, wantCode: exitError, wantStderr: "not implemented"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := run(t.Context(), tt.args, noEnv, &stdout, &stderr)

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
	run(t.Context(), []string{"help"}, noEnv, &stdout, &bytes.Buffer{})

	for _, role := range []string{"api", "worker", "migrate"} {
		if !strings.Contains(stdout.String(), "  "+role+" ") {
			t.Errorf("help does not list the %q role", role)
		}
	}
}

func TestHealthcheck(t *testing.T) {
	t.Parallel()

	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(healthy.Close)
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(failing.Close)

	tests := []struct {
		name     string
		addr     string
		wantCode int
	}{
		{name: "healthy API", addr: healthy.Listener.Addr().String(), wantCode: exitOK},
		{name: "API returns 503", addr: failing.Listener.Addr().String(), wantCode: exitError},
		{name: "nothing listening", addr: "127.0.0.1:1", wantCode: exitError},
		{name: "malformed address", addr: "not-an-address", wantCode: exitError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := func(k string) string {
				if k == "RECURIN_HTTP_ADDR" {
					return tt.addr
				}
				return ""
			}
			if code := run(t.Context(), []string{"healthcheck"}, env, &bytes.Buffer{}, &bytes.Buffer{}); code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
		})
	}
}
