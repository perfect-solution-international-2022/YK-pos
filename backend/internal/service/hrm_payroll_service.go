package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/repository"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
)

/*
GeneratePayrollInput asks for one month's payroll.

EmployeeIDs narrows the run to a subset — a shop paying its contract staff on a
different date than its permanent ones — and an empty list means everybody
active.
*/
type GeneratePayrollInput struct {
	Year        int
	Month       int
	EmployeeIDs []uuid.UUID
	Notes       *string
	GeneratedBy *uuid.UUID
}

/*
PayslipAdjustment is a manager's edit to a draft payslip: the ad-hoc earnings
and deductions, replacing whatever was there before.
*/
type PayslipAdjustment struct {
	Items []entity.PayslipItem
}

/*
PayrollService owns salary calculation.

Everything it computes is integer cents. Rates arrive as percentages (8 = 8%)
and are divided by 100 at the point of use; day counts are fractional because a
half day is 0.5. Every division rounds to the nearest cent exactly once, at the
end of the line it belongs to, so the figures on a payslip add up to its total
rather than drifting by a cent per line.
*/
type PayrollService interface {
	ListRuns(ctx context.Context, businessID uuid.UUID, query ListQuery) ([]entity.PayrollRun, int64, error)
	GetRun(ctx context.Context, businessID, id uuid.UUID) (*entity.PayrollRun, error)
	Generate(ctx context.Context, businessID uuid.UUID, in GeneratePayrollInput) (*entity.PayrollRun, error)
	Finalize(ctx context.Context, businessID, id uuid.UUID) (*entity.PayrollRun, error)
	MarkPaid(ctx context.Context, businessID, id uuid.UUID) (*entity.PayrollRun, error)
	DeleteRun(ctx context.Context, businessID, id uuid.UUID) error

	ListPayslips(ctx context.Context, businessID uuid.UUID, query PayslipQuery) ([]entity.Payslip, int64, error)
	GetPayslip(ctx context.Context, businessID, id uuid.UUID) (*entity.Payslip, error)
	AdjustPayslip(ctx context.Context, businessID, id uuid.UUID, adjustment PayslipAdjustment) (*entity.Payslip, error)
}

type payrollService struct {
	payroll    repository.PayrollRepository
	employees  repository.EmployeeRepository
	attendance repository.AttendanceRepository
	leave      repository.LeaveRequestRepository
	settings   HRMSettingsService
	tx         *repository.TxManager
}

func NewPayrollService(
	payroll repository.PayrollRepository,
	employees repository.EmployeeRepository,
	attendance repository.AttendanceRepository,
	leave repository.LeaveRequestRepository,
	settings HRMSettingsService,
	tx *repository.TxManager,
) PayrollService {
	return &payrollService{
		payroll:    payroll,
		employees:  employees,
		attendance: attendance,
		leave:      leave,
		settings:   settings,
		tx:         tx,
	}
}

func (s *payrollService) ListRuns(
	ctx context.Context, businessID uuid.UUID, query ListQuery,
) ([]entity.PayrollRun, int64, error) {
	rows, total, err := s.payroll.ListRuns(ctx, businessID, query.toParams())
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "failed to list payroll runs", err)
	}
	return rows, total, nil
}

func (s *payrollService) GetRun(ctx context.Context, businessID, id uuid.UUID) (*entity.PayrollRun, error) {
	row, err := s.payroll.FindRunByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "payroll run not found", "failed to load payroll run")
	}
	return row, nil
}

