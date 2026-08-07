package dto

// LeaveRequestSortFields whitelists the leave list's `sort` values; the first is
// the default.
var LeaveRequestSortFields = []string{"start_date", "created_at", "status", "days"}

/*
LeaveTypeResponse is one entitlement category.

is_system marks a type shared by every business (annual, casual, sick, maternity,
no-pay). Those are read-only: a shop that wants different quotas adds its own
type and deactivates the system one, rather than editing a row every other
tenant also sees.
*/
type LeaveTypeResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
	// Days per calendar year. 0 means the type has no entitlement to draw
	// down — no-pay leave — rather than "none available".
	AnnualQuotaDays float64 `json:"annual_quota_days"`
	IsPaid          bool    `json:"is_paid"`
	IsSystem        bool    `json:"is_system"`
	Status          string  `json:"status"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
}

type LeaveTypeRequest struct {
	Name string `json:"name" validate:"required,min=2,max=80"`
	// Uppercased on save, and unique against the system codes too, so a shop
	// cannot register a second "SICK" that splits the balance in two.
	Code            string  `json:"code" validate:"required,min=2,max=20,alphanum"`
	AnnualQuotaDays float64 `json:"annual_quota_days" validate:"min=0,max=365"`
	IsPaid          bool    `json:"is_paid"`
	Status          string  `json:"status" validate:"omitempty,oneof=active inactive"`
}

/*
LeaveRequestResponse is one application and its decision.

days is what the request actually costs the balance: rostered days off inside
the range are excluded (when the shop's rules say so), and a half day is 0.5. It
is stored at request time rather than recomputed, so a later roster change
cannot rewrite how much leave somebody had asked for.
*/
type LeaveRequestResponse struct {
	ID           string  `json:"id"`
	EmployeeID   string  `json:"employee_id"`
	EmployeeName *string `json:"employee_name,omitempty"`
	EmployeeCode *string `json:"employee_code,omitempty"`

	LeaveTypeID   string  `json:"leave_type_id"`
	LeaveTypeName *string `json:"leave_type_name,omitempty"`
	IsPaid        *bool   `json:"is_paid,omitempty"`

	StartDate string  `json:"start_date"`
	EndDate   string  `json:"end_date"`
	Days      float64 `json:"days"`
	HalfDay   bool    `json:"half_day"`
	Reason    *string `json:"reason,omitempty"`

	Status     string  `json:"status"`
	ReviewedBy *string `json:"reviewed_by,omitempty"`
	ReviewedAt *int64  `json:"reviewed_at,omitempty"`
	ReviewNote *string `json:"review_note,omitempty"`

	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

/*
LeaveApplyRequest files an application.

half_day is only meaningful when the start and end are the same date; asking for
half of a fortnight is not something a leave form can express, and accepting it
would put a number in the balance nobody could explain.
*/
type LeaveApplyRequest struct {
	EmployeeID  string  `json:"employee_id" validate:"required,uuid"`
	LeaveTypeID string  `json:"leave_type_id" validate:"required,uuid"`
	StartDate   string  `json:"start_date" validate:"required,datetime=2006-01-02"`
	EndDate     string  `json:"end_date" validate:"required,datetime=2006-01-02"`
	HalfDay     bool    `json:"half_day"`
	Reason      *string `json:"reason" validate:"omitempty,max=1000"`
}

// LeaveDecisionRequest carries an approver's optional note.
type LeaveDecisionRequest struct {
	Note *string `json:"note" validate:"omitempty,max=1000"`
}

/*
LeaveListQuery is the GET /hrm/leave-requests query string. The date bounds
match a request's start_date.
*/
type LeaveListQuery struct {
	HRMListQuery

	EmployeeID  string `query:"employee_id" validate:"omitempty,uuid"`
	LeaveTypeID string `query:"leave_type_id" validate:"omitempty,uuid"`
	Status      string `query:"status" validate:"omitempty,oneof=pending approved rejected cancelled"`
}

/*
LeaveBalanceResponse is one entitlement line.

remaining is signed: an approver can be allowed to go past the quota
(LeaveRules.AllowNegativeBalance), and reporting that as zero would hide it.
unlimited marks a type with no entitlement to draw down, where a balance of zero
would read as "none left" rather than "not applicable".
*/
type LeaveBalanceResponse struct {
	LeaveTypeID   string  `json:"leave_type_id"`
	LeaveTypeName string  `json:"leave_type_name"`
	Code          string  `json:"code"`
	IsPaid        bool    `json:"is_paid"`
	Entitled      float64 `json:"entitled_days"`
	Taken         float64 `json:"taken_days"`
	Pending       float64 `json:"pending_days"`
	Remaining     float64 `json:"remaining_days"`
	Unlimited     bool    `json:"unlimited"`
}

// LeaveBalanceQuery bounds a balance lookup to one employee and one leave year.
type LeaveBalanceQuery struct {
	EmployeeID string `query:"employee_id" validate:"required,uuid"`
	Year       int    `query:"year" validate:"omitempty,min=2000,max=2200"`
}
