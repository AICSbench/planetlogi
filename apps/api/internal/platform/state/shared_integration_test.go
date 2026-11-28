package state_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/AICSbench/planetlogi/apps/api/internal/platform/state"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSharedStateConcurrentFirstWritesAndRollback(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; shared state requires migrated PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	store := state.NewPGStore(pool)
	scope, key := "shared-state-test-"+uuid.NewString(), "crew"
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := store.DeleteShared(cleanup, scope, key); err != nil {
			t.Error(err)
		}
	}()

	// Exercise two writers racing to create the same previously absent row.
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, member := range []string{"pilot-a", "pilot-b"} {
		go func() {
			<-start
			errs <- state.NewPGStore(pool).SaveSharedMerged(ctx, scope, key, func(raw []byte) (any, error) {
				members := map[string]bool{}
				if err := json.Unmarshal(raw, &members); err != nil {
					return nil, err
				}
				members[member] = true
				return members, nil
			})
		}()
	}
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
	if t.Failed() {
		return
	}

	rejected := errors.New("roster update rejected")
	if err := store.SaveSharedMerged(ctx, scope, key, func([]byte) (any, error) {
		return map[string]bool{"unwanted": true}, rejected
	}); !errors.Is(err, rejected) {
		t.Fatalf("merge returned %v, want %v", err, rejected)
	}
	var members map[string]bool
	if found, err := store.LoadShared(ctx, scope, key, &members); err != nil || !found ||
		len(members) != 2 || !members["pilot-a"] || !members["pilot-b"] {
		t.Fatalf("original roster after rejected update: found=%v members=%v err=%v", found, members, err)
	}
	if err := store.DeleteShared(ctx, scope, key); err != nil {
		t.Fatal(err)
	}
	if found, err := store.LoadShared(ctx, scope, key, &members); err != nil || found {
		t.Fatalf("deleted roster: found=%v err=%v", found, err)
	}
}
