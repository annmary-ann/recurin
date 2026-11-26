package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/annmary-ann/recurin/internal/platform/apikey"
	"github.com/annmary-ann/recurin/internal/platform/id"
	"github.com/annmary-ann/recurin/internal/platform/platformdb"
	"github.com/annmary-ann/recurin/internal/platform/postgres"
)

const accountUsage = `Usage: recurin account create <name>

Creates an account and its first secret key (test mode). The key is printed
once and cannot be shown again. Needs RECURIN_DATABASE_URL.
`

const keysUsage = `Usage:
  recurin apikey create <acct_id> <test|live> [name]
  recurin apikey revoke <key_id>

Keys are printed once and cannot be shown again. Needs RECURIN_DATABASE_URL.
`

const keyWarning = "Store the key now; recurin keeps only its hash and cannot show it again."

func runAccount(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) != 2 || args[0] != "create" {
		fmt.Fprint(stderr, accountUsage)
		return exitUsage
	}
	name := args[1]
	return withPool(ctx, getenv, stderr, "account create", func(pool *pgxpool.Pool) error {
		var acct platformdb.Account
		var secret apikey.Secret
		err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			q := platformdb.New(tx)
			var err error
			acct, err = q.CreateAccount(ctx, platformdb.CreateAccountParams{
				ID: id.New[id.Account](), Name: name, Metadata: json.RawMessage(`{}`),
			})
			if err != nil {
				return fmt.Errorf("create account: %w", err)
			}
			secret, _, err = createKey(ctx, q, acct.ID, apikey.Test, "default")
			return err
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "account  %s  %s\napi key  %s  (test mode)\n\n%s\n", acct.ID, acct.Name, secret.Text, keyWarning)
		return nil
	})
}

func runAPIKey(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	switch {
	case len(args) >= 3 && len(args) <= 4 && args[0] == "create":
		acctID, err := id.Parse[id.Account](args[1])
		if err != nil {
			fmt.Fprintf(stderr, "recurin apikey create: %v\n", err)
			return exitUsage
		}
		mode, err := apikey.ParseMode(args[2])
		if err != nil {
			fmt.Fprintf(stderr, "recurin apikey create: %v\n", err)
			return exitUsage
		}
		name := ""
		if len(args) == 4 {
			name = args[3]
		}
		return withPool(ctx, getenv, stderr, "apikey create", func(pool *pgxpool.Pool) error {
			secret, key, err := createKey(ctx, platformdb.New(pool), acctID, mode, name)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "key      %s\napi key  %s  (%s mode)\n\n%s\n", key.ID, secret.Text, mode, keyWarning)
			return nil
		})

	case len(args) == 2 && args[0] == "revoke":
		keyID, err := id.Parse[id.APIKey](args[1])
		if err != nil {
			fmt.Fprintf(stderr, "recurin apikey revoke: %v\n", err)
			return exitUsage
		}
		return withPool(ctx, getenv, stderr, "apikey revoke", func(pool *pgxpool.Pool) error {
			key, err := platformdb.New(pool).RevokeAPIKey(ctx, keyID)
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("no active key %s", keyID)
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "revoked  %s  %s\n", key.ID, key.SecretHint)
			return nil
		})

	default:
		fmt.Fprint(stderr, keysUsage)
		return exitUsage
	}
}

func createKey(ctx context.Context, q *platformdb.Queries, acct id.AccountID, mode apikey.Mode, name string) (apikey.Secret, platformdb.ApiKey, error) {
	secret, err := apikey.Generate(mode)
	if err != nil {
		return apikey.Secret{}, platformdb.ApiKey{}, err
	}
	key, err := q.CreateAPIKey(ctx, platformdb.CreateAPIKeyParams{
		ID:         id.New[id.APIKey](),
		AccountID:  acct,
		Mode:       string(mode),
		Name:       name,
		SecretHash: secret.Hash[:],
		SecretHint: secret.Hint,
	})
	if err != nil {
		return apikey.Secret{}, platformdb.ApiKey{}, fmt.Errorf("create api key: %w", err)
	}
	return secret, key, nil
}

func withPool(ctx context.Context, getenv func(string) string, stderr io.Writer, cmd string, fn func(*pgxpool.Pool) error) int {
	dsn := getenv("RECURIN_DATABASE_URL")
	if dsn == "" {
		fmt.Fprintf(stderr, "recurin %s: RECURIN_DATABASE_URL is not set\n", cmd)
		return exitError
	}
	pool, err := postgres.Connect(ctx, dsn)
	if err != nil {
		fmt.Fprintf(stderr, "recurin %s: %v\n", cmd, err)
		return exitError
	}
	defer pool.Close()
	if err := fn(pool); err != nil {
		fmt.Fprintf(stderr, "recurin %s: %v\n", cmd, err)
		return exitError
	}
	return exitOK
}
