package routes

import (
	"github.com/gofiber/fiber/v2"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/handler"
)

/*
HRMHandlers is the module's handler set, kept together so routes.Handlers grows
by one field rather than eleven.
*/
type HRMHandlers struct {
	Designation  *handler.DesignationHandler
	Shift        *handler.ShiftHandler
	Employee     *handler.EmployeeHandler
	Attendance   *handler.AttendanceHandler
	Leave        *handler.LeaveHandler
	Payroll      *handler.PayrollHandler
	Performance  *handler.PerformanceHandler
	Announcement *handler.AnnouncementHandler
	Report       *handler.HRMReportHandler
	Settings     *handler.HRMSettingsHandler
	Audit        *handler.HRMAuditHandler
}

/*
registerHRM mounts the module under /hrm.

Two permission gates, matching the two names the frontend's PERMISSIONS list
already carries: every read requires hrm.view, every write hrm.manage. The
groups are ordered so the read gate applies to the whole subtree and the write
gate is added per route — Fiber runs a group's middleware before the route's, so
a POST passes through both and a GET through one.

Payroll and leave decisions sit behind hrm.manage rather than a finer permission
of their own on purpose: a permission the frontend does not know about is
unreachable in the UI, and the two lists are the contract.
*/
func registerHRM(app *fiber.App, h HRMHandlers, auth, canView, canManage fiber.Handler) {
	group := app.Group("/hrm", auth, canView)

	registerHRMOrganization(group, h, canManage)
	registerHRMEmployees(group, h.Employee, canManage)
	registerHRMAttendance(group, h.Attendance, canManage)
	registerHRMLeave(group, h.Leave, canManage)
	registerHRMPayroll(group, h.Payroll, canManage)
	registerHRMPerformance(group, h, canManage)
	registerHRMReports(group, h)
	registerHRMSettings(group, h, canManage)
}

func registerHRMOrganization(group fiber.Router, h HRMHandlers, canManage fiber.Handler) {
	designations := group.Group("/designations")
	designations.Get("/", h.Designation.List)
	designations.Get("/:id", h.Designation.Get)
	designations.Post("/", canManage, h.Designation.Create)
	designations.Put("/:id", canManage, h.Designation.Update)
	designations.Delete("/:id", canManage, h.Designation.Delete)

	shifts := group.Group("/shifts")
	shifts.Get("/", h.Shift.List)
	shifts.Get("/:id", h.Shift.Get)
	shifts.Post("/", canManage, h.Shift.Create)
	shifts.Put("/:id", canManage, h.Shift.Update)
	shifts.Delete("/:id", canManage, h.Shift.Delete)
}

func registerHRMEmployees(group fiber.Router, h *handler.EmployeeHandler, canManage fiber.Handler) {
	employees := group.Group("/employees")
	/*
		"me" is registered before ":id" because Fiber matches in declaration
		order — the parameter route would otherwise swallow it and fail to parse
		"me" as a UUID.
	*/
	employees.Get("/me", h.Me)
	employees.Get("/", h.List)
	employees.Get("/:id", h.Get)
	employees.Get("/:id/documents", h.ListDocuments)

	employees.Post("/", canManage, h.Create)
	employees.Put("/:id", canManage, h.Update)
	employees.Delete("/:id", canManage, h.Delete)
	employees.Post("/:id/documents", canManage, h.UploadDocument)
	employees.Delete("/:id/documents/:documentId", canManage, h.DeleteDocument)
}

func registerHRMAttendance(group fiber.Router, h *handler.AttendanceHandler, canManage fiber.Handler) {
	attendance := group.Group("/attendance")
	attendance.Get("/summary", h.Summary)
	attendance.Get("/", h.List)
	attendance.Get("/:id", h.Get)

	/*
		Clocking in and out needs hrm.view only: a cashier punches their own card
		and is not an HR administrator. Naming another employee in the body is
		still possible, which is why the handler falls back to the caller's own
		record and a shop that wants stricter separation gates the terminal, not
		the endpoint.
	*/
	attendance.Post("/clock-in", h.ClockIn)
	attendance.Post("/clock-out", h.ClockOut)

	attendance.Post("/", canManage, h.Record)
	attendance.Delete("/:id", canManage, h.Delete)
}

func registerHRMLeave(group fiber.Router, h *handler.LeaveHandler, canManage fiber.Handler) {
	types := group.Group("/leave-types")
	types.Get("/", h.ListTypes)
	types.Post("/", canManage, h.CreateType)
	types.Put("/:id", canManage, h.UpdateType)
	types.Delete("/:id", canManage, h.DeleteType)

	requests := group.Group("/leave-requests")
	requests.Get("/", h.List)
	requests.Get("/:id", h.Get)
	// Applying and withdrawing are things an employee does for themselves.
	requests.Post("/", h.Request)
	requests.Post("/:id/cancel", h.Cancel)
	// Deciding is not.
	requests.Post("/:id/approve", canManage, h.Approve)
	requests.Post("/:id/reject", canManage, h.Reject)

	group.Get("/leave-balances", h.Balances)
}

func registerHRMPayroll(group fiber.Router, h *handler.PayrollHandler, canManage fiber.Handler) {
	payroll := group.Group("/payroll")

	payroll.Get("/runs", h.ListRuns)
	payroll.Get("/runs/:id", h.GetRun)
	payroll.Post("/runs", canManage, h.Generate)
	payroll.Post("/runs/:id/finalize", canManage, h.Finalize)
	payroll.Post("/runs/:id/pay", canManage, h.MarkPaid)
	payroll.Delete("/runs/:id", canManage, h.DeleteRun)

	payroll.Get("/payslips", h.ListPayslips)
	payroll.Get("/payslips/:id", h.GetPayslip)
	payroll.Put("/payslips/:id", canManage, h.AdjustPayslip)
}

func registerHRMPerformance(group fiber.Router, h HRMHandlers, canManage fiber.Handler) {
	performance := group.Group("/performance")
	performance.Get("/", h.Performance.List)
	performance.Get("/:id", h.Performance.Get)
	performance.Post("/", canManage, h.Performance.Create)
	performance.Put("/:id", canManage, h.Performance.Update)
	performance.Delete("/:id", canManage, h.Performance.Delete)

	announcements := group.Group("/announcements")
	announcements.Get("/", h.Announcement.List)
	announcements.Get("/:id", h.Announcement.Get)
	announcements.Post("/", canManage, h.Announcement.Create)
	announcements.Put("/:id", canManage, h.Announcement.Update)
	announcements.Delete("/:id", canManage, h.Announcement.Delete)
}

func registerHRMReports(group fiber.Router, h HRMHandlers) {
	reports := group.Group("/reports")
	reports.Get("/employees", h.Report.Employees)
	reports.Get("/attendance", h.Report.Attendance)
	reports.Get("/leave", h.Report.Leave)
	reports.Get("/payroll", h.Report.Payroll)

	// The audit trail is read-only: entries are written by the audit worker off
	// the request path, never by an endpoint.
	group.Get("/audit-logs", h.Audit.List)
}

func registerHRMSettings(group fiber.Router, h HRMHandlers, canManage fiber.Handler) {
	group.Get("/settings", h.Settings.Get)
	group.Put("/settings", canManage, h.Settings.Update)
}
