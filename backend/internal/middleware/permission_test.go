package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/ctxkey"
)

/*
stubAuthorization is a hand-written AuthorizationService. The interface is three
methods and the behaviour under test is the gate's, not the resolver's.
*/
type stubAuthorization struct {
	allowed map[string]bool
	err     error
	calls   int
}

func (s *stubAuthorization) EffectiveFor(context.Context, uuid.UUID) ([]string, error) {
	names := make([]string, 0, len(s.allowed))
	for name := range s.allowed {
		names = append(names, name)
	}
	return names, s.err
}

func (s *stubAuthorization) Has(_ context.Context, _ uuid.UUID, permission string) (bool, error) {
	s.calls++
	if s.err != nil {
		return false, s.err
	}
	return s.allowed[permission], nil
}

func (s *stubAuthorization) Invalidate(uuid.UUID) {}

// newPermissionApp mounts the gate behind a stand-in for Auth, so the test can
// control whether a user id is on the context.
func newPermissionApp(authz *stubAuthorization, permission string, userID uuid.UUID) *fiber.App {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(logger)})

	app.Use(func(c *fiber.Ctx) error {
		if userID != uuid.Nil {
			c.Locals(ctxkey.UserID, userID)
			c.SetUserContext(context.WithValue(c.UserContext(), ctxkey.UserID, userID))
		}
		return c.Next()
	})
	app.Get("/guarded", RequirePermission(authz, permission), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	return app
}

func TestRequirePermission(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name       string
		authz      *stubAuthorization
		userID     uuid.UUID
		wantStatus int
	}{
		{
			name:       "allows a user holding the permission",
			authz:      &stubAuthorization{allowed: map[string]bool{"hrm.manage": true}},
			userID:     userID,
			wantStatus: fiber.StatusOK,
		},
		{
			// 403, not 404: the caller is authenticated and the route exists.
			name:       "rejects a user without it",
			authz:      &stubAuthorization{allowed: map[string]bool{"hrm.view": true}},
			userID:     userID,
			wantStatus: fiber.StatusForbidden,
		},
		{
			// No user id means the gate was mounted without Auth in front of
			// it, which is an unauthenticated request rather than a forbidden
			// one.
			name:       "rejects an unauthenticated request",
			authz:      &stubAuthorization{allowed: map[string]bool{"hrm.manage": true}},
			userID:     uuid.Nil,
			wantStatus: fiber.StatusUnauthorized,
		},
		{
			// A resolver failure must not read as "not allowed": that would turn
			// a database blip into a shop-wide permissions outage nobody could
			// diagnose from the response.
			name:       "surfaces a resolver failure as its own error",
			authz:      &stubAuthorization{err: apperror.New(apperror.CodeDatabase, "boom")},
			userID:     userID,
			wantStatus: fiber.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newPermissionApp(tt.authz, "hrm.manage", tt.userID)

			res, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/guarded", nil))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer func() { _ = res.Body.Close() }()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", res.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestRequirePermissionDoesNotConsultTheResolverWhenUnauthenticated(t *testing.T) {
	authz := &stubAuthorization{allowed: map[string]bool{"hrm.manage": true}}
	app := newPermissionApp(authz, "hrm.manage", uuid.Nil)

	res, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/guarded", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	if authz.calls != 0 {
		t.Errorf("resolver was called %d times for an unauthenticated request", authz.calls)
	}
}
