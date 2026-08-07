package service

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/repository"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
)

/*
LeaveRequestInput is one application.

HalfDay is only meaningful on a single-day request; asking for half of a
fortnight is not a thing a leave form can express, and accepting it would put a
number in the balance nobody could explain.
*/
type LeaveRequestInput struct {
	EmployeeID  uuid.UUID
	LeaveTypeID uuid.UUID
	StartDate   time.Time
	EndDate     time.Time
	HalfDay     bool
	Reason      *string
	RequestedBy *uuid.UUID
}

/*
LeaveBalance is one entitlement line for one employee in one year.

Remaining can go negative when an approver has deliberately allowed an overshoot
(LeaveRules.AllowNegativeBalance), so it is signed. Unlimited marks a type with
no entitlement to draw down — no-pay leave — where a balance of zero would read
as "none left" rather than "not applicable".
*/
type LeaveBalance struct {
	LeaveType entity.LeaveType
	Entitled  float64
	Taken     float64
	Pending   float64
	Remaining float64
	Unlimited bool
}

/*
LeaveService owns applications, decisions and balances.
*/
type LeaveService interface {
	ListTypes(ctx context.Context, businessID uuid.UUID, includeInactive bool) ([]entity.LeaveType, error)
	CreateType(ctx context.Context, businessID uuid.UUID, t *entity.LeaveType) (*entity.LeaveType, error)
	UpdateType(ctx context.Context, businessID, id uuid.UUID, apply func(*entity.LeaveType)) (*entity.LeaveType, error)
	DeleteType(ctx context.Context, businessID, id uuid.UUID) error

	List(ctx context.Context, businessID uuid.UUID, query LeaveQuery) ([]entity.LeaveRequest, int64, error)
	Get(ctx context.Context, businessID, id uuid.UUID) (*entity.LeaveRequest, error)
	Request(ctx context.Context, businessID uuid.UUID, in LeaveRequestInput) (*entity.LeaveRequest, error)
	Approve(ctx context.Context, businessID, id, reviewerID uuid.UUID, note *string) (*entity.LeaveRequest, error)
	Reject(ctx context.Context, businessID, id, reviewerID uuid.UUID, note *string) (*entity.LeaveRequest, error)
	Cancel(ctx context.Context, businessID, id uuid.UUID) (*entity.LeaveRequest, error)

	Balances(ctx context.Context, businessID, employeeID uuid.UUID, year int) ([]LeaveBalance, error)
}

type leaveService struct {
	types      repository.LeaveTypeRepository
	requests   repository.LeaveRequestRepository
	employees  repository.EmployeeRepository
	shifts     repository.ShiftRepository
	attendance repository.AttendanceRepository
	settings   HRMSettingsService
}

func NewLeaveService(
	types repository.LeaveTypeRepository,
	requests repository.LeaveRequestRepository,
	employees repository.EmployeeRepository,
	shifts repository.ShiftRepository,
	attendance repository.AttendanceRepository,
	settings HRMSettingsService,
) LeaveService {
	return &leaveService{
		types:      types,
		requests:   requests,
		employees:  employees,
		shifts:     shifts,
		attendance: attendance,
		settings:   settings,
	}
}

func (s *leaveService) ListTypes(
	ctx context.Context, businessID uuid.UUID, includeInactive bool,
) ([]entity.LeaveType, error) {
	rows, err := s.types.ListForBusiness(ctx, businessID, includeInactive)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to list leave types", err)
	}
	return rows, nil
}

func (s *leaveService) CreateType(
	ctx context.Context, businessID uuid.UUID, t *entity.LeaveType,
) (*entity.LeaveType, error) {
	t.BusinessID = &businessID
	// A tenant cannot mint system types: those are shared by every business and
	// only a migration adds one.
	t.IsSystem = false
	if t.Status == "" {
		t.Status = entity.HRMStatusActive
	}

	// The code is checked against the system types too, because a shop
	// re-registering "SICK" would leave two indistinguishable entries in every
	// dropdown and split the balance across them.
	if _, err := s.types.FindByCode(ctx, businessID, t.Code); err == nil {
		return nil, apperror.New(apperror.CodeConflict, "a leave type with this code already exists")
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to check existing leave type", err)
	}

	if err := s.types.Create(ctx, t); err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to create leave type", err)
	}
	return t, nil
}

func (s *leaveService) UpdateType(
	ctx context.Context, businessID, id uuid.UUID, apply func(*entity.LeaveType),
) (*entity.LeaveType, error) {
	row, err := s.types.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "leave type not found", "failed to load leave type")
	}
	if row.IsSystem {
		return nil, apperror.New(
			apperror.CodeForbidden,
			"system leave types cannot be edited; add your own type instead",
		)
	}

	apply(row)

	if err := s.types.Update(ctx, businessID, row); err != nil {
		return nil, mapNotFound(err, "leave type not found", "failed to update leave type")
	}
	return row, nil
}

