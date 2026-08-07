package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/repository"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
)

/*
ManualAttendanceInput is a supervisor's correction or an entry for somebody who
does not punch a clock at all.

ClockIn/ClockOut are optional: a day marked absent or on leave has no times, and
insisting on them would make the common case the awkward one. Status is optional
too — left empty, it is derived from the times exactly as a punched day is.
*/
type ManualAttendanceInput struct {
	EmployeeID uuid.UUID
	WorkDate   time.Time
	ClockIn    *time.Time
	ClockOut   *time.Time
	Status     string
	Note       *string
	RecordedBy *uuid.UUID
}

/*
AttendanceService owns the daily register.

The minute arithmetic lives here rather than in the repository or the handler
because it is the module's only real domain calculation, and payroll depends on
it being applied identically whether a day was punched or typed in.
*/
type AttendanceService interface {
	List(ctx context.Context, businessID uuid.UUID, query AttendanceQuery) ([]entity.Attendance, int64, error)
	Get(ctx context.Context, businessID, id uuid.UUID) (*entity.Attendance, error)

	ClockIn(ctx context.Context, businessID, employeeID uuid.UUID, at time.Time, recordedBy *uuid.UUID) (*entity.Attendance, error)
	ClockOut(ctx context.Context, businessID, employeeID uuid.UUID, at time.Time, recordedBy *uuid.UUID) (*entity.Attendance, error)
	Record(ctx context.Context, businessID uuid.UUID, in ManualAttendanceInput) (*entity.Attendance, error)
	Delete(ctx context.Context, businessID, id uuid.UUID) error

	// Summary aggregates a window, per employee, with the employee rows
	// attached so a caller can label them. An employeeID of uuid.Nil covers
	// everyone, which is what the monthly report asks for.
	Summary(ctx context.Context, businessID uuid.UUID, employeeID uuid.UUID, window DateWindow) (AttendanceReport, error)
}

type attendanceService struct {
	attendance repository.AttendanceRepository
	employees  repository.EmployeeRepository
	shifts     repository.ShiftRepository
	settings   HRMSettingsService
}

func NewAttendanceService(
	attendance repository.AttendanceRepository,
	employees repository.EmployeeRepository,
	shifts repository.ShiftRepository,
	settings HRMSettingsService,
) AttendanceService {
	return &attendanceService{
		attendance: attendance,
		employees:  employees,
		shifts:     shifts,
		settings:   settings,
	}
}

func (s *attendanceService) List(
	ctx context.Context, businessID uuid.UUID, query AttendanceQuery,
) ([]entity.Attendance, int64, error) {
	rows, total, err := s.attendance.List(ctx, businessID, query.toParams())
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "failed to list attendance", err)
	}
	return rows, total, nil
}

func (s *attendanceService) Get(ctx context.Context, businessID, id uuid.UUID) (*entity.Attendance, error) {
	row, err := s.attendance.FindByID(ctx, businessID, id)
	if err != nil {
		return nil, mapNotFound(err, "attendance record not found", "failed to load attendance record")
	}
	return row, nil
}

/*
ClockIn opens the employee's day.

Idempotency is by the (employee_id, work_date) unique index rather than by a
preceding read alone: a second tap on a slow terminal is a conflict, not a
second half-day. It is reported as one so the till can show "already clocked in
at 08:04" instead of silently overwriting the real arrival time — which would
erase the lateness the row exists to record.
*/
func (s *attendanceService) ClockIn(
	ctx context.Context, businessID, employeeID uuid.UUID, at time.Time, recordedBy *uuid.UUID,
) (*entity.Attendance, error) {
	employee, shift, err := s.resolve(ctx, businessID, employeeID)
	if err != nil {
		return nil, err
	}
	if !employee.IsPayable() {
		return nil, apperror.New(apperror.CodeForbidden, "this employee is not currently active")
	}

	workDate := dateOnly(at)
	existing, err := s.attendance.FindByEmployeeDate(ctx, businessID, employeeID, workDate)
	switch {
	case err == nil && existing.ClockInAt != nil:
		return nil, apperror.New(apperror.CodeConflict, "this employee has already clocked in today")
	case err != nil && !errors.Is(err, repository.ErrNotFound):
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to check attendance", err)
	}

	row := &entity.Attendance{
		BusinessID:  businessID,
		EmployeeID:  employeeID,
		WorkDate:    workDate,
		ClockInAt:   &at,
		Source:      entity.AttendanceSourceClock,
		Status:      entity.AttendanceStatusPresent,
		LateMinutes: lateMinutes(shift, workDate, at),
		RecordedBy:  recordedBy,
	}
	if shift != nil {
		row.ShiftID = &shift.ID
	}
	if row.LateMinutes > 0 {
		row.Status = entity.AttendanceStatusLate
	}

	// A row for a day marked absent or on leave in advance is upgraded rather
	// than duplicated: somebody turning up after all is a correction to that
	// day, and the unique index would reject a second row anyway.
	if existing != nil && err == nil {
		row.ID = existing.ID
		if updateErr := s.attendance.Update(ctx, row); updateErr != nil {
			return nil, apperror.Wrap(apperror.CodeDatabase, "failed to record clock-in", updateErr)
		}
		return s.Get(ctx, businessID, row.ID)
	}

	if createErr := s.attendance.Create(ctx, row); createErr != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to record clock-in", createErr)
	}
	return s.Get(ctx, businessID, row.ID)
}

