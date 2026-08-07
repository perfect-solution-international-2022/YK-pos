package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/repository"
)

func TestCountLeaveDays(t *testing.T) {
	// 2026-03-02 is a Monday.
	monday := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	friday := monday.AddDate(0, 0, 4)
	nextMonday := monday.AddDate(0, 0, 7)

	tests := []struct {
		name             string
		start, end       time.Time
		halfDay          bool
		shift            *entity.Shift
		excludeWeeklyOff bool
		want             float64
	}{
		{name: "one day", start: monday, end: monday, want: 1},
		{name: "a working week", start: monday, end: friday, want: 5},
		{
			name: "a half day", start: monday, end: monday, halfDay: true, want: 0.5,
		},
		{
			// Friday to Monday over a Sunday off is three days, not four.
			name: "rostered days off are excluded", start: friday, end: nextMonday,
			shift: dayShift(), excludeWeeklyOff: true, want: 3,
		},
		{
			// The same request without the rule costs the full calendar span.
			name: "days off count when the rule is off", start: friday, end: nextMonday,
			shift: dayShift(), excludeWeeklyOff: false, want: 4,
		},
		{
			// Without a shift there is no roster to exclude anything against.
			name: "no shift means every calendar day counts", start: friday, end: nextMonday,
			excludeWeeklyOff: true, want: 4,
		},
		{
			// A half day flag on a range is ignored: the count is what the range
			// actually covers, and the service rejects the combination anyway.
			name: "half day is ignored on a range", start: monday, end: friday,
			halfDay: true, want: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countLeaveDays(tt.start, tt.end, tt.halfDay, tt.shift, tt.excludeWeeklyOff)
			if got != tt.want {
				t.Errorf("countLeaveDays = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalendarYearBoundsTheLeaveYear(t *testing.T) {
	got := calendarYear(2026)

	if formatted := got.From.Format(time.DateOnly); formatted != "2026-01-01" {
		t.Errorf("year start = %s, want 2026-01-01", formatted)
	}
	if formatted := got.To.Format(time.DateOnly); formatted != "2026-12-31" {
		t.Errorf("year end = %s, want 2026-12-31", formatted)
	}
}

func TestRoundDays(t *testing.T) {
	// 0.1 + 0.2 is 0.30000000000000004 in binary floating point; a balance must
	// not report that back to an employee.
	if got := roundDays(0.1 + 0.2); got != 0.3 {
		t.Errorf("roundDays(0.1+0.2) = %v, want 0.3", got)
	}
	if got := roundDays(6.999999999); got != 7 {
		t.Errorf("roundDays(6.999999999) = %v, want 7", got)
	}
}

/*
stubSettingRepository is an in-memory SettingRepository. Hand-written rather than
generated: the interface has three methods, and the settings service's whole job
is what it does with what comes back from them.
*/
type stubSettingRepository struct {
	rows map[string]datatypes.JSON
	err  error
}

func newStubSettingRepository() *stubSettingRepository {
	return &stubSettingRepository{rows: map[string]datatypes.JSON{}}
}

func (s *stubSettingRepository) Get(_ context.Context, _ uuid.UUID, key string) (*entity.Setting, error) {
	if s.err != nil {
		return nil, s.err
	}
	value, ok := s.rows[key]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &entity.Setting{Key: key, Value: value}, nil
}

func (s *stubSettingRepository) GetMany(_ context.Context, _ uuid.UUID, keys []string) ([]entity.Setting, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]entity.Setting, 0, len(keys))
	for _, key := range keys {
		if value, ok := s.rows[key]; ok {
			out = append(out, entity.Setting{Key: key, Value: value})
		}
	}
	return out, nil
}

func (s *stubSettingRepository) Put(_ context.Context, _ uuid.UUID, key string, value datatypes.JSON) error {
	if s.err != nil {
		return s.err
	}
	s.rows[key] = value
	return nil
}

func TestHRMSettingsFallBackToDefaults(t *testing.T) {
	svc := NewHRMSettingsService(newStubSettingRepository())

	got, err := svc.Get(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("Get returned %v", err)
	}

	want := DefaultHRMSettings()
	if got != want {
		t.Errorf("settings = %+v, want the defaults %+v", got, want)
	}
	// The divisor matters most: a zero here would divide by zero in every
	// payroll run the shop ever does.
	if got.Payroll.WorkingDaysPerMonth <= 0 {
		t.Error("default working days per month must be positive")
	}
}

func TestHRMSettingsMergeStoredValuesOntoDefaults(t *testing.T) {
	repo := newStubSettingRepository()
	// A document written before a field existed: only one key is present.
	repo.rows[SettingKeyHRMPayroll] = datatypes.JSON(`{"epf_employee_percent": 10}`)
	svc := NewHRMSettingsService(repo)

	got, err := svc.Get(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("Get returned %v", err)
	}

	if got.Payroll.EPFEmployeePercent != 10 {
		t.Errorf("EPF employee percent = %v, want the stored 10", got.Payroll.EPFEmployeePercent)
	}
	// Everything the document did not mention keeps its default rather than
	// taking a zero.
	if got.Payroll.WorkingDaysPerMonth != DefaultHRMSettings().Payroll.WorkingDaysPerMonth {
		t.Errorf("working days = %v, want the default", got.Payroll.WorkingDaysPerMonth)
	}
}

func TestHRMSettingsSurviveACorruptedDocument(t *testing.T) {
	repo := newStubSettingRepository()
	repo.rows[SettingKeyHRMLeave] = datatypes.JSON(`{ not json at all`)
	svc := NewHRMSettingsService(repo)

	got, err := svc.Get(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("one bad settings row failed the whole read: %v", err)
	}
	if got.Leave != DefaultHRMSettings().Leave {
		t.Errorf("leave rules = %+v, want the defaults", got.Leave)
	}
}

func TestHRMSettingsUpdateWritesAllThreeGroups(t *testing.T) {
	repo := newStubSettingRepository()
	svc := NewHRMSettingsService(repo)

	settings := DefaultHRMSettings()
	settings.Payroll.OvertimeMultiplier = 2
	settings.Leave.MinNoticeDays = 3

	if _, err := svc.Update(context.Background(), uuid.New(), settings); err != nil {
		t.Fatalf("Update returned %v", err)
	}

	for _, key := range []string{SettingKeyHRMAttendance, SettingKeyHRMLeave, SettingKeyHRMPayroll} {
		if _, ok := repo.rows[key]; !ok {
			t.Errorf("settings group %q was not written", key)
		}
	}

	var payroll PayrollRules
	if err := json.Unmarshal(repo.rows[SettingKeyHRMPayroll], &payroll); err != nil {
		t.Fatalf("stored payroll rules do not decode: %v", err)
	}
	if payroll.OvertimeMultiplier != 2 {
		t.Errorf("stored overtime multiplier = %v, want 2", payroll.OvertimeMultiplier)
	}
}