func (s *leaveService) DeleteType(ctx context.Context, businessID, id uuid.UUID) error {
	row, err := s.types.FindByID(ctx, businessID, id)
	if err != nil {
		return mapNotFound(err, "leave type not found", "failed to load leave type")
	}
	if row.IsSystem {
		return apperror.New(
			apperror.CodeForbidden,
			"system leave types cannot be deleted; deactivate your own type instead",
		)
	}

	if err := s.types.Delete(ctx, businessID, id); err != nil {
		return mapNotFound(err, "leave type not found", "failed to delete leave type")
	}
	return nil
}

func (s *leaveService) List(
	ctx context.Context, businessID uuid.UUID, query LeaveQuery,
) ([]entity.LeaveRequest, int64, error) {
	rows, total, err := s.requests.List(ctx, businessID, query.toParams())
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "failed to list leave requests", err)
	}
	return rows, total, nil
}

func (s *leaveService) Get(ctx context.Context, businessID, id uuid.UUID) (*entity.LeaveRequest, error) {
	row, err := s.requests.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "leave request not found", "failed to load leave request")
	}
	return row, nil
}

/*
Request files an application.

The day count is computed and stored now, against the shift the employee holds
today, because that is the roster the days were requested under. Recomputing it
at approval time — after a roster change — would quietly alter how much leave
somebody had asked for.
*/
func (s *leaveService) Request(
	ctx context.Context, businessID uuid.UUID, in LeaveRequestInput,
) (*entity.LeaveRequest, error) {
	start, end := dateOnly(in.StartDate), dateOnly(in.EndDate)
	if end.Before(start) {
		return nil, apperror.New(apperror.CodeValidationError, "the end date is before the start date")
	}
	if in.HalfDay && !start.Equal(end) {
		return nil, apperror.New(apperror.CodeValidationError, "a half day must start and end on the same date")
	}

	employee, err := s.employees.FindByID(ctx, businessID, in.EmployeeID)
	if err != nil {
		return nil, mapNotFound(err, "employee not found", "failed to load employee")
	}
	leaveType, err := s.types.FindByID(ctx, businessID, in.LeaveTypeID)
	if err != nil {
		return nil, mapNotFound(err, "leave type not found", "failed to load leave type")
	}
	if leaveType.Status != entity.HRMStatusActive {
		return nil, apperror.New(apperror.CodeValidationError, "this leave type is no longer active")
	}

	rules, err := s.settings.Get(ctx, businessID)
	if err != nil {
		return nil, err
	}
	if err := s.assertNotice(start, rules.Leave); err != nil {
		return nil, err
	}

	shift, err := s.shiftFor(ctx, businessID, employee)
	if err != nil {
		return nil, err
	}
	days := countLeaveDays(start, end, in.HalfDay, shift, rules.Leave.ExcludeWeeklyOff)
	if days <= 0 {
		return nil, apperror.New(
			apperror.CodeValidationError,
			"every day in this range is already a rostered day off",
		)
	}
	if rules.Leave.MaxConsecutiveDays > 0 && days > float64(rules.Leave.MaxConsecutiveDays) {
		return nil, apperror.New(apperror.CodeValidationError, "this request exceeds the maximum consecutive leave allowed")
	}

	if err := s.assertNoOverlap(ctx, businessID, in.EmployeeID, start, end, nil); err != nil {
		return nil, err
	}
	if err := s.assertBalance(ctx, businessID, in.EmployeeID, *leaveType, start, days, rules.Leave); err != nil {
		return nil, err
	}

	row := &entity.LeaveRequest{
		BusinessID:  businessID,
		EmployeeID:  in.EmployeeID,
		LeaveTypeID: in.LeaveTypeID,
		StartDate:   start,
		EndDate:     end,
		Days:        days,
		HalfDay:     in.HalfDay,
		Reason:      in.Reason,
		Status:      entity.LeaveStatusPending,
		RequestedBy: in.RequestedBy,
	}
	if err := s.requests.Create(ctx, row); err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to create leave request", err)
	}
	return s.Get(ctx, businessID, row.ID)
}

/*
Approve grants the request and marks the affected days on the register.

The balance is rechecked here, not just at request time: several applications
can sit pending at once, and approving all of them in turn is exactly how a
quota gets overspent without anybody deciding to.
*/
func (s *leaveService) Approve(
	ctx context.Context, businessID, id, reviewerID uuid.UUID, note *string,
) (*entity.LeaveRequest, error) {
	row, err := s.decide(ctx, businessID, id, reviewerID, note, entity.LeaveStatusApproved)
	if err != nil {
		return nil, err
	}

	s.markAttendance(ctx, businessID, row)
	return s.Get(ctx, businessID, row.ID)
}

