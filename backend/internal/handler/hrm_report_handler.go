package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/mapper"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/middleware"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/pagination"
)

/*
HRMReportHandler serves the module's four reports.

None of them are paginated: each is a whole picture — a headcount, a window, a
year — and a page of one would answer nothing. They are gated on hrm.view like
the rest of the module's reads.
*/
type HRMReportHandler struct {
	reports service.HRMReportService
}

func NewHRMReportHandler(reports service.HRMReportService) *HRMReportHandler {
	return &HRMReportHandler{reports: reports}
}

// Employees serves GET /hrm/reports/employees: headcount and wage bill, split
// by designation and by contract type.
func (h *HRMReportHandler) Employees(c *fiber.Ctx) error {
	report, err := h.reports.Employees(c.UserContext(), middleware.BusinessID(c))
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToEmployeeReportResponse(report))
}

/*
Attendance serves GET /hrm/reports/attendance: the register per employee over a
window, defaulting to the current month.
*/
func (h *HRMReportHandler) Attendance(c *fiber.Ctx) error {
	window, err := h.window(c)
	if err != nil {
		return err
	}

	report, err := h.reports.Attendance(c.UserContext(), middleware.BusinessID(c), window)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToAttendanceReportResponse(report))
}

// Leave serves GET /hrm/reports/leave: usage by leave type over a window.
func (h *HRMReportHandler) Leave(c *fiber.Ctx) error {
	window, err := h.window(c)
	if err != nil {
		return err
	}

	report, err := h.reports.Leave(c.UserContext(), middleware.BusinessID(c), window)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToLeaveReportResponse(report))
}

/*
Payroll serves GET /hrm/reports/payroll: a year of monthly totals, defaulting to
the current year.
*/
func (h *HRMReportHandler) Payroll(c *fiber.Ctx) error {
	var query dto.PayrollReportQuery
	if err := parseQuery(c, &query); err != nil {
		return err
	}

	year := query.Year
	if year == 0 {
		year = time.Now().Year()
	}

	report, err := h.reports.Payroll(c.UserContext(), middleware.BusinessID(c), year)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToPayrollReportResponse(report))
}

/*
window reads the from/to bounds shared by the two window reports, defaulting to
the current calendar month. A default rather than "everything" because an
unbounded attendance report over three years of history is a query nobody meant
to run.
*/
func (h *HRMReportHandler) window(c *fiber.Ctx) (service.DateWindow, error) {
	var query dto.ReportRangeQuery
	if err := parseQuery(c, &query); err != nil {
		return service.DateWindow{}, err
	}

	if query.From == "" && query.To == "" {
		now := time.Now()
		return monthWindow(now.Year(), int(now.Month())), nil
	}
	return parseDateWindow(query.From, query.To)
}

/*
HRMSettingsHandler serves the module's policy: attendance thresholds, leave
rules and payroll rates.
*/
type HRMSettingsHandler struct {
	settings service.HRMSettingsService
	audit    service.AuditService
}

func NewHRMSettingsHandler(
	settings service.HRMSettingsService, audit service.AuditService,
) *HRMSettingsHandler {
	return &HRMSettingsHandler{settings: settings, audit: audit}
}

/*
Get serves GET /hrm/settings.

A business that has never saved settings gets the module's defaults rather than
an empty object, so the settings screen has something to render and the payroll
run has a divisor.
*/
func (h *HRMSettingsHandler) Get(c *fiber.Ctx) error {
	settings, err := h.settings.Get(c.UserContext(), middleware.BusinessID(c))
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToHRMSettingsResponse(settings))
}

/*
Update serves PUT /hrm/settings: all three groups at once, matching how the
screen submits them.

The change is audited by value: payroll rates decide what people are paid, and a
silent edit to the EPF percentage would otherwise be invisible after the fact.
*/
func (h *HRMSettingsHandler) Update(c *fiber.Ctx) error {
	var req dto.HRMSettingsResponse
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	saved, err := h.settings.Update(
		c.UserContext(), middleware.BusinessID(c), mapper.ToHRMSettings(req),
	)
	if err != nil {
		return err
	}

	recordAudit(c, h.audit, service.AuditActionSettingsUpdate, "hrm.settings", nil, req)
	return ok(c, fiber.StatusOK, mapper.ToHRMSettingsResponse(saved))
}

/*
HRMAuditHandler serves the audit trail.
*/
type HRMAuditHandler struct {
	logs service.AuditQueryService
}

func NewHRMAuditHandler(logs service.AuditQueryService) *HRMAuditHandler {
	return &HRMAuditHandler{logs: logs}
}

/*
List serves GET /hrm/audit-logs: who did what, in which module, when, and from
which address.

`module` is a prefix match on the resource type, so module=hrm returns the whole
HR trail without the caller enumerating its resources. The trail itself is
business-scoped and append-only — there is no write endpoint here by design.
*/
func (h *HRMAuditHandler) List(c *fiber.Ctx) error {
	var query dto.AuditLogListQuery
	if err := parseQuery(c, &query); err != nil {
		return err
	}

	params, err := listParams(query.PaginationQuery, dto.AuditLogSortFields)
	if err != nil {
		return err
	}
	window, err := parseDateWindow(query.From, query.To)
	if err != nil {
		return err
	}
	userID, err := optionalID(query.UserID, "user_id")
	if err != nil {
		return err
	}

	rows, total, err := h.logs.Search(c.UserContext(), middleware.BusinessID(c), service.AuditQuery{
		ListQuery: toListQuery(params),
		UserID:    userID,
		Action:    query.Action,
		Module:    query.Module,
		Status:    query.Status,
		Window:    window,
	})
	if err != nil {
		return err
	}

	return ok(c, fiber.StatusOK, dto.NewListResponse(
		mapper.ToAuditLogResponseList(rows), pagination.NewMeta(params, total),
	))
}
