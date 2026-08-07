package mapper

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/dto"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
)

/*
The HRM wire format carries three units the rest of the API does not: clock
times as "HH:mm", calendar dates as yyyy-mm-dd, and weekday lists as JSONB. Each
is converted in both directions, so each is round-tripped here — a silent
disagreement between the two halves would show up as a shift that starts at
midnight or a joining date in the year 1.
*/

func TestClockRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		clock string
	}{
		{"start of day", "00:00"},
		{"morning", "08:30"},
		{"afternoon", "17:45"},
		{"last minute of the day", "23:59"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatClock(parseClock(tt.clock)); got != tt.clock {
				t.Errorf("round trip of %q gave %q", tt.clock, got)
			}
		})
	}
}

func TestParseClockRejectsGarbageAsMidnight(t *testing.T) {
	// The value is validated as datetime=15:04 before a mapper sees it, so an
	// unparseable string means the field was absent rather than wrong.
	if got := parseClock("not a time"); got != 0 {
		t.Errorf("parseClock(garbage) = %v, want 0", got)
	}
}

func TestISODateRoundTrip(t *testing.T) {
	const day = "2026-03-02"

	parsed := parseISODateValue(day)
	if got := isoDate(parsed); got != day {
		t.Errorf("round trip of %q gave %q", day, got)
	}
}

func TestISODateOfAZeroTimeIsEmpty(t *testing.T) {
	// "0001-01-01" would read as a real date on the client.
	if got := isoDate(time.Time{}); got != "" {
		t.Errorf("isoDate(zero) = %q, want an empty string", got)
	}
}

func TestWeeklyOffRoundTrip(t *testing.T) {
	days := []int{0, 6}

	if got := jsonToIntSlice(intSliceToJSON(days)); len(got) != 2 || got[0] != 0 || got[1] != 6 {
		t.Errorf("round trip of %v gave %v", days, got)
	}
}

func TestWeeklyOffHandlesEmptyAndBrokenValues(t *testing.T) {
	// The column is NOT NULL, so an empty list has to encode as [] rather than
	// nil.
	if got := string(intSliceToJSON(nil)); got != "[]" {
		t.Errorf("intSliceToJSON(nil) = %q, want []", got)
	}
	// A malformed roster must not fail a whole shift list.
	if got := jsonToIntSlice([]byte("{oops")); len(got) != 0 {
		t.Errorf("jsonToIntSlice(garbage) = %v, want an empty slice", got)
	}
}

func TestToShiftResponseIncludesTheScheduledLength(t *testing.T) {
	shift := entity.Shift{
		Name:         "Day",
		StartTime:    datatypes.NewTime(8, 0, 0, 0),
		EndTime:      datatypes.NewTime(17, 0, 0, 0),
		BreakMinutes: 60,
		WeeklyOff:    datatypes.JSON(`[0]`),
		Status:       entity.HRMStatusActive,
	}
	shift.ID = uuid.New()

	got := ToShiftResponse(shift)

	if got.StartTime != "08:00" || got.EndTime != "17:00" {
		t.Errorf("times = %q-%q, want 08:00-17:00", got.StartTime, got.EndTime)
	}
	// Nine hours less the hour's break, computed server-side so the client does
	// not re-implement the midnight arithmetic.
	if got.ScheduledMinutes != 480 {
		t.Errorf("scheduled minutes = %d, want 480", got.ScheduledMinutes)
	}
	if len(got.WeeklyOff) != 1 || got.WeeklyOff[0] != 0 {
		t.Errorf("weekly off = %v, want [0]", got.WeeklyOff)
	}
}

func TestToEmployeeResponseStripsDocumentPayloads(t *testing.T) {
	employee := entity.Employee{
		FullName:      "Nimal Perera",
		NIC:           "941234567V",
		DesignationID: uuid.New(),
		JoiningDate:   time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC),
		Designation:   &entity.Designation{Name: "Cashier"},
		Documents: []entity.EmployeeDocument{
			{Name: "NIC scan", DataURL: "data:image/png;base64,AAAA", SizeBytes: 4},
		},
	}
	employee.ID = uuid.New()

	got := ToEmployeeResponse(employee)

	if got.JoiningDate != "2024-01-15" {
		t.Errorf("joining date = %q, want 2024-01-15", got.JoiningDate)
	}
	if got.DesignationName == nil || *got.DesignationName != "Cashier" {
		t.Errorf("designation name = %v, want Cashier", got.DesignationName)
	}
	if len(got.Documents) != 1 {
		t.Fatalf("documents = %d, want 1", len(got.Documents))
	}
	/*
		The employee payload lists a document's name and size but never its
		bytes: a page of twenty employees carrying base64 attachments would run
		to tens of megabytes. The document endpoints serve the payload.
	*/
	if got.Documents[0].DataURL != "" {
		t.Error("employee response carried a document payload")
	}
	if got.Documents[0].Name != "NIC scan" {
		t.Errorf("document name = %q, want NIC scan", got.Documents[0].Name)
	}
}

func TestApplyEmployeeUpdateClearsOmittedOptionalFields(t *testing.T) {
	bank := "Sampath"
	existing := entity.Employee{
		FullName:       "Nimal Perera",
		BankName:       &bank,
		EmploymentType: entity.EmploymentTypeFullTime,
		Status:         entity.EmployeeStatusActive,
	}

	// A replacement body that omits the bank details: the operator cleared them.
	ApplyEmployeeUpdate(&existing, dto.EmployeeUpdateRequest{
		FullName:      "Nimal Perera",
		NIC:           "941234567V",
		Phone:         "0771234567",
		DesignationID: uuid.NewString(),
		JoiningDate:   "2024-01-15",
	})

	if existing.BankName != nil {
		t.Errorf("bank name = %v, want it cleared", *existing.BankName)
	}
	// The enum columns have CHECK constraints, so an absent value keeps what is
	// stored rather than writing an empty string the database would reject.
	if existing.EmploymentType != entity.EmploymentTypeFullTime {
		t.Errorf("employment type = %q, want it preserved", existing.EmploymentType)
	}
	if existing.Status != entity.EmployeeStatusActive {
		t.Errorf("status = %q, want it preserved", existing.Status)
	}
}
