// Command recurin is the recurin billing engine. One binary runs in one of
// three roles: the HTTP API, the background worker, or the schema migrator.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/annmary-ann/recurin/internal/api"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `recurin: self-hostable subscription billing for India

Usage:
  recurin <command> [flags]

Commands:
  api          Serve the REST API and gateway webhook receivers
  worker       Run background jobs: billing runs, notices, charges, dunning, webhooks
  migrate      Apply database schema migrations
  healthcheck  Exit 0 if the local API answers /healthz (for container health checks)
  version      Print the version
  help         Show this help

Run one binary per role; scale api and worker independently.

Environment:
  RECURIN_HTTP_ADDR   Address the API listens on (default ":8080")
`

// Exit codes.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

const defaultHTTPAddr = ":8080"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run dispatches to a role and returns the process exit code. It takes its
// arguments, environment and output streams as parameters so tests can drive it.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}

	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	case "version", "--version":
		fmt.Fprintf(stdout, "recurin %s\n", version)
		return exitOK
	case "api":
		return runAPI(ctx, getenv, stderr)
	case "migrate":
		// Real migrations arrive with the database baseline (#8).
		fmt.Fprintln(stdout, "recurin migrate: no migrations yet")
		return exitOK
	case "worker":
		fmt.Fprintln(stderr, "recurin worker: not implemented yet")
		return exitError
	case "healthcheck":
		return runHealthcheck(ctx, getenv, stderr)
	default:
		fmt.Fprintf(stderr, "recurin: unknown command %q\n\n%s", args[0], usage)
		return exitUsage
	}
}

func httpAddr(getenv func(string) string) string {
	if addr := getenv("RECURIN_HTTP_ADDR"); addr != "" {
		return addr
	}
	return defaultHTTPAddr
}

func runAPI(ctx context.Context, getenv func(string) string, stderr io.Writer) int {
	logger := slog.New(slog.NewJSONHandler(stderr, nil)).With("role", "api", "version", version)

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", httpAddr(getenv))
	if err != nil {
		logger.ErrorContext(ctx, "listen failed", "err", err)
		return exitError
	}
	if err := api.Serve(ctx, ln, api.Handler(version), logger); err != nil {
		logger.ErrorContext(ctx, "api stopped with an error", "err", err)
		return exitError
	}
	return exitOK
}

// runHealthcheck calls the API's /healthz on the loopback interface. Distroless
// images have no curl or shell, so the binary checks itself.
func runHealthcheck(ctx context.Context, getenv func(string) string, stderr io.Writer) int {
	host, port, err := net.SplitHostPort(httpAddr(getenv))
	if err != nil {
		fmt.Fprintf(stderr, "healthcheck: bad RECURIN_HTTP_ADDR: %v\n", err)
		return exitError
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	url := "http://" + net.JoinHostPort(host, port) + "/healthz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		fmt.Fprintf(stderr, "healthcheck: %v\n", err)
		return exitError
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "healthcheck: %v\n", err)
		return exitError
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stderr, "healthcheck: %s returned %d\n", url, resp.StatusCode)
		return exitError
	}
	return exitOK
}
