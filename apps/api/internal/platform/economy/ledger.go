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
	ApplyBatch(ctx context.Context, reqs []MutationRequest) ([]MutationResult, error)
}

// PGLedger implements an atomic PostgreSQL SELECT ... FOR UPDATE double-entry ledger.
type PGLedger struct {
	pool *pgxpool.Pool
}

func NewPGLedger(pool *pgxpool.Pool) *PGLedger {
	return &PGLedger{pool: pool}
}

func (l *PGLedger) Apply(ctx context.Context, req MutationRequest) (MutationResult, error) {
	results, err := l.ApplyBatch(ctx, []MutationRequest{req})
	if err != nil {
		return MutationResult{}, err
	}
	return results[0], nil
}

func (l *PGLedger) ApplyBatch(ctx context.Context, reqs []MutationRequest) ([]MutationResult, error) {
	if len(reqs) == 0 {
		return nil, nil
	}
	for _, req := range reqs {
		if req.IdempotencyKey == "" {
			return nil, ErrMissingIdempotency
		}
		if req.Delta == 0 {
			return nil, ErrInvalidDelta
		}
	}

	tx, err := l.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin ledger tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	studentID := reqs[0].StudentID
	if _, err := tx.Exec(ctx, `
		INSERT INTO student_wallets (student_id)
		VALUES ($1::uuid)
		ON CONFLICT (student_id) DO NOTHING
	`, studentID); err != nil {
		return nil, fmt.Errorf("ensure wallet: %w", err)
	}

	var w Wallet
	err = tx.QueryRow(ctx, `
		SELECT student_id::text, total_sp, astro_coins, pilot_xp, pilot_level, total_star_scrolls
		FROM student_wallets
		WHERE student_id::text = $1
		FOR UPDATE
	`, studentID).Scan(&w.StudentID, &w.TotalSP, &w.AstroCoins, &w.PilotXP, &w.PilotLevel, &w.TotalStarScrolls)
	if err != nil {
		return nil, fmt.Errorf("lock wallet: %w", err)
	}

	results := make([]MutationResult, len(reqs))
	pending := make([]int, 0, len(reqs))
	stagedSeen := make(map[string]int64, len(reqs))

	for i, req := range reqs {
		if req.StudentID != studentID {
			return nil, fmt.Errorf("batch ledger mutations must target a single student")
		}
		if bal, exists := stagedSeen[req.IdempotencyKey]; exists {
			results[i] = MutationResult{BalanceAfter: bal, WasDuplicate: true}
			continue
		}
		var existingBalance int64
		err = tx.QueryRow(ctx, `
			SELECT balance_after FROM currency_transactions WHERE idempotency_key = $1
		`, req.IdempotencyKey).Scan(&existingBalance)
		if err == nil {
			stagedSeen[req.IdempotencyKey] = existingBalance
			results[i] = MutationResult{BalanceAfter: existingBalance, WasDuplicate: true}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check idempotency: %w", err)
		}

		balanceAfter, err := applyToWallet(&w, req.Currency, req.Delta)
		if err != nil {
			return nil, err
		}
		stagedSeen[req.IdempotencyKey] = balanceAfter
		results[i] = MutationResult{BalanceAfter: balanceAfter, WasDuplicate: false}
		pending = append(pending, i)
	}

	if len(pending) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE student_wallets
			SET total_sp = $2, astro_coins = $3, pilot_xp = $4, pilot_level = $5,
			    total_star_scrolls = $6, updated_at = NOW()
			WHERE student_id::text = $1
		`, w.StudentID, w.TotalSP, w.AstroCoins, w.PilotXP, w.PilotLevel, w.TotalStarScrolls); err != nil {
			return nil, fmt.Errorf("update wallet: %w", err)
		}

		for _, i := range pending {
			req := reqs[i]
			if _, err := tx.Exec(ctx, `
				INSERT INTO currency_transactions
					(student_id, subject_id, currency, delta, balance_after, source_type, source_id, idempotency_key)
				VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)
			`, req.StudentID, req.SubjectID, string(req.Currency), req.Delta, results[i].BalanceAfter, req.SourceType, req.SourceID, req.IdempotencyKey); err != nil {
				return nil, fmt.Errorf("insert ledger tx: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit ledger tx: %w", err)
	}

	for i := range results {
		results[i].Wallet = w
	}

	return results, nil
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

func (m *MemoryLedger) Apply(ctx context.Context, req MutationRequest) (MutationResult, error) {
	results, err := m.ApplyBatch(ctx, []MutationRequest{req})
	if err != nil {
		return MutationResult{}, err
	}
	return results[0], nil
}

func (m *MemoryLedger) ApplyBatch(_ context.Context, reqs []MutationRequest) ([]MutationResult, error) {
	if len(reqs) == 0 {
		return nil, nil
	}
	for _, req := range reqs {
		if req.IdempotencyKey == "" {
			return nil, ErrMissingIdempotency
		}
		if req.Delta == 0 {
			return nil, ErrInvalidDelta
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	studentID := reqs[0].StudentID
	w := m.wallets[studentID]
	if w.StudentID == "" {
		w = Wallet{StudentID: studentID, PilotLevel: MinPilotLevel}
	}

	results := make([]MutationResult, len(reqs))
	stagedSeen := make(map[string]int64, len(reqs))

	for i, req := range reqs {
		if req.StudentID != studentID {
			return nil, fmt.Errorf("batch ledger mutations must target a single student")
		}
		if bal, exists := m.seen[req.IdempotencyKey]; exists {
			results[i] = MutationResult{Wallet: w, BalanceAfter: bal, WasDuplicate: true}
			continue
		}
		if bal, exists := stagedSeen[req.IdempotencyKey]; exists {
			results[i] = MutationResult{Wallet: w, BalanceAfter: bal, WasDuplicate: true}
			continue
		}
		bal, err := applyToWallet(&w, req.Currency, req.Delta)
		if err != nil {
			return nil, err
		}
		stagedSeen[req.IdempotencyKey] = bal
		results[i] = MutationResult{BalanceAfter: bal, WasDuplicate: false}
	}

	m.wallets[studentID] = w
	for k, v := range stagedSeen {
		m.seen[k] = v
	}
	for i := range results {
		results[i].Wallet = w
	}
	return results, nil
}
