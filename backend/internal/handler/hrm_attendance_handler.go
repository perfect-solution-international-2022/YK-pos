package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/mapper"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/middleware"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/service"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/pagination"
)

/*
AttendanceHandler serves the daily register.
*/
type AttendanceHandler struct {
	attendance service.AttendanceService
	employees  service.EmployeeService
	audit      service.AuditService
}

func NewAttendanceHandler(
	attendance service.AttendanceService,
	employees service.EmployeeService,
	audit service.AuditService,
) *AttendanceHandler {
	return &AttendanceHandler{attendance: attendance, employees: employees, audit: audit}
}

// List serves GET /hrm/attendance: one page of the register, filterable by
// employee, status and date window.
func (h *AttendanceHandler) List(c *fiber.Ctx) error {
	var query dto.AttendanceListQuery
	if err := parseQuery(c, &query); err != nil {
		return err
	}

	params, err := listParams(query.PaginationQuery, dto.AttendanceSortFields)
	if err != nil {
		return err
	}
	window, err := parseDateWindow(query.From, query.To)
	if err != nil {
		return err
	}
	employeeID, err := optionalID(query.EmployeeID, "employee_id")
	if err != nil {
		return err
	}

	rows, total, err := h.attendance.List(c.UserContext(), middleware.BusinessID(c), service.AttendanceQuery{
		ListQuery:  toListQuery(params),
		EmployeeID: employeeID,
		Status:     query.Status,
		Window:     window,
	})
	if err != nil {
		return err
	}

	return ok(c, fiber.StatusOK, dto.NewListResponse(
		mapper.ToAttendanceResponseList(rows), pagination.NewMeta(params, total),
	))
}

func (h *AttendanceHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	row, err := h.attendance.Get(c.UserContext(), middleware.BusinessID(c), id)
	if err != nil {
		return err
	}
	return ok(c, fiber.StatusOK, mapper.ToAttendanceResponse(*row))
}

/*
ClockIn serves POST /hrm/attendance/clock-in.

An omitted employee_id means the caller is punching themselves in, resolved
through the employee record linked to their login. Naming somebody else is a
supervisor action, which the route's hrm.manage gate covers.
*/
func (h *AttendanceHandler) ClockIn(c *fiber.Ctx) error {
	employeeID, at, err := h.resolveClock(c)
	if err != nil {
		return err
	}

	row, err := h.attendance.ClockIn(
		c.UserContext(), middleware.BusinessID(c), employeeID, at, actorID(c),
	)
	if err != nil {
		return err
	}

	recordAudit(c, h.audit, service.AuditActionAttendanceClockIn, "hrm.attendance", &row.ID, map[string]any{
		"employee_id":  employeeID.String(),
		"work_date":    row.WorkDate.Format(time.DateOnly),
		"late_minutes": row.LateMinutes,
	})

	return ok(c, fiber.StatusOK, mapper.ToAttendanceResponse(*row))
}

// ClockOut serves POST /hrm/attendance/clock-out, closing the open day and
// computing what it was worth.
func (h *AttendanceHandler) ClockOut(c *fiber.Ctx) error {
	employeeID, at, err := h.resolveClock(c)
	if err != nil {
		return err
	}

	row, err := h.attendance.ClockOut(
		c.UserContext(), middleware.BusinessID(c), employeeID, at, actorID(c),
	)
	if err != nil {
		return err
	}

	recordAudit(c, h.audit, service.AuditActionAttendanceClockOut, "hrm.attendance", &row.ID, map[string]any{
		"employee_id":      employeeID.String(),
		"work_date":        row.WorkDate.Format(time.DateOnly),
		"worked_minutes":   row.WorkedMinutes,
		"overtime_minutes": row.OvertimeMinutes,
	})

	return ok(c, fiber.StatusOK, mapper.ToAttendanceResponse(*row))
}

