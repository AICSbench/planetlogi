package economy

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Currency string

const (
	CurrencySP     Currency = "SP"
	CurrencyAC     Currency = "AC"
	CurrencyXP     Currency = "XP"
	CurrencyScroll Currency = "SCROLL"
	CurrencySuperS Currency = "S_PTS"
)

var (
	ErrInsufficientFunds  = errors.New("insufficient astro coins balance")
	ErrInvalidDelta       = errors.New("transaction delta cannot be zero")
	ErrMissingIdempotency = errors.New("idempotency key is required")
)

type Wallet struct {
	StudentID        string `json:"studentId"`
	TotalSP          int64  `json:"totalSp"`
	AstroCoins       int64  `json:"astroCoins"`
	PilotXP          int64  `json:"pilotXp"`
	PilotLevel       int    `json:"pilotLevel"`
	TotalStarScrolls int    `json:"totalStarScrolls"`
}

type MutationRequest struct {
	StudentID      string
	SubjectID      *string
	Currency       Currency
	Delta          int64
	SourceType     string
	SourceID       string
	IdempotencyKey string
}

type MutationResult struct {
	Wallet       Wallet
	BalanceAfter int64
	WasDuplicate bool
}

type Ledger interface {
	Apply(ctx context.Context, req MutationRequest) (MutationResult, error)
}

// PGLedger implements an atomic PostgreSQL SELECT ... FOR UPDATE double-entry ledger.
type PGLedger struct {
	pool *pgxpool.Pool
}

func NewPGLedger(pool *pgxpool.Pool) *PGLedger {
	return &PGLedger{pool: pool}
}

func (l *PGLedger) Apply(ctx context.Context, req MutationRequest) (MutationResult, error) {
	if req.IdempotencyKey == "" {
		return MutationResult{}, ErrMissingIdempotency
	}
	if req.Delta == 0 {
		return MutationResult{}, ErrInvalidDelta
	}

	tx, err := l.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return MutationResult{}, fmt.Errorf("begin ledger tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Ensure default wallet exists, then lock student wallet row atomically
	if _, err := tx.Exec(ctx, `
		INSERT INTO student_wallets (student_id)
		VALUES ($1::uuid)
		ON CONFLICT (student_id) DO NOTHING
	`, req.StudentID); err != nil {
		return MutationResult{}, fmt.Errorf("ensure wallet: %w", err)
	}

	var w Wallet
	err = tx.QueryRow(ctx, `
		SELECT student_id::text, total_sp, astro_coins, pilot_xp, pilot_level, total_star_scrolls
		FROM student_wallets
		WHERE student_id::text = $1
		FOR UPDATE
	`, req.StudentID).Scan(&w.StudentID, &w.TotalSP, &w.AstroCoins, &w.PilotXP, &w.PilotLevel, &w.TotalStarScrolls)
	if err != nil {
		return MutationResult{}, fmt.Errorf("lock wallet: %w", err)
	}

	// 2. Check idempotency key inside locked transaction
	var existingBalance int64
	err = tx.QueryRow(ctx, `
		SELECT balance_after FROM currency_transactions WHERE idempotency_key = $1
	`, req.IdempotencyKey).Scan(&existingBalance)
	if err == nil {
		return MutationResult{Wallet: w, BalanceAfter: existingBalance, WasDuplicate: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return MutationResult{}, fmt.Errorf("check idempotency: %w", err)
	}

	// 3. Apply mutation & enforce non-negative invariants
	balanceAfter, err := applyToWallet(&w, req.Currency, req.Delta)
	if err != nil {
		return MutationResult{}, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE student_wallets
		SET total_sp = $2, astro_coins = $3, pilot_xp = $4, pilot_level = $5,
		    total_star_scrolls = $6, updated_at = NOW()
		WHERE student_id::text = $1
	`, w.StudentID, w.TotalSP, w.AstroCoins, w.PilotXP, w.PilotLevel, w.TotalStarScrolls); err != nil {
		return MutationResult{}, fmt.Errorf("update wallet: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO currency_transactions
			(student_id, subject_id, currency, delta, balance_after, source_type, source_id, idempotency_key)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)
	`, req.StudentID, req.SubjectID, string(req.Currency), req.Delta, balanceAfter, req.SourceType, req.SourceID, req.IdempotencyKey); err != nil {
		return MutationResult{}, fmt.Errorf("insert ledger tx: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return MutationResult{}, fmt.Errorf("commit ledger tx: %w", err)
	}

	return MutationResult{Wallet: w, BalanceAfter: balanceAfter, WasDuplicate: false}, nil
}

func applyToWallet(w *Wallet, currency Currency, delta int64) (int64, error) {
	switch currency {
	case CurrencySP:
		next := w.TotalSP + delta
		if next < 0 {
			return 0, ErrInsufficientFunds
		}
		w.TotalSP = next
		return next, nil
	case CurrencyAC:
		next := w.AstroCoins + delta
		if next < 0 {
			return 0, ErrInsufficientFunds
		}
		w.AstroCoins = next
		return next, nil
	case CurrencyXP:
		next := w.PilotXP + delta
		if next < 0 {
			return 0, ErrInsufficientFunds
		}
		w.PilotXP = next
		w.PilotLevel = ComputeLevelProgress(next).Level
		return next, nil
	case CurrencyScroll:
		next := int64(w.TotalStarScrolls) + delta
		if next < 0 {
			return 0, ErrInsufficientFunds
		}
		w.TotalStarScrolls = int(next)
		return next, nil
	case CurrencySuperS:
		if delta < 0 {
			return 0, ErrInsufficientFunds
		}
		return delta, nil
	default:
		return 0, fmt.Errorf("unsupported currency: %s", currency)
	}
}

// MemoryLedger provides a concurrency-safe ledger for unit & race tests.
type MemoryLedger struct {
	mu      sync.Mutex
	wallets map[string]Wallet
	seen    map[string]int64
}

func NewMemoryLedger(initial ...Wallet) *MemoryLedger {
	m := &MemoryLedger{
		wallets: make(map[string]Wallet),
		seen:    make(map[string]int64),
	}
	for _, w := range initial {
		m.wallets[w.StudentID] = w
	}
	return m
}

func (m *MemoryLedger) Apply(_ context.Context, req MutationRequest) (MutationResult, error) {
	if req.IdempotencyKey == "" {
		return MutationResult{}, ErrMissingIdempotency
	}
	if req.Delta == 0 {
		return MutationResult{}, ErrInvalidDelta
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	w := m.wallets[req.StudentID]
	if w.StudentID == "" {
		w = Wallet{StudentID: req.StudentID, PilotLevel: MinPilotLevel}
	}
	if bal, exists := m.seen[req.IdempotencyKey]; exists {
		return MutationResult{Wallet: w, BalanceAfter: bal, WasDuplicate: true}, nil
	}
	bal, err := applyToWallet(&w, req.Currency, req.Delta)
	if err != nil {
		return MutationResult{}, err
	}
	m.wallets[req.StudentID] = w
	m.seen[req.IdempotencyKey] = bal
	return MutationResult{Wallet: w, BalanceAfter: bal, WasDuplicate: false}, nil
}