/*
ClockOut closes the day and computes what it was worth.

The row is looked up by the work date of the clock-in, not of the moment of
clocking out — a night shift punched in at 22:00 and out at 06:00 is one day's
work, and looking up "today" at 06:00 would find nothing and open a second day.
*/
func (s *attendanceService) ClockOut(
	ctx context.Context, businessID, employeeID uuid.UUID, at time.Time, recordedBy *uuid.UUID,
) (*entity.Attendance, error) {
	_, shift, err := s.resolve(ctx, businessID, employeeID)
	if err != nil {
		return nil, err
	}

	row, err := s.findOpenDay(ctx, businessID, employeeID, at)
	if err != nil {
		return nil, err
	}
	if row.ClockInAt == nil {
		return nil, apperror.New(apperror.CodeConflict, "this employee has not clocked in")
	}
	if row.ClockOutAt != nil {
		return nil, apperror.New(apperror.CodeConflict, "this employee has already clocked out")
	}
	if at.Before(*row.ClockInAt) {
		return nil, apperror.New(apperror.CodeValidationError, "the clock-out time is before the clock-in time")
	}

	rules, err := s.settings.Get(ctx, businessID)
	if err != nil {
		return nil, err
	}

	row.ClockOutAt = &at
	if recordedBy != nil {
		row.RecordedBy = recordedBy
	}
	applyWorkedTime(row, shift, rules.Attendance)

	if err := s.attendance.Update(ctx, row); err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to record clock-out", err)
	}
	return s.Get(ctx, businessID, row.ID)
}

/*
Record writes an attendance day by hand, creating or replacing whichever row the
date already holds.

Replacement rather than a patch, for the same reason PUT /products replaces: the
form submits the whole day, and "absent, no times" has to be expressible — which
a sparse update could not distinguish from "left the times alone".
*/
func (s *attendanceService) Record(
	ctx context.Context, businessID uuid.UUID, in ManualAttendanceInput,
) (*entity.Attendance, error) {
	_, shift, err := s.resolve(ctx, businessID, in.EmployeeID)
	if err != nil {
		return nil, err
	}

	rules, err := s.settings.Get(ctx, businessID)
	if err != nil {
		return nil, err
	}

	workDate := dateOnly(in.WorkDate)
	if !rules.Attendance.AllowFutureEntry && workDate.After(dateOnly(time.Now())) {
		return nil, apperror.New(apperror.CodeValidationError, "attendance cannot be recorded for a future date")
	}
	if in.ClockIn != nil && in.ClockOut != nil && in.ClockOut.Before(*in.ClockIn) {
		return nil, apperror.New(apperror.CodeValidationError, "the clock-out time is before the clock-in time")
	}

	row := &entity.Attendance{
		BusinessID: businessID,
		EmployeeID: in.EmployeeID,
		WorkDate:   workDate,
		ClockInAt:  in.ClockIn,
		ClockOutAt: in.ClockOut,
		Status:     in.Status,
		Source:     entity.AttendanceSourceManual,
		Note:       in.Note,
		RecordedBy: in.RecordedBy,
	}
	if shift != nil {
		row.ShiftID = &shift.ID
	}
	if in.ClockIn != nil {
		row.LateMinutes = lateMinutes(shift, workDate, *in.ClockIn)
	}
	if in.ClockIn != nil && in.ClockOut != nil {
		applyWorkedTime(row, shift, rules.Attendance)
	}
	if in.Status != "" {
		// An explicit status wins over the derived one: a supervisor marking a
		// short day as present despite the hours is making a decision, and the
		// arithmetic must not overrule it.
		row.Status = in.Status
	}
	if row.Status == "" {
		row.Status = entity.AttendanceStatusAbsent
	}

	existing, err := s.attendance.FindByEmployeeDate(ctx, businessID, in.EmployeeID, workDate)
	switch {
	case err == nil:
		row.ID = existing.ID
		if updateErr := s.attendance.Update(ctx, row); updateErr != nil {
			return nil, apperror.Wrap(apperror.CodeDatabase, "failed to save attendance", updateErr)
		}
	case errors.Is(err, repository.ErrNotFound):
		if createErr := s.attendance.Create(ctx, row); createErr != nil {
			return nil, apperror.Wrap(apperror.CodeDatabase, "failed to save attendance", createErr)
		}
	default:
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to check attendance", err)
	}

	return s.Get(ctx, businessID, row.ID)
}

