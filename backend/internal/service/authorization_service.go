package service

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/repository"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
)

/*
AuthorizationService answers "may this user do X".

Separate from PermissionService, which owns the permission catalogue: one
answers what permissions exist, the other what a person holds. Splitting them
keeps the catalogue's small interface out of the request path and lets this one
carry a cache without the seeder inheriting it.

Permissions are not in the access token. They could be — the claims already
carry roles — but a 12-hour token would then pin a permission set for a whole
shift, and revoking someone's payroll access would take until tomorrow. Reading
them per request against a short-lived cache keeps revocation bounded by
AUTH_PERMISSION_CACHE_TTL instead.
*/
type AuthorizationService interface {
	// EffectiveFor returns every permission name the user's roles grant.
	EffectiveFor(ctx context.Context, userID uuid.UUID) ([]string, error)
	// Has reports whether the user holds one specific permission.
	Has(ctx context.Context, userID uuid.UUID, permission string) (bool, error)
	// Invalidate drops a user's cached set, for the moment their roles change.
	Invalidate(userID uuid.UUID)
}

type authorizationService struct {
	users repository.UserRepository
	roles repository.RoleRepository

	ttl   time.Duration
	mu    sync.RWMutex
	cache map[uuid.UUID]permissionCacheEntry
}

type permissionCacheEntry struct {
	permissions map[string]struct{}
	expiresAt   time.Time
}

func NewAuthorizationService(
	users repository.UserRepository, roles repository.RoleRepository, ttl time.Duration,
) AuthorizationService {
	return &authorizationService{
		users: users,
		roles: roles,
		ttl:   ttl,
		cache: make(map[uuid.UUID]permissionCacheEntry),
	}
}

func (s *authorizationService) EffectiveFor(ctx context.Context, userID uuid.UUID) ([]string, error) {
	set, err := s.resolve(ctx, userID)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	return names, nil
}

func (s *authorizationService) Has(ctx context.Context, userID uuid.UUID, permission string) (bool, error) {
	set, err := s.resolve(ctx, userID)
	if err != nil {
		return false, err
	}
	_, ok := set[permission]
	return ok, nil
}

func (s *authorizationService) Invalidate(userID uuid.UUID) {
	s.mu.Lock()
	delete(s.cache, userID)
	s.mu.Unlock()
}

/*
resolve returns the user's permission set, from cache when it is still fresh.

The cache is per-process, like the in-memory token denylist a Redis-less
deployment falls back to. Behind several instances each keeps its own copy, so
the TTL — not an invalidation message — is what bounds how long a revoked
permission can still be honoured. That is exactly what AUTH_PERMISSION_CACHE_TTL
documents itself as doing.

A miss is resolved without holding the lock, so a slow database cannot block
every other request's permission check behind one query. Two concurrent misses
for the same user may both query; they compute the same answer, and the second
write simply replaces the first.
*/
func (s *authorizationService) resolve(ctx context.Context, userID uuid.UUID) (map[string]struct{}, error) {
	if set, ok := s.lookup(userID); ok {
		return set, nil
	}

	roleIDs, err := s.users.ListRoleIDs(ctx, userID)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to load user roles", err)
	}

	names, err := s.roles.ListPermissionNamesForRoles(ctx, roleIDs)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to load permissions", err)
	}

	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}

	s.mu.Lock()
	s.cache[userID] = permissionCacheEntry{permissions: set, expiresAt: time.Now().Add(s.ttl)}
	s.mu.Unlock()

	return set, nil
}

func (s *authorizationService) lookup(userID uuid.UUID) (map[string]struct{}, bool) {
	s.mu.RLock()
	entry, ok := s.cache[userID]
	s.mu.RUnlock()

	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.permissions, true
}
