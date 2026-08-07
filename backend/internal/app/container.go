package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/SandaruwanWeerawardhana/pos-backend/config"
	"github.com/SandaruwanWeerawardhana/pos-backend/database"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/handler"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/middleware"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/repository"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/routes"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
	pkgjwt "github.com/SandaruwanWeerawardhana/pos-backend/pkg/jwt"
	pkglogger "github.com/SandaruwanWeerawardhana/pos-backend/pkg/logger"
)

const auditQueueSize = 1024

type Container struct {
	Config      *config.Config
	Logger      *slog.Logger
	DB          *gorm.DB
	Redis       *redis.Client
	Tokens      service.TokenService
	Permissions service.PermissionService
	// Authorization resolves a user's effective permissions for the route
	// gates. Separate from Permissions, which owns the permission catalogue.
	Authorization service.AuthorizationService
	AuditWorker   *middleware.AuditWorker
	Handlers      routes.Handlers
	Health        *handler.HealthHandler
}

func NewContainer(ctx context.Context) (*Container, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	logger := pkglogger.New(cfg.App.LogFormat, cfg.App.LogLevel)

	db, err := database.NewPostgres(cfg.DB)
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("app: unwrap postgres: %w", err)
	}
	closeDB := true
	defer func() {
		if closeDB {
			_ = sqlDB.Close()
		}
	}()

	if cfg.DB.AutoMigrate {
		migrator, migrateErr := database.NewMigrator(sqlDB)
		if migrateErr != nil {
			return nil, migrateErr
		}
		if migrateErr = migrator.Up(); migrateErr != nil {
			return nil, fmt.Errorf("app: migrate database: %w", migrateErr)
		}
	}

	redisClient, err := database.NewRedis(ctx, cfg.Redis)
	if err != nil {
		return nil, err
	}

	/*
		A nil client means REDIS_ENABLED=false: the denylist and the rate
		limiter degrade to process-local memory, which only holds up for a
		single local instance.
	*/
	denylist := service.TokenDenylist(service.NewMemoryDenylist())
	if redisClient != nil {
		denylist = service.NewRedisDenylist(redisClient)
	} else {
		logger.Warn("redis disabled: token denylist and rate limits are in-memory, per-process, and lost on restart")
	}

	users := repository.NewUserRepository(db)
	businesses := repository.NewBusinessRepository(db)
	branches := repository.NewBranchRepository(db)
	roles := repository.NewRoleRepository(db)
	permissions := repository.NewPermissionRepository(db)
	refreshTokens := repository.NewRefreshTokenRepository(db)
	passwordResets := repository.NewPasswordResetRepository(db)
	auditLogs := repository.NewAuditLogRepository(db)
	products := repository.NewProductRepository(db)
	/*
		Non-transactional binding, for the read path. Order sync rebinds its own
		repositories to each order's transaction and does not use this one.
	*/
	orders := repository.NewOrderRepository(db)
	tx := repository.NewTxManager(db)

	/*
		HRM. Every repository here is scoped by business_id the same way the POS
		ones are; settings reuses the existing settings table rather than adding
		three tables for three JSON documents, and the audit trail is read
		through its own query repository so the append-only write side keeps its
		small interface.
	*/
	designations := repository.NewDesignationRepository(db)
	shifts := repository.NewShiftRepository(db)
	employees := repository.NewEmployeeRepository(db)
	attendance := repository.NewAttendanceRepository(db)
	leaveTypes := repository.NewLeaveTypeRepository(db)
	leaveRequests := repository.NewLeaveRequestRepository(db)
	payrollRuns := repository.NewPayrollRepository(db)
	performanceReviews := repository.NewPerformanceRepository(db)
	announcements := repository.NewAnnouncementRepository(db)
	hrmReports := repository.NewHRMReportRepository(db)
	settings := repository.NewSettingRepository(db)
	auditLogQueries := repository.NewAuditLogQueryRepository(db)

	issuer := pkgjwt.NewIssuer(
		cfg.JWT.AccessSecret,
		cfg.JWT.Issuer,
		cfg.JWT.Audience,
		cfg.JWT.AccessTTL,
	)
	tokenService := service.NewTokenService(refreshTokens, denylist, issuer, cfg.JWT.RefreshTTL)
	auditService := service.NewAuditService(auditLogs)
	auditWorker := middleware.NewAuditWorker(auditService, auditQueueSize, logger)
	authService := service.NewAuthService(service.AuthServiceDeps{
		Users:      users,
		Businesses: businesses,
		Branches:   branches,
		Roles:      roles,
		Tokens:     tokenService,
		Audit:      auditWorker,
		Tx:         tx,
		NewTxRepos: service.DefaultTxRepos,
	}, service.AuthServiceConfig{
		BcryptCost:          cfg.Bcrypt.Cost,
		MaxFailedLogins:     cfg.Auth.MaxFailedLogins,
		LockoutDuration:     cfg.Auth.LockoutDuration,
		RegistrationEnabled: cfg.Auth.RegistrationEnabled,
	})
	userService := service.NewUserService(users, cfg.Bcrypt.Cost, roles)
	roleService := service.NewRoleService(roles)
	permissionService := service.NewPermissionService(permissions)
	passwordResetService := service.NewPasswordResetService(
		users, passwordResets, tokenService, cfg.Bcrypt.Cost,
	)
	productService := service.NewProductService(products)
	orderSyncService := service.NewOrderSyncService(tx, service.DefaultOrderTxRepos)
	orderService := service.NewOrderService(orders)

	/*
		Permissions are resolved per request behind a TTL cache rather than being
		baked into the access token: the token lives 12 hours (cashiers work long
		shifts), so a permission carried in a claim would survive a revocation
		for the rest of the day. AUTH_PERMISSION_CACHE_TTL bounds that instead.
	*/
	authorizationService := service.NewAuthorizationService(users, roles, cfg.Auth.PermissionCacheTTL)

	hrmSettingsService := service.NewHRMSettingsService(settings)
	designationService := service.NewDesignationService(designations)
	shiftService := service.NewShiftService(shifts)
	employeeService := service.NewEmployeeService(service.EmployeeServiceDeps{
		Employees:    employees,
		Designations: designations,
		Shifts:       shifts,
		Users:        users,
		Roles:        roles,
		Tx:           tx,
		NewTxRepos:   service.DefaultEmployeeTxRepos,
	}, cfg.Bcrypt.Cost)
	attendanceService := service.NewAttendanceService(attendance, employees, shifts, hrmSettingsService)
	leaveService := service.NewLeaveService(
		leaveTypes, leaveRequests, employees, shifts, attendance, hrmSettingsService,
	)
	payrollService := service.NewPayrollService(
		payrollRuns, employees, attendance, leaveRequests, hrmSettingsService, tx,
	)
	performanceService := service.NewPerformanceService(performanceReviews, employees)
	announcementService := service.NewAnnouncementService(announcements)
	// The report service takes AttendanceService, not the repository: its
	// attendance report is the summary endpoint's aggregate over a different
	// window, and building it twice would be two places to drift.
	hrmReportService := service.NewHRMReportService(hrmReports, attendanceService)
	auditQueryService := service.NewAuditQueryService(auditLogQueries)

	closeDB = false
	return &Container{
		Config:        cfg,
		Logger:        logger,
		DB:            db,
		Redis:         redisClient,
		Tokens:        tokenService,
		Permissions:   permissionService,
		Authorization: authorizationService,
		AuditWorker:   auditWorker,
		Handlers: routes.Handlers{
			/*
				The reset token is echoed back in the response only in a local
				environment, where there is no mail server to deliver it.
			*/
			Auth:    handler.NewAuthHandler(authService, passwordResetService, cfg.App.IsLocal()),
			User:    handler.NewUserHandler(userService),
			Role:    handler.NewRoleHandler(roleService),
			Product: handler.NewProductHandler(productService),
			Order:   handler.NewOrderHandler(orderSyncService, orderService),
			/*
				The audit worker is passed to the HRM handlers that write:
				personnel changes, attendance corrections, leave decisions,
				payroll and settings are exactly what an auditor asks about
				later, and the request-scoped detail those entries carry (IP,
				user agent, request id) only exists at the handler layer.
			*/
			HRM: routes.HRMHandlers{
				Designation:  handler.NewDesignationHandler(designationService),
				Shift:        handler.NewShiftHandler(shiftService),
				Employee:     handler.NewEmployeeHandler(employeeService, auditWorker),
				Attendance:   handler.NewAttendanceHandler(attendanceService, employeeService, auditWorker),
				Leave:        handler.NewLeaveHandler(leaveService, auditWorker),
				Payroll:      handler.NewPayrollHandler(payrollService, auditWorker),
				Performance:  handler.NewPerformanceHandler(performanceService),
				Announcement: handler.NewAnnouncementHandler(announcementService),
				Report:       handler.NewHRMReportHandler(hrmReportService),
				Settings:     handler.NewHRMSettingsHandler(hrmSettingsService, auditWorker),
				Audit:        handler.NewHRMAuditHandler(auditQueryService),
			},
		},
		Health: handler.NewHealthHandler(sqlDB, redisClient),
	}, nil
}

func (c *Container) Close() error {
	c.AuditWorker.Close()

	var closeErr error
	if sqlDB, err := c.DB.DB(); err != nil {
		closeErr = err
	} else if err = sqlDB.Close(); err != nil {
		closeErr = err
	}
	if c.Redis != nil {
		if err := c.Redis.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}