func (s *attendanceService) Delete(ctx context.Context, businessID, id uuid.UUID) error {
	if err := s.attendance.Delete(ctx, businessID, id); err != nil {
		return mapNotFound(err, "attendance record not found", "failed to delete attendance record")
	}
	return nil
}

/*
Summary aggregates the register for a window.

The employee rows come from a second query keyed by id rather than from a join:
the aggregate groups by employee_id, and pulling every displayed column into
that GROUP BY would cost more and gain nothing. The lookup is by id, not by
status, so somebody who has since resigned still appears with their name rather
than as a bare uuid.
*/
func (s *attendanceService) Summary(
	ctx context.Context, businessID uuid.UUID, employeeID uuid.UUID, window DateWindow,
) (AttendanceReport, error) {
	var ids []uuid.UUID
	if employeeID != uuid.Nil {
		ids = []uuid.UUID{employeeID}
	}

	rows, err := s.attendance.SummarizeByEmployee(ctx, businessID, window.toRange(), ids)
	if err != nil {
		return AttendanceReport{}, apperror.Wrap(apperror.CodeDatabase, "failed to summarise attendance", err)
	}

	employees, err := s.namesFor(ctx, businessID, rows)
	if err != nil {
		return AttendanceReport{}, err
	}

	return AttendanceReport{Range: window.toRange(), Rows: rows, Employees: employees}, nil
}

// namesFor loads the employees a summary refers to, keyed by id.
func (s *attendanceService) namesFor(
	ctx context.Context, businessID uuid.UUID, rows []repository.AttendanceSummary,
) (map[uuid.UUID]entity.Employee, error) {
	byID := make(map[uuid.UUID]entity.Employee, len(rows))
	if len(rows) == 0 {
		return byID, nil
	}

	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.EmployeeID)
	}

	employees, _, err := s.employees.List(ctx, businessID, repository.EmployeeListParams{
		ListParams: repository.ListParams{Limit: len(ids), Sort: "full_name", Order: "asc"},
		IDs:        ids,
	})
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to load employees", err)
	}

	for _, employee := range employees {
		byID[employee.ID] = employee
	}
	return byID, nil
}

/*
findOpenDay locates the attendance row a clock-out belongs to.

It tries the day of the punch first and falls back to the previous day, because
a night shift's clock-out lands on the following calendar date. The fallback is
only taken when that earlier row is still open, so an ordinary morning punch
after a completed previous day is not mistaken for one.
*/
func (s *attendanceService) findOpenDay(
	ctx context.Context, businessID, employeeID uuid.UUID, at time.Time,
) (*entity.Attendance, error) {
	today := dateOnly(at)
	row, err := s.attendance.FindByEmployeeDate(ctx, businessID, employeeID, today)
	if err == nil && row.ClockOutAt == nil {
		return row, nil
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to load attendance", err)
	}

	previous, prevErr := s.attendance.FindByEmployeeDate(ctx, businessID, employeeID, today.AddDate(0, 0, -1))
	if prevErr == nil && previous.ClockInAt != nil && previous.ClockOutAt == nil {
		return previous, nil
	}
	if prevErr != nil && !errors.Is(prevErr, repository.ErrNotFound) {
		return nil, apperror.Wrap(apperror.CodeDatabase, "failed to load attendance", prevErr)
	}

	if row != nil && err == nil {
		// Today's row exists but is already closed; the caller's conflict
		// checks report that precisely.
		return row, nil
	}
	return nil, apperror.New(apperror.CodeNotFound, "this employee has not clocked in")
}

