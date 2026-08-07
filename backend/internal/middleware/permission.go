package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
)

// RequirePermission rejects a request whose user does not hold permission.
//
// Must be mounted after Auth: it reads the user id Auth put on the context, and
// a missing one means the route is unauthenticated rather than unauthorised.
//
// The check is a database read behind a short-lived cache rather than a claim
// on the token. The access token lives 12 hours (cashiers work long shifts), so
// a permission baked into it would survive a revocation for the rest of the
// day; resolving per request bounds that to AUTH_PERMISSION_CACHE_TTL.
func RequirePermission(authz service.AuthorizationService, permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := UserID(c)
		if userID == uuid.Nil {
			return apperror.New(apperror.CodeUnauthenticated, "authentication is required")
		}

		allowed, err := authz.Has(c.UserContext(), userID, permission)
		if err != nil {
			return err
		}
		if !allowed {
			// Deliberately generic: naming the missing permission would tell an
			// attacker which one to go after, and it tells a legitimate user
			// nothing they can act on either.
			return apperror.New(apperror.CodeForbidden, "you do not have permission to do this")
		}

		return c.Next()
	}
}
