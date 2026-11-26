package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/annmary-ann/recurin/internal/api"
	"github.com/annmary-ann/recurin/internal/api/gen"
	"github.com/annmary-ann/recurin/internal/platform/apikey"
	"github.com/annmary-ann/recurin/internal/platform/id"
	"github.com/annmary-ann/recurin/internal/platform/platformdb"
	"github.com/annmary-ann/recurin/internal/platform/postgres/pgtest"
)

// fixture is an API backed by a real database with one account and keys.
type fixture struct {
	handler        http.Handler
	account        platformdb.Account
	testKey        string
	liveKey        string
	revokedKey     string
	otherAcctKey   string
	otherAccountID id.AccountID
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	q := platformdb.New(pgtest.New(t))
	ctx := t.Context()

	newAccount := func(name string) platformdb.Account {
		a, err := q.CreateAccount(ctx, platformdb.CreateAccountParams{
			ID: id.New[id.Account](), Name: name, Metadata: json.RawMessage(`{"region":"south"}`),
		})
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	newKey := func(acct id.AccountID, mode apikey.Mode) (string, id.APIKeyID) {
		s, err := apikey.Generate(mode)
		if err != nil {
			t.Fatal(err)
		}
		k, err := q.CreateAPIKey(ctx, platformdb.CreateAPIKeyParams{
			ID: id.New[id.APIKey](), AccountID: acct, Mode: string(mode), SecretHash: s.Hash[:], SecretHint: s.Hint,
		})
		if err != nil {
			t.Fatal(err)
		}
		return s.Text, k.ID
	}

	f := fixture{account: newAccount("Acme Learning")}
	f.testKey, _ = newKey(f.account.ID, apikey.Test)
	f.liveKey, _ = newKey(f.account.ID, apikey.Live)
	var revokedID id.APIKeyID
	f.revokedKey, revokedID = newKey(f.account.ID, apikey.Test)
	if _, err := q.RevokeAPIKey(ctx, revokedID); err != nil {
		t.Fatal(err)
	}
	other := newAccount("Other Co")
	f.otherAccountID = other.ID
	f.otherAcctKey, _ = newKey(other.ID, apikey.Test)

	h, err := api.New(api.Config{Version: "v1.2.3", Queries: q})
	if err != nil {
		t.Fatal(err)
	}
	f.handler = h
	return f
}

// call sends a request, checks the response against api/openapi.yaml when the
// route is in the spec, and checks the headers every response carries.
func call(t *testing.T, h http.Handler, method, path, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, http.NoBody)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	rid := rec.Header().Get("Request-Id")
	if _, err := id.Parse[id.Request](rid); err != nil {
		t.Errorf("Request-Id header %q: %v", rid, err)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	validateAgainstSpec(t, req, rec)
	return rec
}

func validateAgainstSpec(t *testing.T, req *http.Request, rec *httptest.ResponseRecorder) {
	t.Helper()
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	spec.Servers = nil
	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatal(err)
	}
	route, params, err := router.FindRoute(req)
	if err != nil {
		return // not a spec route (404/405 tests); those tests check the body themselves
	}
	err = openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request: req, PathParams: params, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		},
		Status: rec.Code,
		Header: rec.Header(),
		Body:   io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
	})
	if err != nil {
		t.Errorf("%s %s -> %d does not match the spec: %v\nbody: %s", req.Method, req.URL.Path, rec.Code, err, rec.Body)
	}
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) gen.ErrorDetail {
	t.Helper()
	var body gen.Error
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body %q: %v", rec.Body, err)
	}
	if body.Error.RequestId != rec.Header().Get("Request-Id") {
		t.Errorf("request_id %q differs from header %q", body.Error.RequestId, rec.Header().Get("Request-Id"))
	}
	return body.Error
}

func TestHealthzNeedsNoKey(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	rec := call(t, f.handler, http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var body gen.Health
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Status != gen.Ok || body.Version != "v1.2.3" {
		t.Fatalf("body = %+v, %v", body, err)
	}
}

func TestUnauthenticatedCallsGet401(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	malformed := "sk_test_" + strings.Repeat("u", 52)
	cases := []struct {
		name, authorization, wantCode, wantMessage string
	}{
		{"no header", "", "api_key_missing", "No API key provided"},
		{"not bearer", "Basic " + f.testKey, "api_key_missing", "No API key provided"},
		{"empty bearer", "Bearer ", "api_key_missing", "No API key provided"},
		{"malformed key", "Bearer " + malformed, "api_key_invalid", "Keys look like"},
		{"publishable-style key", "Bearer pk_test_abc", "api_key_invalid", "Keys look like"},
		{"unknown key", "Bearer " + mustGenerate(t, apikey.Test), "api_key_invalid", "Invalid API key provided: sk_test_..."},
		{"revoked key", "Bearer " + f.revokedKey, "api_key_invalid", "Invalid API key provided: sk_test_..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := call(t, f.handler, http.MethodGet, "/v1/account", tc.authorization)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body)
			}
			if got := rec.Header().Get("WWW-Authenticate"); !strings.HasPrefix(got, "Bearer") {
				t.Errorf("WWW-Authenticate = %q", got)
			}
			e := decodeError(t, rec)
			if e.Type != gen.AuthenticationError || e.Code != tc.wantCode || !strings.Contains(e.Message, tc.wantMessage) {
				t.Fatalf("error = %+v, want %s containing %q", e, tc.wantCode, tc.wantMessage)
			}
			// The secret part of a key is never echoed back.
			if key := strings.TrimPrefix(tc.authorization, "Bearer "); len(key) > 20 && strings.Contains(rec.Body.String(), key[8:20]) {
				t.Fatalf("response echoes the key: %s", rec.Body)
			}
		})
	}
}

