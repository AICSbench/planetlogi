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

// LoadShared reads one JSON document from the shared (non-student-scoped)
// state table, so cross-student records such as Star Crew rosters and Galaxy
// Showcase entries survive restarts and are visible to every API instance.
func (s *PGStore) LoadShared(ctx context.Context, scope, key string, dest any) (bool, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT payload FROM shared_state
		WHERE scope = $1 AND key = $2
	`, scope, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(raw, dest)
}

// SaveShared upserts one shared JSON document.
func (s *PGStore) SaveShared(ctx context.Context, scope, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO shared_state (scope, key, payload, updated_at)
		VALUES ($1, $2, $3::jsonb, NOW())
		ON CONFLICT (scope, key) DO UPDATE SET
			payload = EXCLUDED.payload,
			updated_at = NOW()
	`, scope, key, raw)
	return err
}

// SaveSharedMerged serializes a read-modify-write of one shared document under
// a row lock, so concurrent joins or leaves from multiple instances cannot
// drop each other's changes.
func (s *PGStore) SaveSharedMerged(ctx context.Context, scope, key string, merge func(current []byte) (any, error)) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Ensure the row exists before locking it, so two first writers serialize
	// instead of both merging an absent document.
	if _, err := tx.Exec(ctx, `
		INSERT INTO shared_state (scope, key, payload)
		VALUES ($1, $2, '{}'::jsonb)
		ON CONFLICT (scope, key) DO NOTHING
	`, scope, key); err != nil {
		return err
	}
	var current []byte
	if err := tx.QueryRow(ctx, `
		SELECT payload FROM shared_state
		WHERE scope = $1 AND key = $2
		FOR UPDATE
	`, scope, key).Scan(&current); err != nil {
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
		UPDATE shared_state SET payload = $3::jsonb, updated_at = NOW()
		WHERE scope = $1 AND key = $2
	`, scope, key, raw); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteShared removes one shared document (for example a Star Crew
// membership row after the member leaves).
func (s *PGStore) DeleteShared(ctx context.Context, scope, key string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM shared_state WHERE scope = $1 AND key = $2`, scope, key)
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
