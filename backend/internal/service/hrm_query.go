package service

import (
	"time"

	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/repository"
)

/*
The query types every HRM list read takes.

They exist as service types rather than as the repository's own params because
handlers are forbidden from importing internal/repository (see .golangci.yml's
depguard rules) — the same reason OrderService declares OrderListQuery instead of
taking repository.OrderListParams. The conversions below are the single place
the two shapes meet.

Sort has already been validated against the endpoint's whitelist by the handler
(pagination.Parse), and Search LIKE-escaped by the same call. Nothing downstream
re-checks either; an unvalidated Sort reaching here is SQL injection.
*/

// ListQuery is the paging and sorting every HRM list accepts.
type ListQuery struct {
	Limit  int
	Offset int
	Sort   string
	Order  string // "asc" | "desc"
	Search string
}

func (q ListQuery) toParams() repository.ListParams {
	return repository.ListParams{
		Limit:  q.Limit,
		Offset: q.Offset,
		Sort:   q.Sort,
		Order:  q.Order,
		Search: q.Search,
	}
}

/*
DateWindow bounds a query on a business date. A zero bound is unbounded, so the
same struct expresses "March", "everything since March" and "everything".
*/
type DateWindow struct {
	From time.Time
	To   time.Time
}

func (w DateWindow) toRange() repository.DateRange {
	return repository.DateRange{From: w.From, To: w.To}
}

// DesignationQuery filters the job-title list.
type DesignationQuery struct {
	ListQuery
	Status string
}

// ShiftQuery filters the shift list.
type ShiftQuery struct {
	ListQuery
	Status string
}

// EmployeeQuery filters the staff list.
type EmployeeQuery struct {
	ListQuery

	IDs            []uuid.UUID
	DesignationID  *uuid.UUID
	ShiftID        *uuid.UUID
	BranchID       *uuid.UUID
	EmploymentType string
	Status         string
}

func (q EmployeeQuery) toParams() repository.EmployeeListParams {
	return repository.EmployeeListParams{
		ListParams:     q.ListQuery.toParams(),
		IDs:            q.IDs,
		DesignationID:  q.DesignationID,
		ShiftID:        q.ShiftID,
		BranchID:       q.BranchID,
		EmploymentType: q.EmploymentType,
		Status:         q.Status,
	}
}

// AttendanceQuery filters the daily register.
type AttendanceQuery struct {
	ListQuery

	EmployeeID *uuid.UUID
	Status     string
	Window     DateWindow
}

func (q AttendanceQuery) toParams() repository.AttendanceListParams {
	return repository.AttendanceListParams{
		ListParams: q.ListQuery.toParams(),
		EmployeeID: q.EmployeeID,
		Status:     q.Status,
		Range:      q.Window.toRange(),
	}
}

// LeaveQuery filters the leave history. The window bounds a request's start
// date, so "March" means leave that began in March.
type LeaveQuery struct {
	ListQuery

	EmployeeID  *uuid.UUID
	LeaveTypeID *uuid.UUID
	Status      string
	Window      DateWindow
}

func (q LeaveQuery) toParams() repository.LeaveRequestListParams {
	return repository.LeaveRequestListParams{
		ListParams:  q.ListQuery.toParams(),
		EmployeeID:  q.EmployeeID,
		LeaveTypeID: q.LeaveTypeID,
		Status:      q.Status,
		Range:       q.Window.toRange(),
	}
}

// PayslipQuery filters payslip history.
type PayslipQuery struct {
	ListQuery

	EmployeeID  *uuid.UUID
	PeriodYear  *int
	PeriodMonth *int
	Status      string
}

func (q PayslipQuery) toParams() repository.PayslipListParams {
	return repository.PayslipListParams{
		ListParams:  q.ListQuery.toParams(),
		EmployeeID:  q.EmployeeID,
		PeriodYear:  q.PeriodYear,
		PeriodMonth: q.PeriodMonth,
		Status:      q.Status,
	}
}

// PerformanceQuery filters the evaluation history.
type PerformanceQuery struct {
	ListQuery

	EmployeeID  *uuid.UUID
	PeriodYear  *int
	PeriodMonth *int
}

func (q PerformanceQuery) toParams() repository.PerformanceListParams {
	return repository.PerformanceListParams{
		ListParams:  q.ListQuery.toParams(),
		EmployeeID:  q.EmployeeID,
		PeriodYear:  q.PeriodYear,
		PeriodMonth: q.PeriodMonth,
	}
}

/*
AnnouncementQuery filters the noticeboard. PublishedOn narrows to notices in
force on a day — published, on or before it, and not yet expired — which is the
staff-facing view as opposed to the full board an HR officer edits.
*/
type AnnouncementQuery struct {
	ListQuery

	Status      string
	PublishedOn *time.Time
}

func (q AnnouncementQuery) toParams() repository.AnnouncementListParams {
	return repository.AnnouncementListParams{
		ListParams:  q.ListQuery.toParams(),
		Status:      q.Status,
		PublishedOn: q.PublishedOn,
	}
}

// AuditQuery filters the audit trail. Module is a prefix match on the resource
// type, so "hrm" returns the module's whole trail.
type AuditQuery struct {
	ListQuery

	UserID *uuid.UUID
	Action string
	Module string
	Status string
	Window DateWindow
}

func (q AuditQuery) toParams() repository.AuditLogSearchParams {
	return repository.AuditLogSearchParams{
		ListParams: q.ListQuery.toParams(),
		UserID:     q.UserID,
		Action:     q.Action,
		Module:     q.Module,
		Status:     q.Status,
		Range:      q.Window.toRange(),
	}
}