/*
resolve loads the employee and the shift they are rostered on. A shift is
optional — casual staff may have none — so a nil shift means "no schedule to
measure against", and the arithmetic below treats every worked minute as plain
worked time.
*/
func (s *attendanceService) resolve(
	ctx context.Context, businessID, employeeID uuid.UUID,
) (*entity.Employee, *entity.Shift, error) {
	employee, err := s.employees.FindByID(ctx, businessID, employeeID)
	if err != nil {
		return nil, nil, mapNotFound(err, "employee not found", "failed to load employee")
	}
	if employee.ShiftID == nil {
		return employee, nil, nil
	}

	shift, err := s.shifts.FindByID(ctx, businessID, *employee.ShiftID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return employee, nil, nil
		}
		return nil, nil, apperror.Wrap(apperror.CodeDatabase, "failed to load shift", err)
	}
	return employee, shift, nil
}

/*
applyWorkedTime fills in the derived minute columns and the resulting status.

Worked time is the span between the punches less the shift's unpaid break.
Overtime is measured against the shift's scheduled minutes — which are zero on a
rostered day off, so somebody who comes in on their day off has the whole shift
counted as overtime rather than as a normal day.

An explicitly-set status is not touched here; Record reapplies it afterwards.
*/
func applyWorkedTime(row *entity.Attendance, shift *entity.Shift, rules AttendanceRules) {
	if row.ClockInAt == nil || row.ClockOutAt == nil {
		return
	}

	worked := int(row.ClockOutAt.Sub(*row.ClockInAt).Minutes())
	scheduled := 0
	if shift != nil {
		worked -= shift.BreakMinutes
		if !shift.IsWeeklyOff(int(row.WorkDate.Weekday())) {
			scheduled = shift.ScheduledMinutes()
		}
	}
	if worked < 0 {
		worked = 0
	}
	row.WorkedMinutes = worked

	if scheduled > 0 && worked < scheduled {
		row.EarlyLeaveMinutes = scheduled - worked
	} else {
		row.EarlyLeaveMinutes = 0
	}

	row.OvertimeMinutes = 0
	if rules.OvertimeEnabled && worked > scheduled {
		if overtime := worked - scheduled; overtime >= rules.MinOvertimeMinutes {
			row.OvertimeMinutes = overtime
		}
	}

	switch {
	case worked >= rules.FullDayMinutes:
		row.Status = entity.AttendanceStatusPresent
	case worked >= rules.HalfDayMinutes:
		row.Status = entity.AttendanceStatusHalfDay
	default:
		row.Status = entity.AttendanceStatusAbsent
	}
	// Lateness survives a full day's work: arriving late and staying late is
	// still a late arrival, which is what the punctuality report reads.
	if row.LateMinutes > 0 && row.Status == entity.AttendanceStatusPresent {
		row.Status = entity.AttendanceStatusLate
	}
}

/*
lateMinutes measures arrival against the shift's start plus its grace period.

Shift times are wall-clock with no timezone, so they are compared against the
punch's local time-of-day — the same clock the shop reads off the wall. A shift
that crosses midnight is measured from its start on the work date, which is why
the work date is passed in rather than derived from the punch.
*/
func lateMinutes(shift *entity.Shift, workDate, at time.Time) int {
	if shift == nil {
		return 0
	}
	if shift.IsWeeklyOff(int(workDate.Weekday())) {
		return 0
	}

	arrival := at.Hour()*60 + at.Minute()
	allowed := shift.StartMinutes() + shift.GraceMinutes

	// A punch after midnight on a night shift reads as a very small arrival
	// minute against a very large allowance; rolling it forward a day keeps the
	// comparison meaningful instead of reporting 1200 minutes early.
	if shift.CrossesMidnight() && arrival < shift.StartMinutes() {
		arrival += 24 * 60
	}

	if arrival <= allowed {
		return 0
	}
	return arrival - allowed
}

/*
dateOnly truncates an instant to the calendar day it falls on, in the server's
timezone, which is the wall clock the shop keeps. DATE columns carry no time and
no zone, so every read and write of work_date goes through here to make sure the
two sides agree on where a day starts.
*/
func dateOnly(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
