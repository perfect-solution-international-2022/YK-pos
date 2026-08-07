package service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/repository"
	"github.com/SandaruwanWeerawardhana/pos-backend/pkg/apperror"
)

/*
Reserved settings keys. HRM's three policy groups are JSON documents in the
existing settings table rather than three bespoke tables: they are read whole,
written whole, and never queried by their contents, so a table per group would
buy three migrations and three repositories to store what is always one blob.
*/
const (
	SettingKeyHRMAttendance = "hrm.attendance"
	SettingKeyHRMLeave      = "hrm.leave"
	SettingKeyHRMPayroll    = "hrm.payroll"
)

/*
AttendanceRules governs how a punched day is classified.

Minutes rather than hours throughout, matching the attendance table's stored
columns, so no call site has to convert.
*/
type AttendanceRules struct {
	// Worked minutes at or above which a day counts as a full day. Below
	// HalfDayMinutes the day is absent; between the two it is a half day.
	FullDayMinutes int `json:"full_day_minutes"`
	HalfDayMinutes int `json:"half_day_minutes"`

	// OvertimeEnabled false means minutes beyond the shift are recorded but
	// never paid — some shops roster overtime as time off in lieu instead.
	OvertimeEnabled bool `json:"overtime_enabled"`
	// Overtime shorter than this is ignored, so a cashier who takes four
	// minutes to cash up does not generate a payroll line every day.
	MinOvertimeMinutes int `json:"min_overtime_minutes"`

	// AllowFutureEntry permits recording attendance for a date that has not
	// happened yet. Off by default: it is almost always a typo in the date.
	AllowFutureEntry bool `json:"allow_future_entry"`
}

/*
LeaveRules governs what a request may ask for.
*/
type LeaveRules struct {
	// 0 means no cap.
	MaxConsecutiveDays int `json:"max_consecutive_days"`
	// Days of notice required before the start date. 0 allows same-day
	// applications, which is what sick leave needs.
	MinNoticeDays int `json:"min_notice_days"`
	// AllowNegativeBalance lets an approver go past the annual quota. Off by
	// default — the overshoot is usually a miscount, not a decision.
	AllowNegativeBalance bool `json:"allow_negative_balance"`
	// ExcludeWeeklyOff drops the employee's rostered days off from the day
	// count, so a Friday-to-Monday request over a Sunday off is three days.
	ExcludeWeeklyOff bool `json:"exclude_weekly_off"`
}

/*
PayrollRules governs the salary calculation.

Percentages are 0-100 (8 = 8%), not fractions — unlike a product's tax_rate,
which is a fraction. The difference is deliberate on both sides: a payroll
officer enters "8" for EPF, and the calculation divides by 100 at one place.
*/
type PayrollRules struct {
	// The divisor turning a monthly salary into a daily rate.
	WorkingDaysPerMonth float64 `json:"working_days_per_month"`
	WorkingHoursPerDay  float64 `json:"working_hours_per_day"`
	OvertimeMultiplier  float64 `json:"overtime_multiplier"`

	EPFEmployeePercent float64 `json:"epf_employee_percent"`
	EPFEmployerPercent float64 `json:"epf_employer_percent"`
	ETFPercent         float64 `json:"etf_percent"`
	TaxPercent         float64 `json:"tax_percent"`

	// DeductAbsentDays subtracts a daily rate for each unexcused absence.
	DeductAbsentDays bool `json:"deduct_absent_days"`
	// DeductUnpaidLeave does the same for approved no-pay leave.
	DeductUnpaidLeave bool `json:"deduct_unpaid_leave"`
}

// HRMSettings is the whole policy set, which is how the settings screen reads
// and writes it.
type HRMSettings struct {
	Attendance AttendanceRules `json:"attendance"`
	Leave      LeaveRules      `json:"leave"`
	Payroll    PayrollRules    `json:"payroll"`
}

