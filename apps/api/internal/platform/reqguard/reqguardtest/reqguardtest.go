// Package reqguardtest provides an in-memory auth service with real signed tokens for
// handler-level tests of the Phase 2 modules.
package reqguardtest

import (
	"context"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/AICSbench/planetlogi/apps/api/internal/config"
	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
)

const password = "test-password"

type memStore struct {
	byIdentifier map[string]*auth.CredentialRecord
}

func (m *memStore) FindByIdentifier(_ context.Context, identifier string) (*auth.CredentialRecord, error) {
	if rec, ok := m.byIdentifier[strings.ToLower(strings.TrimSpace(identifier))]; ok {
		return rec, nil
	}
	return nil, auth.ErrNotFound
}

func (m *memStore) FindByIDAndRole(_ context.Context, id string, role auth.Role) (*auth.Principal, error) {
	for _, rec := range m.byIdentifier {
		if rec.Principal.ID == id && rec.Principal.Role == role {
			p := rec.Principal
			return &p, nil
		}
	}
	return nil, auth.ErrNotFound
}

func (m *memStore) RecentStarPointsFeed(context.Context, int) ([]auth.LiveFeedItem, error) {
	return nil, nil
}

// Auth is an auth service seeded with the given principals.
type Auth struct {
	Service *auth.Service
}

// New builds an auth service whose principals can log in with their ID as identifier.
func New(t *testing.T, principals ...auth.Principal) *Auth {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	store := &memStore{byIdentifier: map[string]*auth.CredentialRecord{}}
	for _, p := range principals {
		store.byIdentifier[strings.ToLower(p.ID)] = &auth.CredentialRecord{Principal: p, PasswordHash: string(hash)}
	}
	cfg := config.Load()
	cfg.AppEnv = "development"
	return &Auth{Service: auth.NewService(cfg, store, nil, nil)}
}

// BearerHeader logs the principal in and returns an "Authorization: Bearer ..." header line.
func (a *Auth) BearerHeader(t *testing.T, principalID string) string {
	t.Helper()
	_, pair, err := a.Service.Login(context.Background(), principalID, password)
	if err != nil {
		t.Fatalf("login %s: %v", principalID, err)
	}
	return "Authorization: Bearer " + pair.AccessToken
}
