package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/repository"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
)

/*
EmployeeReport is the headcount picture: totals, the split by job title, and the
split by contract type.
*/
type EmployeeReport struct {
	TotalEmployees    int64
	ActiveEmployees   int64
	MonthlySalaryCost int64
	ByDesignation     []repository.HeadcountRow
	ByEmploymentType  []repository.EmploymentTypeRow
}

/*
AttendanceReport is one window's register, per employee, with the employee rows
attached so the report can name people rather than ids.
*/
type AttendanceReport struct {
	Range     repository.DateRange
	Rows      []repository.AttendanceSummary
	Employees map[uuid.UUID]entity.Employee
}

// LeaveReport is one window's leave usage by type.
type LeaveReport struct {
	Range repository.DateRange
	Rows  []repository.LeaveReportRow
}

// PayrollReport is one year's payroll totals by month.
type PayrollReport struct {
	Year int
	Rows []repository.PayrollReportRow
}

/*
HRMReportService assembles the module's four reports.

It owns no writes and holds no rules of its own: each report is one aggregate
query plus whatever labelling the response needs. The totals are summed here
rather than in a second query, because the grouped rows are already in hand and
a shop has tens of designations, not millions.
*/
type HRMReportService interface {
	Employees(ctx context.Context, businessID uuid.UUID) (EmployeeReport, error)
	Attendance(ctx context.Context, businessID uuid.UUID, window DateWindow) (AttendanceReport, error)
	Leave(ctx context.Context, businessID uuid.UUID, window DateWindow) (LeaveReport, error)
	Payroll(ctx context.Context, businessID uuid.UUID, year int) (PayrollReport, error)
}

type hrmReportService struct {
	reports    repository.HRMReportRepository
	attendance AttendanceService
}

/*
NewHRMReportService takes AttendanceService rather than the attendance
repository: the attendance report is the summary endpoint's aggregate over a
different window, and building it twice would be two places for "how a register
is summarised" to drift apart.
*/
func NewHRMReportService(
	reports repository.HRMReportRepository, attendance AttendanceService,
) HRMReportService {
	return &hrmReportService{reports: reports, attendance: attendance}
}

func (s *hrmReportService) Employees(ctx context.Context, businessID uuid.UUID) (EmployeeReport, error) {
	byDesignation, err := s.reports.HeadcountByDesignation(ctx, businessID)
	if err != nil {
		return EmployeeReport{}, apperror.Wrap(apperror.CodeDatabase, "failed to build the employee report", err)
	}
	byType, err := s.reports.HeadcountByEmploymentType(ctx, businessID)
	if err != nil {
		return EmployeeReport{}, apperror.Wrap(apperror.CodeDatabase, "failed to build the employee report", err)
	}

	report := EmployeeReport{ByDesignation: byDesignation, ByEmploymentType: byType}
	for _, row := range byDesignation {
		report.TotalEmployees += row.Total
		report.ActiveEmployees += row.ActiveTotal
		report.MonthlySalaryCost += row.SalaryCents
	}
	return report, nil
}

// Attendance reports the register for a window, across every employee.
func (s *hrmReportService) Attendance(
	ctx context.Context, businessID uuid.UUID, window DateWindow,
) (AttendanceReport, error) {
	return s.attendance.Summary(ctx, businessID, uuid.Nil, window)
}

func (s *hrmReportService) Leave(
	ctx context.Context, businessID uuid.UUID, window DateWindow,
) (LeaveReport, error) {
	r := window.toRange()
	rows, err := s.reports.LeaveUsage(ctx, businessID, r)
	if err != nil {
		return LeaveReport{}, apperror.Wrap(apperror.CodeDatabase, "failed to build the leave report", err)
	}
	return LeaveReport{Range: r, Rows: rows}, nil
}

func (s *hrmReportService) Payroll(
	ctx context.Context, businessID uuid.UUID, year int,
) (PayrollReport, error) {
	rows, err := s.reports.PayrollTotals(ctx, businessID, year)
	if err != nil {
		return PayrollReport{}, apperror.Wrap(apperror.CodeDatabase, "failed to build the payroll report", err)
	}
	return PayrollReport{Year: year, Rows: rows}, nil
}