/*
Generate computes a month's payroll.

Regenerating a draft is allowed and replaces its payslips wholesale — attendance
gets corrected after the fact, and a manager needs to be able to re-run the month
rather than patch twenty slips by hand. A finalised or paid run is not
regenerable: those figures have been shown to staff.

The run and its payslips are written in one transaction, so a half-computed
month — some people paid, some missing — is never visible to a report.
*/
func (s *payrollService) Generate(
	ctx context.Context, businessID uuid.UUID, in GeneratePayrollInput,
) (*entity.PayrollRun, error) {
	if in.Month < 1 || in.Month > 12 {
		return nil, apperror.New(apperror.CodeValidationError, "month must be between 1 and 12")
	}

	settings, err := s.settings.Get(ctx, businessID)
	if err != nil {
		return nil, err
	}
	if settings.Payroll.WorkingDaysPerMonth <= 0 {
		return nil, apperror.New(
			apperror.CodeValidationError,
			"payroll settings must define a positive number of working days per month",
		)
	}

	run, err := s.resolveRun(ctx, businessID, in, settings.Payroll)
	if err != nil {
		return nil, err
	}

	employees, err := s.employees.ListPayable(ctx, businessID, in.EmployeeIDs)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to load employees", err)
	}
	if len(employees) == 0 {
		return nil, apperror.New(apperror.CodeValidationError, "there are no active employees to pay")
	}

	period := monthRange(in.Year, in.Month)
	ids := make([]uuid.UUID, 0, len(employees))
	for _, e := range employees {
		ids = append(ids, e.ID)
	}

	attendance, err := s.attendance.SummarizeByEmployee(ctx, businessID, period, ids)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to summarise attendance", err)
	}
	attendanceByEmployee := make(map[uuid.UUID]repository.AttendanceSummary, len(attendance))
	for _, row := range attendance {
		attendanceByEmployee[row.EmployeeID] = row
	}

	leave, err := s.leave.SumApprovedByPayType(ctx, businessID, period, ids)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to total approved leave", err)
	}
	leaveByEmployee := make(map[uuid.UUID]repository.LeaveDaysByPayType, len(leave))
	for _, row := range leave {
		leaveByEmployee[row.EmployeeID] = row
	}

	payslips := make([]entity.Payslip, 0, len(employees))
	for _, employee := range employees {
		payslips = append(payslips, computePayslip(
			employee, run, attendanceByEmployee[employee.ID], leaveByEmployee[employee.ID], settings.Payroll,
		))
	}

	err = s.tx.WithTransaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
		if run.CreatedAt.IsZero() {
			if createErr := s.payroll.CreateRun(ctx, tx, run); createErr != nil {
				return apperror.Wrap(apperror.CodeDatabase, "failed to create the payroll run", createErr)
			}
		}
		if replaceErr := s.payroll.ReplacePayslips(ctx, tx, run.ID, payslips); replaceErr != nil {
			return apperror.Wrap(apperror.CodeDatabase, "failed to write payslips", replaceErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetRun(ctx, businessID, run.ID)
}

/*
resolveRun returns the run to write into: the existing draft for the period, or
a new one. A finalised or paid run is refused here rather than deeper in, so the
expensive aggregation below never runs for a request that cannot succeed.
*/
func (s *payrollService) resolveRun(
	ctx context.Context, businessID uuid.UUID, in GeneratePayrollInput, rules PayrollRules,
) (*entity.PayrollRun, error) {
	snapshot, err := json.Marshal(rules)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "failed to snapshot payroll rules", err)
	}

	existing, err := s.payroll.FindRunByPeriod(ctx, businessID, in.Year, in.Month)
	switch {
	case err == nil && !existing.IsEditable():
		return nil, apperror.New(
			apperror.CodeConflict,
			"payroll for this month has already been finalised",
		)
	case err == nil:
		existing.Rules = datatypes.JSON(snapshot)
		if in.Notes != nil {
			existing.Notes = in.Notes
		}
		return existing, nil
	case !errors.Is(err, repository.ErrNotFound):
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to check the payroll period", err)
	}

	return &entity.PayrollRun{
		BusinessID:  businessID,
		PeriodYear:  in.Year,
		PeriodMonth: in.Month,
		Status:      entity.PayrollStatusDraft,
		Rules:       datatypes.JSON(snapshot),
		Notes:       in.Notes,
		GeneratedBy: in.GeneratedBy,
	}, nil
}

/*
Finalize freezes a run and every payslip in it. One-way: reopening would let a
figure change after staff had been shown it, which is the thing the status
exists to prevent.
*/
func (s *payrollService) Finalize(ctx context.Context, businessID, id uuid.UUID) (*entity.PayrollRun, error) {
	return s.transition(ctx, businessID, id, entity.PayrollStatusFinalized)
}

// MarkPaid records that a finalised run has actually been disbursed.
func (s *payrollService) MarkPaid(ctx context.Context, businessID, id uuid.UUID) (*entity.PayrollRun, error) {
	return s.transition(ctx, businessID, id, entity.PayrollStatusPaid)
}

