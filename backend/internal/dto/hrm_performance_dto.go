package dto

// Sort whitelists; the first entry is the default.
var (
	PerformanceSortFields  = []string{"period_year", "created_at", "rating"}
	AnnouncementSortFields = []string{"publish_date", "created_at", "status", "title"}
	AuditLogSortFields     = []string{"created_at", "action"}
)

/*
PerformanceReviewResponse is one monthly evaluation.

Ratings are 1.00-5.00 with halves allowed. They are rates, not currency, so they
are plain numbers rather than integer cents.
*/
type PerformanceReviewResponse struct {
	ID           string  `json:"id"`
	EmployeeID   string  `json:"employee_id"`
	EmployeeName *string `json:"employee_name,omitempty"`
	EmployeeCode *string `json:"employee_code,omitempty"`

	PeriodYear  int `json:"period_year"`
	PeriodMonth int `json:"period_month"`

	Rating       float64  `json:"rating"`
	Punctuality  *float64 `json:"punctuality,omitempty"`
	Teamwork     *float64 `json:"teamwork,omitempty"`
	Productivity *float64 `json:"productivity,omitempty"`

	ManagerNotes *string `json:"manager_notes,omitempty"`
	ReviewerID   *string `json:"reviewer_id,omitempty"`

	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

/*
PerformanceReviewCreateRequest files an evaluation.

One per employee per month: posting a second for a period already reviewed is a
conflict, not a new row, or an average rating would depend on how many times the
form was saved.
*/
type PerformanceReviewCreateRequest struct {
	EmployeeID  string `json:"employee_id" validate:"required,uuid"`
	PeriodYear  int    `json:"period_year" validate:"required,min=2000,max=2200"`
	PeriodMonth int    `json:"period_month" validate:"required,min=1,max=12"`

	Rating       float64  `json:"rating" validate:"required,min=1,max=5"`
	Punctuality  *float64 `json:"punctuality" validate:"omitempty,min=1,max=5"`
	Teamwork     *float64 `json:"teamwork" validate:"omitempty,min=1,max=5"`
	Productivity *float64 `json:"productivity" validate:"omitempty,min=1,max=5"`

	ManagerNotes *string `json:"manager_notes" validate:"omitempty,max=2000"`
}

/*
PerformanceReviewUpdateRequest edits the scores and notes. The employee and the
period are not editable: moving a review to another month is filing a different
review, and would collide with whatever already sits there.
*/
type PerformanceReviewUpdateRequest struct {
	Rating       float64  `json:"rating" validate:"required,min=1,max=5"`
	Punctuality  *float64 `json:"punctuality" validate:"omitempty,min=1,max=5"`
	Teamwork     *float64 `json:"teamwork" validate:"omitempty,min=1,max=5"`
	Productivity *float64 `json:"productivity" validate:"omitempty,min=1,max=5"`
	ManagerNotes *string  `json:"manager_notes" validate:"omitempty,max=2000"`
}

// PerformanceListQuery is the GET /hrm/performance query string.
type PerformanceListQuery struct {
	PaginationQuery

	EmployeeID string `query:"employee_id" validate:"omitempty,uuid"`
	Year       int    `query:"period_year" validate:"omitempty,min=2000,max=2200"`
	Month      int    `query:"period_month" validate:"omitempty,min=1,max=12"`
}

/*
AnnouncementResponse is one staff notice. publish_date is a calendar day, not a
timestamp — "from Monday" is what a noticeboard means.
*/
type AnnouncementResponse struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	PublishDate string  `json:"publish_date"`
	ExpiresOn   *string `json:"expires_on,omitempty"`
	Status      string  `json:"status"`
	CreatedBy   *string `json:"created_by,omitempty"`
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
}

type AnnouncementRequest struct {
	Title       string  `json:"title" validate:"required,min=2,max=200"`
	Description string  `json:"description" validate:"required,min=2,max=5000"`
	PublishDate string  `json:"publish_date" validate:"required,datetime=2006-01-02"`
	ExpiresOn   *string `json:"expires_on" validate:"omitempty,datetime=2006-01-02"`
	Status      string  `json:"status" validate:"omitempty,oneof=draft published archived"`
}

/*
AnnouncementListQuery is the GET /hrm/announcements query string.

current=true narrows to notices in force today — published, on or before today,
and not yet expired — which is what a staff-facing list asks for, as opposed to
the full noticeboard an HR officer edits.
*/
type AnnouncementListQuery struct {
	PaginationQuery

	Status  string `query:"status" validate:"omitempty,oneof=draft published archived"`
	Current bool   `query:"current"`
}

/*
AuditLogResponse is one entry of the trail.

The five things an auditor asks for — who, what, which module, when, from
where — plus the before/after values when the action recorded them.
*/
type AuditLogResponse struct {
	ID     string  `json:"id"`
	UserID *string `json:"user_id,omitempty"`
	Action string  `json:"action"`
	Module *string `json:"module,omitempty"`
	Status string  `json:"status"`

	ResourceID *string        `json:"resource_id,omitempty"`
	OldValues  map[string]any `json:"old_values,omitempty"`
	NewValues  map[string]any `json:"new_values,omitempty"`

	IPAddress *string `json:"ip_address,omitempty"`
	UserAgent *string `json:"user_agent,omitempty"`
	RequestID *string `json:"request_id,omitempty"`

	CreatedAt int64 `json:"created_at"`
}

/*
AuditLogListQuery is the GET /hrm/audit-logs query string.

module is a prefix match on the resource type, so "hrm" returns the whole
module's trail without the caller enumerating its resources.
*/
type AuditLogListQuery struct {
	HRMListQuery

	UserID string `query:"user_id" validate:"omitempty,uuid"`
	Action string `query:"action" validate:"omitempty,max=80"`
	Module string `query:"module" validate:"omitempty,max=80"`
	Status string `query:"status" validate:"omitempty,oneof=success failure"`
}
