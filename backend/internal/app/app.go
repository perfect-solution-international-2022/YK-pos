package app

import (
	"github.com/gofiber/fiber/v2"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/middleware"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/routes"
)

// Permission names the HRM routes gate on. Seeded by migration 00017 and
// mirrored in the frontend's PERMISSIONS list — that list is the contract, so a
// name added here has to exist on both sides or it is unreachable in the UI.
const (
	permissionHRMView   = "hrm.view"
	permissionHRMManage = "hrm.manage"
)

func New(container *Container) *fiber.App {
	cfg := container.Config
	app := fiber.New(fiber.Config{
		AppName:                 cfg.App.Name,
		BodyLimit:               cfg.HTTP.BodyLimit,
		ReadTimeout:             cfg.HTTP.ReadTimeout,
		WriteTimeout:            cfg.HTTP.WriteTimeout,
		IdleTimeout:             cfg.HTTP.IdleTimeout,
		EnableTrustedProxyCheck: len(cfg.HTTP.TrustedProxies) > 0,
		TrustedProxies:          cfg.HTTP.TrustedProxies,
		ErrorHandler:            middleware.ErrorHandler(container.Logger),
	})

	app.Use(middleware.Recover())
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(container.Logger))
	app.Use(middleware.Helmet())
	app.Use(middleware.CORS(cfg.Security))
	app.Use(middleware.Compress())

	routes.RegisterSystem(app, container.Health)

	app.Use(middleware.RateLimit(
		cfg.Redis,
		cfg.Security.RateLimitMax,
		cfg.Security.RateLimitWindow,
	))
	routes.Register(
		app,
		container.Handlers,
		middleware.Auth(container.Tokens),
		middleware.AuthRateLimit(
			cfg.Redis,
			cfg.Security.AuthRateLimitMax,
			cfg.Security.AuthRateLimitWindow,
		),
		// Permission gates. Mounted after Auth on the routes that use them, so
		// they read the user id Auth put on the context.
		routes.Gates{
			HRMView:   middleware.RequirePermission(container.Authorization, permissionHRMView),
			HRMManage: middleware.RequirePermission(container.Authorization, permissionHRMManage),
		},
	)
	return app
}
