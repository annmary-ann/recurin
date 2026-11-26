package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
	"github.com/oapi-codegen/runtime"

	"github.com/annmary-ann/recurin/internal/api/gen"
)

// apiError is an error the API reports to the client. Its fields map onto the
// Error schema in api/openapi.yaml. cause is logged, never sent.
type apiError struct {
	status  int
	typ     gen.ErrorType
	code    string
	message string
	param   string
	cause   error
}

func (e *apiError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.code, e.message, e.cause)
	}
	return e.code + ": " + e.message
}

func (e *apiError) Unwrap() error { return e.cause }

func errInvalidRequest(code, message, param string) *apiError {
	return &apiError{status: http.StatusBadRequest, typ: gen.InvalidRequestError, code: code, message: message, param: param}
}

func errRouteNotFound(r *http.Request) *apiError {
	return &apiError{
		status: http.StatusNotFound, typ: gen.NotFoundError, code: "route_not_found",
		message: fmt.Sprintf("No route %s %s. See the API reference for available endpoints.", r.Method, r.URL.Path),
	}
}

func errMethodNotAllowed(r *http.Request) *apiError {
	return &apiError{
		status: http.StatusMethodNotAllowed, typ: gen.InvalidRequestError, code: "method_not_allowed",
		message: fmt.Sprintf("%s is not allowed on %s.", r.Method, r.URL.Path),
	}
}

func errInternal(cause error) *apiError {
	return &apiError{
		status: http.StatusInternalServerError, typ: gen.ApiError, code: "internal_error",
		message: "Something went wrong on our side. Retry later, quoting the request ID if it persists.",
		cause:   cause,
	}
}

// errorWriter writes apiErrors as JSON and logs the ones that are our fault.
type errorWriter struct {
	logger *slog.Logger
}

func (ew errorWriter) write(w http.ResponseWriter, r *http.Request, e *apiError) {
	if e.status >= 500 {
		ew.logger.ErrorContext(r.Context(), "request failed",
			"request_id", requestIDFrom(r.Context()), "method", r.Method, "path", r.URL.Path,
			"code", e.code, "err", e.cause)
	}
	body := gen.Error{Error: gen.ErrorDetail{
		Type:      e.typ,
		Code:      e.code,
		Message:   e.message,
		RequestId: requestIDFrom(r.Context()),
	}}
	if e.param != "" {
		body.Error.Param = &e.param
	}
	if e.status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer realm="recurin"`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.status)
	_ = runtime.JSONMarshalWrite(w, body) // the client is gone if this fails
}

// validationError turns a failed OpenAPI check into the API's error format.
func (ew errorWriter) validationError(_ context.Context, err error, w http.ResponseWriter, r *http.Request, opts nethttpmiddleware.ErrorHandlerOpts) {
	ew.write(w, r, classifyValidationError(err, r, opts.StatusCode))
}

func classifyValidationError(err error, r *http.Request, suggested int) *apiError {
	// Our authenticator's own errors carry the right code and status.
	var secErr *openapi3filter.SecurityRequirementsError
	if errors.As(err, &secErr) {
		for _, e := range secErr.Errors {
			var ae *apiError
			if errors.As(e, &ae) {
				return ae
			}
		}
		return errMissingKey()
	}
	switch {
	case errors.Is(err, routers.ErrPathNotFound):
		return errRouteNotFound(r)
	case errors.Is(err, routers.ErrMethodNotAllowed):
		return errMethodNotAllowed(r)
	}

	var reqErr *openapi3filter.RequestError
	if errors.As(err, &reqErr) {
		switch {
		case reqErr.Parameter != nil:
			return errInvalidRequest("parameter_invalid",
				fmt.Sprintf("Invalid %s parameter %q: %s", reqErr.Parameter.In, reqErr.Parameter.Name, reasonOf(reqErr)),
				reqErr.Parameter.Name)
		case reqErr.RequestBody != nil:
			return errInvalidRequest("body_invalid", "Invalid request body: "+reasonOf(reqErr), bodyField(reqErr))
		}
	}
	if suggested == http.StatusNotFound {
		return errRouteNotFound(r)
	}
	return errInvalidRequest("request_invalid", err.Error(), "")
}

// reasonOf gives the most specific explanation kin-openapi has.
func reasonOf(e *openapi3filter.RequestError) string {
	var se *openapi3.SchemaError
	if errors.As(e.Err, &se) && se.Reason != "" {
		return se.Reason
	}
	if e.Reason != "" {
		return e.Reason
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "invalid"
}

// bodyField names the JSON field a body schema error is about, as a dotted path.
func bodyField(e *openapi3filter.RequestError) string {
	var se *openapi3.SchemaError
	if !errors.As(e.Err, &se) {
		return ""
	}
	path := ""
	for i, p := range se.JSONPointer() {
		if i > 0 {
			path += "."
		}
		path += p
	}
	return path
}

// recoverPanics turns a handler panic into a 500 instead of a dropped connection.
func (ew errorWriter) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler { //nolint:errorlint // sentinel compared as net/http does
					panic(v)
				}
				ew.write(w, r, errInternal(fmt.Errorf("panic: %v", v)))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
