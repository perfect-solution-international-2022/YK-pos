package service

import (
	"testing"
	"time"

	"gorm.io/datatypes"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
)

/*
The attendance arithmetic decides what payroll pays for, so it is tested
directly. Reaching it through AttendanceService would need four repositories and
say nothing more about the maths.
*/

// dayShift is 08:00-17:00 with an hour's break and ten minutes' grace: eight
// paid hours, Sunday off.
func dayShift() *entity.Shift {
	return &entity.Shift{
		StartTime:    datatypes.NewTime(8, 0, 0, 0),
		EndTime:      datatypes.NewTime(17, 0, 0, 0),
		BreakMinutes: 60,
		GraceMinutes: 10,
		WeeklyOff:    datatypes.JSON(`[0]`),
	}
}

// nightShift is 22:00-06:00 with no break: it crosses midnight.
func nightShift() *entity.Shift {
	return &entity.Shift{
		StartTime: datatypes.NewTime(22, 0, 0, 0),
		EndTime:   datatypes.NewTime(6, 0, 0, 0),
	}
}

func at(day time.Time, hour, minute int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, time.UTC)
}

func TestLateMinutes(t *testing.T) {
	// A Monday, so the day shift's Sunday off does not apply.
	monday := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	sunday := monday.AddDate(0, 0, -1)

	tests := []struct {
		name     string
		shift    *entity.Shift
		workDate time.Time
		arrival  time.Time
		want     int
	}{
		{"on time", dayShift(), monday, at(monday, 8, 0), 0},
		{"early", dayShift(), monday, at(monday, 7, 45), 0},
		{"inside the grace period", dayShift(), monday, at(monday, 8, 10), 0},
		{"one minute past grace", dayShift(), monday, at(monday, 8, 11), 1},
		{"an hour late", dayShift(), monday, at(monday, 9, 10), 60},
		{
			// Nobody is late on their rostered day off: the whole shift becomes
			// overtime instead.
			name: "no lateness on a weekly off day", shift: dayShift(), workDate: sunday,
			arrival: at(sunday, 11, 0), want: 0,
		},
		{
			// An employee with no shift has no schedule to be late against.
			name: "no shift means no lateness", shift: nil, workDate: monday,
			arrival: at(monday, 14, 0), want: 0,
		},
		{"night shift on time", nightShift(), monday, at(monday, 22, 0), 0},
		{
			/*
				A punch after midnight reads as a tiny arrival minute against a
				22:00 start. Rolling it forward a day is what keeps this from
				reporting the employee as twenty-two hours early.
			*/
			name: "night shift punch after midnight", shift: nightShift(), workDate: monday,
			arrival: at(monday.AddDate(0, 0, 1), 0, 30), want: 150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lateMinutes(tt.shift, tt.workDate, tt.arrival); got != tt.want {
				t.Errorf("lateMinutes = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestApplyWorkedTime(t *testing.T) {
	monday := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	sunday := monday.AddDate(0, 0, -1)
	rules := DefaultHRMSettings().Attendance

	tests := []struct {
		name         string
		shift        *entity.Shift
		workDate     time.Time
		in, out      time.Time
		late         int
		wantWorked   int
		wantOvertime int
		wantEarly    int
		wantStatus   string
	}{
		{
			// 08:00-17:00 is nine hours, less the hour's break: eight worked.
			name: "a full day", shift: dayShift(), workDate: monday,
			in: at(monday, 8, 0), out: at(monday, 17, 0),
			wantWorked: 480, wantStatus: entity.AttendanceStatusPresent,
		},
		{
			name:  "late arrival keeps the late status even after a full day",
			shift: dayShift(), workDate: monday,
			in: at(monday, 9, 0), out: at(monday, 18, 0), late: 50,
			wantWorked: 480, wantStatus: entity.AttendanceStatusLate,
		},
		{
			// Five hours worked is over the 240-minute half-day floor and under
			// the 480-minute full day.
			name: "a short day is a half day", shift: dayShift(), workDate: monday,
			in: at(monday, 8, 0), out: at(monday, 14, 0),
			wantWorked: 300, wantEarly: 180, wantStatus: entity.AttendanceStatusHalfDay,
		},
		{
			name: "under the half-day floor is an absence", shift: dayShift(), workDate: monday,
			in: at(monday, 8, 0), out: at(monday, 11, 0),
			wantWorked: 120, wantEarly: 360, wantStatus: entity.AttendanceStatusAbsent,
		},
		{
			name: "staying late earns overtime", shift: dayShift(), workDate: monday,
			in: at(monday, 8, 0), out: at(monday, 19, 0),
			wantWorked: 600, wantOvertime: 120, wantStatus: entity.AttendanceStatusPresent,
		},
		{
			// Five minutes is under MinOvertimeMinutes: cashing up must not put
			// a payroll line on every day of the month.
			name: "trivial overtime is ignored", shift: dayShift(), workDate: monday,
			in: at(monday, 8, 0), out: at(monday, 17, 5),
			wantWorked: 485, wantOvertime: 0, wantStatus: entity.AttendanceStatusPresent,
		},
		{
			/*
				On a rostered day off the shift schedules nothing, so every
				worked minute is overtime — which is what coming in on your day
				off actually is.
			*/
			name: "a whole day off is overtime", shift: dayShift(), workDate: sunday,
			in: at(sunday, 9, 0), out: at(sunday, 14, 0),
			wantWorked: 240, wantOvertime: 240, wantStatus: entity.AttendanceStatusHalfDay,
		},
		{
			// With no shift there is no break to subtract and nothing to call
			// overtime against.
			name: "no shift means raw worked time", shift: nil, workDate: monday,
			in: at(monday, 9, 0), out: at(monday, 17, 0),
			wantWorked: 480, wantOvertime: 480, wantStatus: entity.AttendanceStatusPresent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := &entity.Attendance{
				WorkDate:    tt.workDate,
				ClockInAt:   &tt.in,
				ClockOutAt:  &tt.out,
				LateMinutes: tt.late,
			}

			applyWorkedTime(row, tt.shift, rules)

			if row.WorkedMinutes != tt.wantWorked {
				t.Errorf("worked = %d, want %d", row.WorkedMinutes, tt.wantWorked)
			}
			if row.OvertimeMinutes != tt.wantOvertime {
				t.Errorf("overtime = %d, want %d", row.OvertimeMinutes, tt.wantOvertime)
			}
			if row.EarlyLeaveMinutes != tt.wantEarly {
				t.Errorf("early leave = %d, want %d", row.EarlyLeaveMinutes, tt.wantEarly)
			}
			if row.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", row.Status, tt.wantStatus)
			}
		})
	}
}

func TestApplyWorkedTimeIgnoresAnOpenDay(t *testing.T) {
	monday := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	in := at(monday, 8, 0)
	row := &entity.Attendance{WorkDate: monday, ClockInAt: &in, Status: entity.AttendanceStatusPresent}

	applyWorkedTime(row, dayShift(), DefaultHRMSettings().Attendance)

	// Somebody still on shift has not worked a short day; the row stays as the
	// clock-in left it until they punch out.
	if row.WorkedMinutes != 0 || row.Status != entity.AttendanceStatusPresent {
		t.Errorf("open day was scored: worked=%d status=%q", row.WorkedMinutes, row.Status)
	}
}

func TestDateOnlyTruncatesToTheCalendarDay(t *testing.T) {
	got := dateOnly(time.Date(2026, time.March, 2, 23, 45, 12, 0, time.UTC))
	want := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)

	if !got.Equal(want) {
		t.Errorf("dateOnly = %v, want %v", got, want)
	}
}
