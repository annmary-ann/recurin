// Package api is recurin's HTTP layer: routing, middleware and the server
// lifecycle. Business logic lives in the module packages; handlers here only
// translate between HTTP and those modules.
//
// The contract is api/openapi.yaml. internal/api/gen is generated from it, the
// handlers implement its strict interface, and every request is validated
// against it before a handler runs, including which operations need an API key.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/annmary-ann/recurin/internal/api/gen"
	"github.com/annmary-ann/recurin/internal/platform/id"
	"github.com/annmary-ann/recurin/internal/platform/platformdb"
)

// Server timeouts. ReadHeaderTimeout guards against slow-loris clients.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// Queries is the data the HTTP layer reads. *platformdb.Queries implements it.
type Queries interface {
	AuthenticateAPIKey(ctx context.Context, secretHash []byte) (platformdb.AuthenticateAPIKeyRow, error)
	GetAccount(ctx context.Context, id id.AccountID) (platformdb.Account, error)
}

// Config is what the API needs to run.
type Config struct {
	Version string
	Queries Queries
	Logger  *slog.Logger
}

// New returns the HTTP handler for the recurin API.
func New(cfg Config) (http.Handler, error) {
	if cfg.Queries == nil {
		return nil, errors.New("api: Config.Queries is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	spec, err := gen.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("api: load embedded OpenAPI spec: %w", err)
	}

	errs := errorWriter{logger: cfg.Logger}
	auth := authenticator{queries: cfg.Queries}

	r := chi.NewRouter()
	r.Use(
		withRequestID,
		withNoStore,
		errs.recoverPanics,
		withAuthSlot,
		nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
			Options:              openapiOptions(auth.authenticate),
			ErrorHandlerWithOpts: errs.validationError,
			DoNotValidateServers: true,
		}),
	)
	// The validator answers unknown routes and methods first; these only fire
	// if chi and the spec ever disagree.
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		errs.write(w, r, errRouteNotFound(r))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		errs.write(w, r, errMethodNotAllowed(r))
	})

	strict := gen.NewStrictHandlerWithOptions(&handlers{version: cfg.Version, queries: cfg.Queries}, nil,
		gen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				errs.write(w, r, errInvalidRequest("body_invalid", err.Error(), ""))
			},
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				errs.write(w, r, errInternal(err))
			},
		})
	return gen.HandlerWithOptions(strict, gen.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			errs.write(w, r, errInvalidRequest("parameter_invalid", err.Error(), ""))
		},
	}), nil
}

// Serve serves h on ln until ctx is cancelled, then drains in-flight requests
// for up to shutdownTimeout. It returns nil after a clean shutdown.
func Serve(ctx context.Context, ln net.Listener, h http.Handler, logger *slog.Logger) error {
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	logger.InfoContext(ctx, "api listening", "addr", ln.Addr().String())

	select {
	case err := <-errCh:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	logger.InfoContext(ctx, "api shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