func (s *payrollService) transition(
	ctx context.Context, businessID, id uuid.UUID, status string,
) (*entity.PayrollRun, error) {
	run, err := s.payroll.FindRunByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "payroll run not found", "failed to load payroll run")
	}

	now := time.Now()
	switch status {
	case entity.PayrollStatusFinalized:
		if run.Status != entity.PayrollStatusDraft {
			return nil, apperror.New(apperror.CodeConflict, "only a draft payroll run can be finalised")
		}
		if len(run.Payslips) == 0 {
			return nil, apperror.New(apperror.CodeValidationError, "this payroll run has no payslips")
		}
		run.FinalizedAt = &now
	case entity.PayrollStatusPaid:
		if run.Status != entity.PayrollStatusFinalized {
			return nil, apperror.New(apperror.CodeConflict, "only a finalised payroll run can be marked paid")
		}
		run.PaidAt = &now
	}
	run.Status = status

	err = s.tx.WithTransaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
		if updateErr := s.payroll.UpdateRunStatus(ctx, businessID, id, run); updateErr != nil {
			return mapNotFound(updateErr, "payroll run not found", "failed to update the payroll run")
		}
		// The payslips move with the run, so finalising cannot leave individual
		// slips editable behind a frozen header.
		if statusErr := s.payroll.SetPayslipStatusForRun(ctx, tx, id, status); statusErr != nil {
			return apperror.Wrap(apperror.CodeDatabase, "failed to update payslips", statusErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetRun(ctx, businessID, id)
}

/*
DeleteRun removes a draft. A finalised run is kept: it is the record of what was
paid, and deleting it would take the payslips with it.
*/
func (s *payrollService) DeleteRun(ctx context.Context, businessID, id uuid.UUID) error {
	run, err := s.payroll.FindRunByID(ctx, businessID, id)
	if err != nil {
		return mapNotFound(err, "payroll run not found", "failed to load payroll run")
	}
	if !run.IsEditable() {
		return apperror.New(apperror.CodeConflict, "a finalised payroll run cannot be deleted")
	}

	if err := s.payroll.DeleteRun(ctx, businessID, id); err != nil {
		return mapNotFound(err, "payroll run not found", "failed to delete the payroll run")
	}
	return nil
}

func (s *payrollService) ListPayslips(
	ctx context.Context, businessID uuid.UUID, query PayslipQuery,
) ([]entity.Payslip, int64, error) {
	rows, total, err := s.payroll.ListPayslips(ctx, businessID, query.toParams())
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "failed to list payslips", err)
	}
	return rows, total, nil
}

func (s *payrollService) GetPayslip(ctx context.Context, businessID, id uuid.UUID) (*entity.Payslip, error) {
	row, err := s.payroll.FindPayslipByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "payslip not found", "failed to load payslip")
	}
	return row, nil
}

/*
AdjustPayslip replaces a draft payslip's ad-hoc lines and re-totals it.

Replacement rather than append, so removing a bonus is expressible; the computed
components (basic, overtime, statutory deductions) are untouched, because those
come from attendance and the rules snapshot rather than from a form.
*/
func (s *payrollService) AdjustPayslip(
	ctx context.Context, businessID, id uuid.UUID, adjustment PayslipAdjustment,
) (*entity.Payslip, error) {
	payslip, err := s.payroll.FindPayslipByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "payslip not found", "failed to load payslip")
	}
	if payslip.Status != entity.PayrollStatusDraft {
		return nil, apperror.New(apperror.CodeConflict, "a finalised payslip cannot be adjusted")
	}

	var earnings, deductions int64
	for _, item := range adjustment.Items {
		if item.Kind == entity.PayslipItemEarning {
			earnings += item.AmountCents
		} else {
			deductions += item.AmountCents
		}
	}

	// Ad-hoc earnings all land in bonus: allowances are a payroll-rules concept
	// this module does not model separately yet, and splitting them here would
	// invent a distinction the payslip cannot explain.
	payslip.BonusCents = earnings
	payslip.OtherDeductionCents = deductions
	retotal(payslip)

	err = s.tx.WithTransaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
		if itemsErr := s.payroll.ReplacePayslipItems(ctx, tx, payslip.ID, adjustment.Items); itemsErr != nil {
			return apperror.Wrap(apperror.CodeDatabase, "failed to save payslip lines", itemsErr)
		}
		if updateErr := s.payroll.UpdatePayslip(ctx, tx, payslip); updateErr != nil {
			return mapNotFound(updateErr, "payslip not found", "failed to update payslip")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetPayslip(ctx, businessID, id)
}