func (s *leaveService) Reject(
	ctx context.Context, businessID, id, reviewerID uuid.UUID, note *string,
) (*entity.LeaveRequest, error) {
	row, err := s.decide(ctx, businessID, id, reviewerID, note, entity.LeaveStatusRejected)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, businessID, row.ID)
}

/*
Cancel withdraws a request that has not been decided yet. An approved request is
not cancellable here: the days are already on the register and may already be
inside a finalised payroll run, so unwinding one is a correction a manager makes
deliberately, not a self-service action.
*/
func (s *leaveService) Cancel(ctx context.Context, businessID, id uuid.UUID) (*entity.LeaveRequest, error) {
	row, err := s.requests.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "leave request not found", "failed to load leave request")
	}
	if row.IsDecided() {
		return nil, apperror.New(apperror.CodeConflict, "this request has already been decided")
	}

	row.Status = entity.LeaveStatusCancelled
	if err := s.requests.Update(ctx, row); err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to cancel leave request", err)
	}
	return s.Get(ctx, businessID, id)
}

/*
Balances reports every active leave type for one employee in one calendar year.

Derived from the approved requests rather than stored in a balances table: a
stored figure would need every write path to keep it in step, and the two would
drift the first time a request was corrected. The query behind it is indexed on
(employee_id, leave_type_id, status).
*/
func (s *leaveService) Balances(
	ctx context.Context, businessID, employeeID uuid.UUID, year int,
) ([]LeaveBalance, error) {
	if _, err := s.employees.FindByID(ctx, businessID, employeeID); err != nil {
		return nil, mapNotFound(err, "employee not found", "failed to load employee")
	}

	types, err := s.types.ListForBusiness(ctx, businessID, false)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to list leave types", err)
	}

	yearRange := calendarYear(year)
	taken, err := s.requests.SumApprovedByType(ctx, businessID, employeeID, yearRange)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to total leave taken", err)
	}
	takenByType := make(map[uuid.UUID]float64, len(taken))
	for _, row := range taken {
		takenByType[row.LeaveTypeID] = row.Days
	}

	pending, _, err := s.requests.List(ctx, businessID, repository.LeaveRequestListParams{
		ListParams: repository.ListParams{Limit: pendingBalanceLimit, Sort: "start_date", Order: "desc"},
		EmployeeID: &employeeID,
		Status:     entity.LeaveStatusPending,
		Range:      yearRange,
	})
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to total pending leave", err)
	}
	pendingByType := make(map[uuid.UUID]float64, len(pending))
	for _, row := range pending {
		pendingByType[row.LeaveTypeID] += row.Days
	}

	balances := make([]LeaveBalance, 0, len(types))
	for _, t := range types {
		balance := LeaveBalance{
			LeaveType: t,
			Entitled:  t.AnnualQuotaDays,
			Taken:     roundDays(takenByType[t.ID]),
			Pending:   roundDays(pendingByType[t.ID]),
			Unlimited: t.AnnualQuotaDays == 0,
		}
		balance.Remaining = roundDays(balance.Entitled - balance.Taken)
		balances = append(balances, balance)
	}
	return balances, nil
}

/*
pendingBalanceLimit caps the pending requests a balance calculation reads. A
year's pending applications for one employee is a handful; the cap exists so a
runaway client cannot turn a balance lookup into an unbounded fetch.
*/
const pendingBalanceLimit = 200

func (s *leaveService) decide(
	ctx context.Context, businessID, id, reviewerID uuid.UUID, note *string, status string,
) (*entity.LeaveRequest, error) {
	row, err := s.requests.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "leave request not found", "failed to load leave request")
	}
	if row.IsDecided() {
		return nil, apperror.New(apperror.CodeConflict, "this request has already been decided")
	}

	if status == entity.LeaveStatusApproved {
		rules, rulesErr := s.settings.Get(ctx, businessID)
		if rulesErr != nil {
			return nil, rulesErr
		}
		leaveType, typeErr := s.types.FindByID(ctx, businessID, row.LeaveTypeID)
		if typeErr != nil {
			return nil, mapNotFound(typeErr, "leave type not found", "failed to load leave type")
		}
		if err := s.assertBalance(
			ctx, businessID, row.EmployeeID, *leaveType, row.StartDate, row.Days, rules.Leave,
		); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	row.Status = status
	row.ReviewedBy = &reviewerID
	row.ReviewedAt = &now
	row.ReviewNote = note

	if err := s.requests.Update(ctx, row); err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to record the decision", err)
	}
	return row, nil
}

