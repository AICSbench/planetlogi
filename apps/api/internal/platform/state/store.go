package state

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists one JSON document per student and module so Phase 2 progress
// survives process restarts.
type Store interface {
	Load(ctx context.Context, studentID, module string, dest any) (bool, error)
	Save(ctx context.Context, studentID, module string, value any) error
}

// MergeStore serializes a read-modify-write of one student's document under a
// row lock, so concurrent saves from multiple instances cannot drop records.
type MergeStore interface {
	SaveMerged(ctx context.Context, studentID, module string, merge func(current []byte) (any, error)) error
}

type PGStore struct {
	pool *pgxpool.Pool
}

func NewPGStore(pool *pgxpool.Pool) *PGStore {
	return &PGStore{pool: pool}
}

func (s *PGStore) Load(ctx context.Context, studentID, module string, dest any) (bool, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT payload FROM phase2_state
		WHERE student_id = $1::uuid AND module = $2
	`, studentID, module).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(raw, dest)
}

func (s *PGStore) Save(ctx context.Context, studentID, module string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO phase2_state (student_id, module, payload, updated_at)
		VALUES ($1::uuid, $2, $3::jsonb, NOW())
		ON CONFLICT (student_id, module) DO UPDATE SET
			payload = EXCLUDED.payload,
			updated_at = NOW()
	`, studentID, module, raw)
	return err
}

// SaveMerged locks the student's module row, lets the caller merge its
// in-memory snapshot with the current durable document, then writes the
// merged result in the same transaction.
func (s *PGStore) SaveMerged(ctx context.Context, studentID, module string, merge func(current []byte) (any, error)) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.SaveMergedTx(ctx, tx, studentID, module, merge); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SaveMergedTx performs the same locked read-merge-write inside the caller's
// transaction so Phase 3 state can commit atomically with ledger mutations
// (wallet debits) applied through ApplyBatchTx on the same tx. Merge errors
// (for example a failed quota check) abort the whole transaction.
func (s *PGStore) SaveMergedTx(ctx context.Context, tx pgx.Tx, studentID, module string, merge func(current []byte) (any, error)) error {
	// Ensure the row exists before locking it, so two first writers serialize
	// instead of both merging an absent document.
	if _, err := tx.Exec(ctx, `
		INSERT INTO phase2_state (student_id, module, payload)
		VALUES ($1::uuid, $2, '{}'::jsonb)
		ON CONFLICT (student_id, module) DO NOTHING
	`, studentID, module); err != nil {
		return err
	}
	var current []byte
	if err := tx.QueryRow(ctx, `
		SELECT payload FROM phase2_state
		WHERE student_id = $1::uuid AND module = $2
		FOR UPDATE
	`, studentID, module).Scan(&current); err != nil {
		return err
	}
	merged, err := merge(current)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE phase2_state SET payload = $3::jsonb, updated_at = NOW()
		WHERE student_id = $1::uuid AND module = $2
	`, studentID, module, raw); err != nil {
		return err
	}
	return nil
}
