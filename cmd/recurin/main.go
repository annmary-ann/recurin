// Command recurin is the recurin billing engine. One binary runs in one of
// three roles: the HTTP API, the background worker, or the schema migrator.
package main

import (
	"fmt"
	"io"
	"os"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `recurin: self-hostable subscription billing for India

Usage:
  recurin <command> [flags]

Commands:
  api       Serve the REST API and gateway webhook receivers
  worker    Run background jobs: billing runs, notices, charges, dunning, webhooks
  migrate   Apply database schema migrations
  version   Print the version
  help      Show this help

Run one binary per role; scale api and worker independently.
`

// Exit codes.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches to a role and returns the process exit code. It takes its
// arguments and output streams as parameters so tests can drive it.
func run(args []string, stdout, stderr io.Writer) int {
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
	case "api", "worker", "migrate":
		fmt.Fprintf(stderr, "recurin %s: not implemented yet\n", args[0])
		return exitError
	default:
		fmt.Fprintf(stderr, "recurin: unknown command %q\n\n%s", args[0], usage)
		return exitUsage
	}
}
