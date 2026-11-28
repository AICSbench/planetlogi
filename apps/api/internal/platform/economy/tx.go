package economy

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// TxLedger is implemented by durable ledgers that can join the caller's
// transaction so wallet mutations commit (or roll back) together with the
// caller's own state writes — required for entitlements such as Planet Stories
// unlocks and badge recovery that must never exist without their debit.
type TxLedger interface {
	ApplyBatchTx(ctx context.Context, tx pgx.Tx, reqs []MutationRequest) ([]MutationResult, error)
}

// ApplyBatchInTx applies ledger mutations inside the caller's transaction when
// the ledger supports it. Memory test ledgers fall back to a separate
// (non-transactional) apply, matching their in-memory semantics.
func ApplyBatchInTx(ctx context.Context, tx pgx.Tx, ledger Ledger, reqs []MutationRequest) ([]MutationResult, error) {
	if tl, ok := ledger.(TxLedger); ok && tx != nil {
		return tl.ApplyBatchTx(ctx, tx, reqs)
	}
	return ledger.ApplyBatch(ctx, reqs)
}