func TestGetAccount(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, tc := range []struct {
		name     string
		key      string
		livemode bool
	}{{"test key", f.testKey, false}, {"live key", f.liveKey, true}} {
		t.Run(tc.name, func(t *testing.T) {
			rec := call(t, f.handler, http.MethodGet, "/v1/account", "Bearer "+tc.key)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
			}
			var a gen.Account
			if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
				t.Fatal(err)
			}
			if a.Id != f.account.ID.String() || a.Object != gen.AccountObjectAccount || a.Name != "Acme Learning" ||
				a.Livemode != tc.livemode || a.Metadata["region"] != "south" || !a.CreatedAt.Equal(f.account.CreatedAt) {
				t.Fatalf("account = %+v", a)
			}
		})
	}
}

func TestKeysAreScopedToTheirAccount(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	rec := call(t, f.handler, http.MethodGet, "/v1/account", "Bearer "+f.otherAcctKey)
	var a gen.Account
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil || a.Id != f.otherAccountID.String() {
		t.Fatalf("other account's key returned %+v, %v", a, err)
	}
}

func TestBearerSchemeIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if rec := call(t, f.handler, http.MethodGet, "/v1/account", "bearer "+f.testKey); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
}

func TestUnknownRouteAndMethod(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	rec := call(t, f.handler, http.MethodGet, "/v1/nope", "Bearer "+f.testKey)
	if e := decodeError(t, rec); rec.Code != http.StatusNotFound || e.Type != gen.NotFoundError || e.Code != "route_not_found" {
		t.Fatalf("GET /v1/nope = %d %+v", rec.Code, e)
	}
	rec = call(t, f.handler, http.MethodPost, "/healthz", "")
	if e := decodeError(t, rec); rec.Code != http.StatusMethodNotAllowed || e.Code != "method_not_allowed" {
		t.Fatalf("POST /healthz = %d %+v", rec.Code, e)
	}
}

// failingQueries simulates the database being down.
type failingQueries struct{}

var errDatabaseDown = errors.New("connection refused")

func (failingQueries) AuthenticateAPIKey(context.Context, []byte) (platformdb.AuthenticateAPIKeyRow, error) {
	return platformdb.AuthenticateAPIKeyRow{}, errDatabaseDown
}

func (failingQueries) GetAccount(context.Context, id.AccountID) (platformdb.Account, error) {
	return platformdb.Account{}, errDatabaseDown
}

func TestDatabaseFailureIs500NotAuthError(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	h, err := api.New(api.Config{Queries: failingQueries{}, Logger: slog.New(slog.NewJSONHandler(&logs, nil))})
	if err != nil {
		t.Fatal(err)
	}
	rec := call(t, h, http.MethodGet, "/v1/account", "Bearer "+mustGenerate(t, apikey.Test))
	e := decodeError(t, rec)
	if rec.Code != http.StatusInternalServerError || e.Type != gen.ApiError || e.Code != "internal_error" {
		t.Fatalf("got %d %+v, want 500 api_error", rec.Code, e)
	}
	if strings.Contains(rec.Body.String(), "connection refused") {
		t.Fatal("internal error details leaked to the client")
	}
	if !strings.Contains(logs.String(), "connection refused") || !strings.Contains(logs.String(), e.RequestId) {
		t.Fatalf("the failure and request ID should be logged: %s", logs.String())
	}
}

func TestNewRequiresQueries(t *testing.T) {
	t.Parallel()
	if _, err := api.New(api.Config{}); err == nil {
		t.Fatal("want an error without Queries")
	}
}

func TestServeShutsDownCleanly(t *testing.T) {
	t.Parallel()

	h, err := api.New(api.Config{Version: "dev", Queries: failingQueries{}})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() { done <- api.Serve(ctx, ln, h, slog.New(slog.DiscardHandler)) }()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+ln.Addr().String()+"/healthz", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v, want nil after shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after the context was cancelled")
	}
}

func mustGenerate(t *testing.T, mode apikey.Mode) string {
	t.Helper()
	s, err := apikey.Generate(mode)
	if err != nil {
		t.Fatal(err)
	}
	return s.Text
}