/*
markAttendance writes an on-leave row for each approved day that does not
already have one.

Best-effort by design: the decision is already committed, and payroll reads
leave from the requests themselves, so a register row that fails to write is a
cosmetic gap rather than a pay error. Days that already have attendance are left
alone — somebody who worked and then had leave approved over the same date is a
discrepancy for a human to look at, not one for this to overwrite.
*/
func (s *leaveService) markAttendance(ctx context.Context, businessID uuid.UUID, row *entity.LeaveRequest) {
	for day := row.StartDate; !day.After(row.EndDate); day = day.AddDate(0, 0, 1) {
		if _, err := s.attendance.FindByEmployeeDate(ctx, businessID, row.EmployeeID, day); err == nil {
			continue
		} else if !errors.Is(err, repository.ErrNotFound) {
			return
		}

		leaveDay := day
		_ = s.attendance.Create(ctx, &entity.Attendance{
			BusinessID: businessID,
			EmployeeID: row.EmployeeID,
			WorkDate:   leaveDay,
			Status:     entity.AttendanceStatusOnLeave,
			Source:     entity.AttendanceSourceSystem,
		})
	}
}

func (s *leaveService) assertNotice(start time.Time, rules LeaveRules) error {
	if rules.MinNoticeDays <= 0 {
		return nil
	}
	earliest := dateOnly(time.Now()).AddDate(0, 0, rules.MinNoticeDays)
	if start.Before(earliest) {
		return apperror.New(apperror.CodeValidationError, "this request does not give the required notice")
	}
	return nil
}

func (s *leaveService) assertNoOverlap(
	ctx context.Context, businessID, employeeID uuid.UUID, start, end time.Time, excludeID *uuid.UUID,
) error {
	overlapping, err := s.requests.FindOverlapping(ctx, businessID, employeeID, start, end, excludeID)
	if err != nil {
		return apperror.Wrap(apperror.CodeDatabase, "failed to check existing leave", err)
	}
	if len(overlapping) > 0 {
		return apperror.New(apperror.CodeConflict, "this employee already has leave booked in that period")
	}
	return nil
}

/*
assertBalance stops a request drawing more than the annual entitlement.

A quota of zero means the type has no entitlement to exhaust (no-pay leave), so
it is skipped rather than treated as "none available". Approved days only count
against the year the request starts in, matching how Balances reports it.
*/
func (s *leaveService) assertBalance(
	ctx context.Context, businessID, employeeID uuid.UUID, leaveType entity.LeaveType,
	start time.Time, days float64, rules LeaveRules,
) error {
	if leaveType.AnnualQuotaDays <= 0 || rules.AllowNegativeBalance {
		return nil
	}

	taken, err := s.requests.SumApprovedByType(ctx, businessID, employeeID, calendarYear(start.Year()))
	if err != nil {
		return apperror.Wrap(apperror.CodeDatabase, "failed to total leave taken", err)
	}

	var used float64
	for _, row := range taken {
		if row.LeaveTypeID == leaveType.ID {
			used = row.Days
			break
		}
	}
	if used+days > leaveType.AnnualQuotaDays {
		return apperror.New(
			apperror.CodeValidationError,
			"this request exceeds the employee's remaining "+leaveType.Name+" balance",
		)
	}
	return nil
}

func (s *leaveService) shiftFor(
	ctx context.Context, businessID uuid.UUID, employee *entity.Employee,
) (*entity.Shift, error) {
	if employee.ShiftID == nil {
		return nil, nil
	}
	shift, err := s.shifts.FindByID(ctx, businessID, *employee.ShiftID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to load shift", err)
	}
	return shift, nil
}

/*
countLeaveDays counts the working days a request covers.

Rostered days off are excluded when the shop's rules say so, which is what makes
a Friday-to-Monday request over a Sunday off count as three days rather than
four. Without a shift there is nothing to exclude, so every calendar day counts.
*/
func countLeaveDays(start, end time.Time, halfDay bool, shift *entity.Shift, excludeWeeklyOff bool) float64 {
	if halfDay && start.Equal(end) {
		return 0.5
	}

	var days float64
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		if excludeWeeklyOff && shift != nil && shift.IsWeeklyOff(int(day.Weekday())) {
			continue
		}
		days++
	}
	return roundDays(days)
}

/*
calendarYear bounds a leave year. Leave entitlement runs on the calendar, so a
request is counted against the year it starts in — a run of days over New Year
belongs to the year it began, which is also how the balance screen reads.
*/
func calendarYear(year int) repository.DateRange {
	return repository.DateRange{
		From: time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC),
	}
}

/*
roundDays trims floating-point noise off a day count before it is stored or
compared. Days come in halves, so two decimals is more precision than the domain
has; without this, 0.1+0.2 arithmetic can make a balance read 6.999999999.
*/
func roundDays(v float64) float64 {
	return math.Round(v*100) / 100
}
