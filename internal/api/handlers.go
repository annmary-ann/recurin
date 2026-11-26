package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/annmary-ann/recurin/internal/api/gen"
)

// handlers implements the strict server interface generated from the spec.
type handlers struct {
	version string
	queries Queries
}

var _ gen.StrictServerInterface = (*handlers)(nil)

// GetHealth reports that the process is up. It deliberately does not check
// dependencies: a liveness probe that fails when Postgres blips would make an
// orchestrator restart healthy API pods. Readiness gets its own endpoint.
func (h *handlers) GetHealth(context.Context, gen.GetHealthRequestObject) (gen.GetHealthResponseObject, error) {
	return gen.GetHealth200JSONResponse{Status: gen.Ok, Version: h.version}, nil
}

// GetAccount returns the account that owns the calling API key.
func (h *handlers) GetAccount(ctx context.Context, _ gen.GetAccountRequestObject) (gen.GetAccountResponseObject, error) {
	p, ok := PrincipalFrom(ctx)
	if !ok {
		return nil, errors.New("GetAccount ran without an authenticated principal")
	}
	a, err := h.queries.GetAccount(ctx, p.AccountID)
	if err != nil {
		return nil, fmt.Errorf("load account %s: %w", p.AccountID, err)
	}
	meta := gen.Metadata{}
	if err := json.Unmarshal(a.Metadata, &meta); err != nil {
		return nil, fmt.Errorf("account %s metadata: %w", a.ID, err)
	}
	return gen.GetAccount200JSONResponse{Body: gen.Account{
		Id:        a.ID.String(),
		Object:    gen.AccountObjectAccount,
		Name:      a.Name,
		Livemode:  p.Livemode(),
		CreatedAt: a.CreatedAt.UTC(),
		Metadata:  meta,
	}}, nil
}
