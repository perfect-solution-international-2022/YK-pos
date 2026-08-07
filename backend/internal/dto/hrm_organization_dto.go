package dto

/*
HRM domain models follow the same wire conventions as products and orders:
snake_case field names, money in integer cents, timestamps as epoch
milliseconds, and calendar dates as yyyy-mm-dd strings.

Clock times are "HH:mm" — a shift starts at 08:00 wherever the shop is, so they
are neither timestamps nor zoned.
*/

// Sort whitelists. Every list endpoint validates its `sort` query parameter
// against one of these before the value reaches an ORDER BY, which GORM does
// not escape. The first entry is the endpoint's default sort.
var (
	DesignationSortFields = []string{"name", "created_at", "status"}
	ShiftSortFields       = []string{"start_time", "name", "created_at", "status"}
)

type DesignationResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Status      string  `json:"status"`
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
}

/*
DesignationRequest is the body for both POST and PUT.

One shape for both because a designation has three fields and an edit submits
all of them — the replace-versus-patch argument that PUT /products makes at
length does not need restating for a row this small.
*/
type DesignationRequest struct {
	Name        string  `json:"name" validate:"required,min=2,max=80"`
	Description *string `json:"description" validate:"omitempty,max=500"`
	Status      string  `json:"status" validate:"omitempty,oneof=active inactive"`
}

type ShiftResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// "HH:mm", 24-hour. An end at or before the start means the shift crosses
	// midnight.
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	BreakMinutes int    `json:"break_minutes"`
	GraceMinutes int    `json:"grace_minutes"`
	// Weekday numbers 0 (Sunday) - 6 (Saturday), matching Date#getDay().
	WeeklyOff []int  `json:"weekly_off"`
	Status    string `json:"status"`
	// Derived: the paid length of the shift in minutes, start to end less the
	// break. Sent so the client does not have to re-implement the midnight
	// arithmetic to show "8h 30m" next to a roster.
	ScheduledMinutes int   `json:"scheduled_minutes"`
	CreatedAt        int64 `json:"created_at"`
	UpdatedAt        int64 `json:"updated_at"`
}

type ShiftRequest struct {
	Name string `json:"name" validate:"required,min=2,max=80"`
	// datetime=15:04 rejects "8:00" and "25:00" alike, which a bare max=5
	// would let through.
	StartTime    string `json:"start_time" validate:"required,datetime=15:04"`
	EndTime      string `json:"end_time" validate:"required,datetime=15:04"`
	BreakMinutes int    `json:"break_minutes" validate:"min=0,max=480"`
	GraceMinutes int    `json:"grace_minutes" validate:"min=0,max=240"`
	// At most six: a shift with seven days off is not a shift.
	WeeklyOff []int  `json:"weekly_off" validate:"omitempty,max=6,dive,min=0,max=6"`
	Status    string `json:"status" validate:"omitempty,oneof=active inactive"`
}