/*
Record serves POST /hrm/attendance: a supervisor writing a day by hand, or an
entry for somebody who does not punch a clock at all.

Creates or replaces whichever row the date already holds — replacement, not a
patch, because "absent, no times" has to be expressible and a sparse update
could not tell that from "left the times alone".
*/
func (h *AttendanceHandler) Record(c *fiber.Ctx) error {
	var req dto.AttendanceRecordRequest
	if err := parseAndValidate(c, &req); err != nil {
		return err
	}

	input, err := mapper.ToManualAttendanceInput(req, actorID(c))
	if err != nil {
		return apperror.Wrap(apperror.CodeValidationError, "invalid employee id", err)
	}

	row, err := h.attendance.Record(c.UserContext(), middleware.BusinessID(c), input)
	if err != nil {
		return err
	}

	recordAudit(c, h.audit, service.AuditActionAttendanceRecord, "hrm.attendance", &row.ID, map[string]any{
		"employee_id": row.EmployeeID.String(),
		"work_date":   row.WorkDate.Format(time.DateOnly),
		"status":      row.Status,
	})

	return ok(c, fiber.StatusOK, mapper.ToAttendanceResponse(*row))
}

// Delete serves DELETE /hrm/attendance/{id} → 204, for a row entered against
// the wrong person or the wrong day.
func (h *AttendanceHandler) Delete(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return err
	}

	if err := h.attendance.Delete(c.UserContext(), middleware.BusinessID(c), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

/*
Summary serves GET /hrm/attendance/summary: per-employee totals over a window.

The window can be given as from/to or as year/month, because "March 2026" is
what a report screen asks for and making the client work out the last day of the
month to express it would be a needless trap. Absent both, it defaults to the
current month.
*/
func (h *AttendanceHandler) Summary(c *fiber.Ctx) error {
	var query dto.AttendanceSummaryQuery
	if err := parseQuery(c, &query); err != nil {
		return err
	}

	window, err := h.resolveWindow(query)
	if err != nil {
		return err
	}

	employeeID := uuid.Nil
	if query.EmployeeID != "" {
		employeeID, err = parseID(query.EmployeeID, "employee_id")
		if err != nil {
			return err
		}
	}

	report, err := h.attendance.Summary(
		c.UserContext(), middleware.BusinessID(c), employeeID, window,
	)
	if err != nil {
		return err
	}

	return ok(c, fiber.StatusOK, mapper.ToAttendanceReportResponse(report))
}

/*
resolveClock reads the target employee and the punch time.

The time defaults to the server's clock; a client-supplied one is honoured so a
terminal that was offline can post the real punch rather than the moment it
reconnected.
*/
func (h *AttendanceHandler) resolveClock(c *fiber.Ctx) (uuid.UUID, time.Time, error) {
	var req dto.ClockRequest
	if err := parseAndValidate(c, &req); err != nil {
		return uuid.Nil, time.Time{}, err
	}

	at := time.Now()
	if req.At != nil && *req.At > 0 {
		at = time.UnixMilli(*req.At)
	}

	if req.EmployeeID != "" {
		employeeID, err := parseID(req.EmployeeID, "employee_id")
		return employeeID, at, err
	}

	employee, err := h.employees.GetByUserID(
		c.UserContext(), middleware.BusinessID(c), middleware.UserID(c),
	)
	if err != nil {
		return uuid.Nil, time.Time{}, err
	}
	return employee.ID, at, nil
}

// resolveWindow turns the summary query's two forms into one window, defaulting
// to the current month.
func (h *AttendanceHandler) resolveWindow(query dto.AttendanceSummaryQuery) (service.DateWindow, error) {
	if query.From != "" || query.To != "" {
		return parseDateWindow(query.From, query.To)
	}

	year, month := query.Year, query.Month
	now := time.Now()
	if year == 0 {
		year = now.Year()
	}
	if month == 0 {
		month = int(now.Month())
	}
	return monthWindow(year, month), nil
}
