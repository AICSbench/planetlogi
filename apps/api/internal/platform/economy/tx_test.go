package economy

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// The caller owns the transaction; the adapter only forwards it to the ledger.
type callerTx struct{ pgx.Tx }

type rejectingTxLedger struct {
	Ledger
	gotTx   pgx.Tx
	gotReqs []MutationRequest
	err     error
}

func (l *rejectingTxLedger) ApplyBatchTx(_ context.Context, tx pgx.Tx, reqs []MutationRequest) ([]MutationResult, error) {
	l.gotTx, l.gotReqs = tx, reqs
	return nil, l.err
}

func TestApplyBatchInTxUsesCallerTransactionAndPropagatesFailure(t *testing.T) {
	tx := &callerTx{}
	req := MutationRequest{StudentID: "pilot", Currency: CurrencyAC, Delta: -20}
	wantErr := errors.New("transaction debit failed")
	// The embedded ordinary ledger is nil: applying outside the caller's
	// transaction would be an invalid fallback after this failure.
	ledger := &rejectingTxLedger{err: wantErr}
	if _, err := ApplyBatchInTx(t.Context(), tx, ledger, []MutationRequest{req}); !errors.Is(err, wantErr) {
		t.Fatalf("debit error = %v; want %v", err, wantErr)
	}
	if ledger.gotTx != tx || len(ledger.gotReqs) != 1 || ledger.gotReqs[0] != req {
		t.Fatalf("transactional ledger received tx=%v requests=%+v", ledger.gotTx, ledger.gotReqs)
	}
}

func TestApplyBatchInTxSupportsMemoryLedger(t *testing.T) {
	ledger := NewMemoryLedger(Wallet{StudentID: "pilot", AstroCoins: 20})
	results, err := ApplyBatchInTx(t.Context(), &callerTx{}, ledger, []MutationRequest{{
		StudentID: "pilot", Currency: CurrencyAC, Delta: -20,
		SourceType: "STORY_UNLOCK", SourceID: "chapter", IdempotencyKey: "pilot-chapter",
	}})
	if err != nil || len(results) != 1 || results[0].Wallet.AstroCoins != 0 {
		t.Fatalf("memory ledger results = %+v, error = %v; want one debit of 20 AC", results, err)
	}
}
