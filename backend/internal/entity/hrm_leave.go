package entity

import (
	"time"

	"github.com/google/uuid"
)

const (
	LeaveStatusPending   = "pending"
	LeaveStatusApproved  = "approved"
	LeaveStatusRejected  = "rejected"
	LeaveStatusCancelled = "cancelled"
)

// LeaveType is an entitlement category. A row with a nil BusinessID is a system
// type available to every tenant — the same pattern the roles table uses for
// the four global system roles, and for the same reason: it removes the need to
// seed reference data every time a shop registers.
//
// AnnualQuotaDays of 0 means the type has no entitlement to draw down (no-pay
// leave), which the balance calculation reports as unlimited-but-unpaid rather
// than as a balance of zero.
type LeaveType struct {
	IDMixin
	Timestamps
	SoftDelete

	BusinessID      *uuid.UUID `gorm:"column:business_id"`
	Name            string     `gorm:"column:name;not null"`
	Code            string     `gorm:"column:code;not null"`
	AnnualQuotaDays float64    `gorm:"column:annual_quota_days;not null"`
	IsPaid          bool       `gorm:"column:is_paid;not null"`
	IsSystem        bool       `gorm:"column:is_system;not null"`
	Status          string     `gorm:"column:status;not null"`
}

func (LeaveType) TableName() string { return "hrm_leave_types" }

// LeaveRequest is one application, from submission through to a decision.
//
// Days is stored rather than derived from the date range on read. The count
// excludes the employee's weekly-off days as their shift stood when the request
// was made, so recomputing it later — after a roster change — would silently
// rewrite how much leave someone had already taken.
type LeaveRequest struct {
	IDMixin
	Timestamps

	BusinessID  uuid.UUID `gorm:"column:business_id;not null"`
	EmployeeID  uuid.UUID `gorm:"column:employee_id;not null"`
	LeaveTypeID uuid.UUID `gorm:"column:leave_type_id;not null"`

	StartDate time.Time `gorm:"column:start_date;type:date;not null"`
	EndDate   time.Time `gorm:"column:end_date;type:date;not null"`
	Days      float64   `gorm:"column:days;not null"`
	HalfDay   bool      `gorm:"column:half_day;not null"`
	Reason    *string   `gorm:"column:reason"`

	Status      string     `gorm:"column:status;not null"`
	ReviewedBy  *uuid.UUID `gorm:"column:reviewed_by"`
	ReviewedAt  *time.Time `gorm:"column:reviewed_at"`
	ReviewNote  *string    `gorm:"column:review_note"`
	RequestedBy *uuid.UUID `gorm:"column:requested_by"`

	Employee  *Employee  `gorm:"foreignKey:EmployeeID;references:ID"`
	LeaveType *LeaveType `gorm:"foreignKey:LeaveTypeID;references:ID"`
}

func (LeaveRequest) TableName() string { return "hrm_leave_requests" }

// IsDecided reports whether the request has already left the pending state. A
// second decision on a decided request is rejected rather than applied, so an
// approval a manager has already communicated cannot be quietly reversed by a
// stale browser tab.
func (r LeaveRequest) IsDecided() bool { return r.Status != LeaveStatusPending }
