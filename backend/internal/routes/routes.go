package routes

import (
	"github.com/gofiber/fiber/v2"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/handler"
)

type Handlers struct {
	Auth    *handler.AuthHandler
	User    *handler.UserHandler
	Role    *handler.RoleHandler
	Product *handler.ProductHandler
	Order   *handler.OrderHandler
	HRM     HRMHandlers
}

// Gates are the permission middlewares the routes mount. Only HRM uses them so
// far — endpoint gating for the POS routes is Phase 2 — so they are a struct
// rather than two more positional parameters that would need renaming when the
// next module arrives.
type Gates struct {
	HRMView   fiber.Handler
	HRMManage fiber.Handler
}

func Register(app *fiber.App, handlers Handlers, auth, authRateLimit fiber.Handler, gates Gates) {
	registerAuth(app, handlers.Auth, auth, authRateLimit)
	registerUsers(app, handlers.User, auth)
	registerRoles(app, handlers.Role, auth)
	registerProducts(app, handlers.Product, auth)
	registerOrders(app, handlers.Order, auth)
	registerHRM(app, handlers.HRM, auth, gates.HRMView, gates.HRMManage)
}
