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