/*
DefaultHRMSettings is what a business gets before it has saved anything, and the
base every stored document is decoded onto — so a key added to one of these
structs after a shop last saved its settings takes the default rather than a
zero, which for a divisor like WorkingDaysPerMonth would be a division by zero
in the payroll run.

The statutory rates are Sri Lankan (EPF 8/12, ETF 3), matching the currency and
NIC field the rest of the module is built around. A shop elsewhere overwrites
them once on the settings screen.
*/
func DefaultHRMSettings() HRMSettings {
	return HRMSettings{
		Attendance: AttendanceRules{
			FullDayMinutes:     480,
			HalfDayMinutes:     240,
			OvertimeEnabled:    true,
			MinOvertimeMinutes: 15,
			AllowFutureEntry:   false,
		},
		Leave: LeaveRules{
			MaxConsecutiveDays:   30,
			MinNoticeDays:        0,
			AllowNegativeBalance: false,
			ExcludeWeeklyOff:     true,
		},
		Payroll: PayrollRules{
			WorkingDaysPerMonth: 26,
			WorkingHoursPerDay:  8,
			OvertimeMultiplier:  1.5,
			EPFEmployeePercent:  8,
			EPFEmployerPercent:  12,
			ETFPercent:          3,
			TaxPercent:          0,
			DeductAbsentDays:    true,
			DeductUnpaidLeave:   true,
		},
	}
}

/*
HRMSettingsService reads and writes the module's policy.

Every other HRM service depends on this one rather than on the repository, so
there is a single place where "what are this shop's rules" is answered and a
single place where the defaults live.
*/
type HRMSettingsService interface {
	Get(ctx context.Context, businessID uuid.UUID) (HRMSettings, error)
	Update(ctx context.Context, businessID uuid.UUID, settings HRMSettings) (HRMSettings, error)
}

type hrmSettingsService struct {
	settings repository.SettingRepository
}

func NewHRMSettingsService(settings repository.SettingRepository) HRMSettingsService {
	return &hrmSettingsService{settings: settings}
}

func (s *hrmSettingsService) Get(ctx context.Context, businessID uuid.UUID) (HRMSettings, error) {
	rows, err := s.settings.GetMany(ctx, businessID, []string{
		SettingKeyHRMAttendance, SettingKeyHRMLeave, SettingKeyHRMPayroll,
	})
	if err != nil {
		return HRMSettings{}, apperror.Wrap(apperror.CodeDatabase, "failed to load HRM settings", err)
	}

	out := DefaultHRMSettings()
	for _, row := range rows {
		/*
			Decoded onto the defaults, so a document written before a field
			existed keeps that field's default instead of taking a zero. A blob
			that fails to decode is left at the defaults rather than failing the
			request: one corrupted settings row must not take the whole HR
			module offline.
		*/
		switch row.Key {
		case SettingKeyHRMAttendance:
			_ = json.Unmarshal(row.Value, &out.Attendance)
		case SettingKeyHRMLeave:
			_ = json.Unmarshal(row.Value, &out.Leave)
		case SettingKeyHRMPayroll:
			_ = json.Unmarshal(row.Value, &out.Payroll)
		}
	}
	return out, nil
}

/*
Update replaces all three groups. Whole-document writes, matching how the screen
submits them: a partial write would need the same absent-versus-null distinction
that PUT /products avoids for the same reason.
*/
func (s *hrmSettingsService) Update(
	ctx context.Context, businessID uuid.UUID, settings HRMSettings,
) (HRMSettings, error) {
	groups := []struct {
		key   string
		value any
	}{
		{SettingKeyHRMAttendance, settings.Attendance},
		{SettingKeyHRMLeave, settings.Leave},
		{SettingKeyHRMPayroll, settings.Payroll},
	}

	for _, g := range groups {
		raw, err := json.Marshal(g.value)
		if err != nil {
			return HRMSettings{}, apperror.Wrap(apperror.CodeInternal, "failed to encode HRM settings", err)
		}
		if err := s.settings.Put(ctx, businessID, g.key, datatypes.JSON(raw)); err != nil {
			return HRMSettings{}, apperror.Wrap(apperror.CodeDatabase, "failed to save HRM settings", err)
		}
	}

	return settings, nil
}
