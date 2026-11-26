package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/jackc/pgx/v5"

	"github.com/annmary-ann/recurin/internal/api/gen"
	"github.com/annmary-ann/recurin/internal/platform/apikey"
	"github.com/annmary-ann/recurin/internal/platform/id"
)

// Principal is who is calling: the account and mode of the API key used.
type Principal struct {
	AccountID id.AccountID
	KeyID     id.APIKeyID
	Mode      apikey.Mode
}

// Livemode reports whether the request acts on live data.
func (p Principal) Livemode() bool { return p.Mode == apikey.Live }

// The validator authenticates inside its own call, and cannot hand a new
// context to the handler. So withAuthSlot puts an empty slot in the request
// context first, and the authenticator fills it in.
type authSlot struct{ principal *Principal }

type authSlotKey struct{}

func withAuthSlot(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authSlotKey{}, &authSlot{})))
	})
}

// PrincipalFrom returns the authenticated caller. It is false on operations
// the spec marks as public (security: []).
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	slot, _ := ctx.Value(authSlotKey{}).(*authSlot)
	if slot == nil || slot.principal == nil {
		return Principal{}, false
	}
	return *slot.principal, true
}

type authenticator struct {
	queries Queries
}

// authenticate runs for every operation whose spec requires secretKey.
func (a authenticator) authenticate(ctx context.Context, in *openapi3filter.AuthenticationInput) error {
	r := in.RequestValidationInput.Request
	text, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		return errMissingKey()
	}
	mode, hash, err := apikey.Parse(text)
	if err != nil {
		return errInvalidKey("Invalid API key provided. Keys look like sk_test_... or sk_live_....")
	}
	row, err := a.queries.AuthenticateAPIKey(ctx, hash[:])
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Unknown and revoked keys get the same answer.
		return errInvalidKey("Invalid API key provided: " + apikey.HintOf(text) + ".")
	case err != nil:
		return errInternal(err)
	case apikey.Mode(row.Mode) != mode:
		return errInternal(errors.New("api key mode in the database does not match its prefix"))
	}
	slot, _ := r.Context().Value(authSlotKey{}).(*authSlot)
	if slot == nil {
		return errInternal(errors.New("authentication ran without an auth slot"))
	}
	slot.principal = &Principal{AccountID: row.AccountID, KeyID: row.ID, Mode: mode}
	return nil
}

// bearerToken extracts the token from "Bearer <token>" (scheme case-insensitive).
func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}

func errMissingKey() *apiError {
	return &apiError{
		status: http.StatusUnauthorized, typ: gen.AuthenticationError, code: "api_key_missing",
		message: `No API key provided. Send it as "Authorization: Bearer sk_test_...".`,
	}
}

func errInvalidKey(message string) *apiError {
	return &apiError{status: http.StatusUnauthorized, typ: gen.AuthenticationError, code: "api_key_invalid", message: message}
}

func openapiOptions(auth openapi3filter.AuthenticationFunc) openapi3filter.Options {
	return openapi3filter.Options{AuthenticationFunc: auth, MultiError: false}
}