/*
computePayslip is the salary calculation.

	daily rate    = basic / working days per month
	hourly rate   = basic / (working days per month * working hours per day)
	overtime      = hourly rate / 60 * overtime minutes * multiplier
	absence       = daily rate * (unexcused absences + unpaid leave days)
	gross         = basic + overtime + bonus + allowances
	statutory     = EPF employee share + ETF + tax, all on the basic salary
	net           = gross - absence - statutory - other deductions

EPF and ETF are computed on the basic salary rather than on gross, which is how
they are assessed: overtime and a festival bonus are not pensionable pay. The
employer's EPF share and the ETF contribution are recorded on the payslip for
the employer's own returns — neither is deducted from the employee, so neither
appears in the deduction total.

Net is floored at zero. A negative wage is a data-entry error (a deduction
larger than the salary), and paying it out as a negative number would propagate
the mistake into every report downstream.
*/
func computePayslip(
	employee entity.Employee,
	run *entity.PayrollRun,
	attendance repository.AttendanceSummary,
	leave repository.LeaveDaysByPayType,
	rules PayrollRules,
) entity.Payslip {
	basic := employee.BasicSalaryCents
	dailyRate := float64(basic) / rules.WorkingDaysPerMonth

	payslip := entity.Payslip{
		PayrollRunID:     run.ID,
		BusinessID:       run.BusinessID,
		EmployeeID:       employee.ID,
		PeriodYear:       run.PeriodYear,
		PeriodMonth:      run.PeriodMonth,
		BasicSalaryCents: basic,
		PresentDays:      roundDays(attendance.PresentDays),
		AbsentDays:       roundDays(attendance.AbsentDays),
		PaidLeaveDays:    roundDays(leave.PaidDays),
		UnpaidLeaveDays:  roundDays(leave.UnpaidDays),
		OvertimeMinutes:  int(attendance.OvertimeMinutes),
		Status:           entity.PayrollStatusDraft,
	}

	if rules.OvertimeMultiplier > 0 && rules.WorkingHoursPerDay > 0 && attendance.OvertimeMinutes > 0 {
		hourlyRate := float64(basic) / (rules.WorkingDaysPerMonth * rules.WorkingHoursPerDay)
		payslip.OvertimeCents = roundCents(
			hourlyRate / 60 * float64(attendance.OvertimeMinutes) * rules.OvertimeMultiplier,
		)
	}

	var unpaidDays float64
	if rules.DeductAbsentDays {
		unpaidDays += attendance.AbsentDays
	}
	if rules.DeductUnpaidLeave {
		unpaidDays += leave.UnpaidDays
	}
	payslip.AbsenceDeductionCents = roundCents(dailyRate * unpaidDays)

	// Payable days is what the employee is actually being paid for, reported so
	// a payslip can show the arithmetic behind its absence line.
	payslip.PayableDays = roundDays(math.Max(0, rules.WorkingDaysPerMonth-unpaidDays))

	payslip.EPFEmployeeCents = percentOf(basic, rules.EPFEmployeePercent)
	payslip.EPFEmployerCents = percentOf(basic, rules.EPFEmployerPercent)
	payslip.ETFCents = percentOf(basic, rules.ETFPercent)
	payslip.TaxCents = percentOf(basic, rules.TaxPercent)

	retotal(&payslip)
	return payslip
}

/*
retotal recomputes gross, deductions and net from the components already on the
payslip. Shared by generation and adjustment so a manually-added bonus reaches
the net figure through exactly the same arithmetic as a computed one.
*/
func retotal(p *entity.Payslip) {
	p.GrossCents = p.BasicSalaryCents + p.OvertimeCents + p.BonusCents + p.AllowanceCents
	p.DeductionCents = p.AbsenceDeductionCents + p.EPFEmployeeCents + p.TaxCents + p.OtherDeductionCents

	net := p.GrossCents - p.DeductionCents
	if net < 0 {
		net = 0
	}
	p.NetCents = net
}

/*
percentOf applies a 0-100 percentage to an integer-cent amount, rounding to the
nearest cent. Percentages, not fractions, because a payroll officer enters "8"
for EPF — the conversion happens here and nowhere else.
*/
func percentOf(cents int64, percent float64) int64 {
	if percent <= 0 {
		return 0
	}
	return roundCents(float64(cents) * percent / 100)
}

/*
roundCents converts a computed amount to whole cents, rounding half away from
zero. Called once per payslip line rather than once at the end, so the lines a
payslip prints add up to the total it prints.
*/
func roundCents(v float64) int64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return int64(math.Round(v))
}

/*
monthRange bounds a payroll period. The end is the last day of the month rather
than the first of the next, because the attendance and leave queries filter on
DATE columns with an inclusive upper bound.
*/
func monthRange(year, month int) repository.DateRange {
	from := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	return repository.DateRange{From: from, To: from.AddDate(0, 1, -1)}
}
