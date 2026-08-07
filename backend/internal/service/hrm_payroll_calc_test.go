package service

import (
	"testing"

	"github.com/google/uuid"

	"github.com/SandaruwanWeerawardhana/pos-backend/internal/entity"
	"github.com/SandaruwanWeerawardhana/pos-backend/internal/repository"
)

/*
The salary calculation is tested directly rather than through PayrollService,
because the service needs a TxManager over a live *gorm.DB and the arithmetic is
what actually decides what people are paid. An in-package test is the only way
to reach it without standing up a database for a pure function.
*/

func testPayrollRules() PayrollRules {
	rules := DefaultHRMSettings().Payroll
	// 25 days and 8 hours makes the daily rate a round number, so a wrong
	// answer below is a wrong formula rather than a rounding artefact.
	rules.WorkingDaysPerMonth = 25
	rules.WorkingHoursPerDay = 8
	return rules
}

func testPayrollRun() *entity.PayrollRun {
	run := &entity.PayrollRun{BusinessID: uuid.New(), PeriodYear: 2026, PeriodMonth: 3}
	run.ID = uuid.New()
	return run
}

func testEmployee(salaryCents int64) entity.Employee {
	e := entity.Employee{BasicSalaryCents: salaryCents, Status: entity.EmployeeStatusActive}
	e.ID = uuid.New()
	return e
}

func TestComputePayslip(t *testing.T) {
	run := testPayrollRun()
	rules := testPayrollRules()

	tests := []struct {
		name       string
		salary     int64
		attendance repository.AttendanceSummary
		leave      repository.LeaveDaysByPayType
		mutate     func(*PayrollRules)
		want       entity.Payslip
	}{
		{
			// 100,000.00 basic. EPF employee 8% = 8,000.00, employer 12% =
			// 12,000.00, ETF 3% = 3,000.00. Only the employee share is deducted.
			name:       "full attendance pays the basic less statutory deductions",
			salary:     10_000_000,
			attendance: repository.AttendanceSummary{PresentDays: 25},
			want: entity.Payslip{
				GrossCents:       10_000_000,
				EPFEmployeeCents: 800_000,
				EPFEmployerCents: 1_200_000,
				ETFCents:         300_000,
				DeductionCents:   800_000,
				NetCents:         9_200_000,
				PayableDays:      25,
			},
		},
		{
			// Daily rate is 100,000.00 / 25 = 4,000.00. Two absences cost
			// 8,000.00 on top of the 8% EPF.
			name:       "absences are deducted at the daily rate",
			salary:     10_000_000,
			attendance: repository.AttendanceSummary{PresentDays: 23, AbsentDays: 2},
			want: entity.Payslip{
				GrossCents:            10_000_000,
				AbsenceDeductionCents: 800_000,
				EPFEmployeeCents:      800_000,
				DeductionCents:        1_600_000,
				NetCents:              8_400_000,
				PayableDays:           23,
			},
		},
		{
			// Paid leave costs nothing; unpaid leave costs a daily rate a day.
			name:       "paid leave is free and unpaid leave is deducted",
			salary:     10_000_000,
			attendance: repository.AttendanceSummary{PresentDays: 20},
			leave:      repository.LeaveDaysByPayType{PaidDays: 3, UnpaidDays: 2},
			want: entity.Payslip{
				GrossCents:            10_000_000,
				AbsenceDeductionCents: 800_000,
				EPFEmployeeCents:      800_000,
				DeductionCents:        1_600_000,
				NetCents:              8_400_000,
				PayableDays:           23,
				PaidLeaveDays:         3,
				UnpaidLeaveDays:       2,
			},
		},
		{
			// Hourly rate is 100,000.00 / (25 * 8) = 500.00. Two hours at 1.5x
			// is 1,500.00. Overtime is not pensionable, so EPF stays on basic.
			name:       "overtime pays at the multiplier and is not pensionable",
			salary:     10_000_000,
			attendance: repository.AttendanceSummary{PresentDays: 25, OvertimeMinutes: 120},
			want: entity.Payslip{
				GrossCents:       10_150_000,
				OvertimeCents:    150_000,
				OvertimeMinutes:  120,
				EPFEmployeeCents: 800_000,
				DeductionCents:   800_000,
				NetCents:         9_350_000,
				PayableDays:      25,
			},
		},
		{
			name:       "overtime is recorded but unpaid when the shop has it disabled",
			salary:     10_000_000,
			attendance: repository.AttendanceSummary{PresentDays: 25, OvertimeMinutes: 120},
			mutate:     func(r *PayrollRules) { r.OvertimeMultiplier = 0 },
			want: entity.Payslip{
				GrossCents:       10_000_000,
				OvertimeMinutes:  120,
				EPFEmployeeCents: 800_000,
				DeductionCents:   800_000,
				NetCents:         9_200_000,
				PayableDays:      25,
			},
		},
		{
			/*
				A month of absences would compute a negative wage. Net floors at
				zero: paying out a negative number would propagate a data-entry
				error into every report downstream.
			*/
			name:       "net never goes below zero",
			salary:     10_000_000,
			attendance: repository.AttendanceSummary{AbsentDays: 25},
			want: entity.Payslip{
				GrossCents:            10_000_000,
				AbsenceDeductionCents: 10_000_000,
				EPFEmployeeCents:      800_000,
				DeductionCents:        10_800_000,
				NetCents:              0,
				PayableDays:           0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ruleSet := rules
			if tt.mutate != nil {
				tt.mutate(&ruleSet)
			}

			got := computePayslip(testEmployee(tt.salary), run, tt.attendance, tt.leave, ruleSet)

			assertCents(t, "gross", tt.want.GrossCents, got.GrossCents)
			assertCents(t, "deductions", tt.want.DeductionCents, got.DeductionCents)
			assertCents(t, "net", tt.want.NetCents, got.NetCents)
			assertCents(t, "overtime", tt.want.OvertimeCents, got.OvertimeCents)
			assertCents(t, "absence deduction", tt.want.AbsenceDeductionCents, got.AbsenceDeductionCents)
			assertCents(t, "EPF employee", tt.want.EPFEmployeeCents, got.EPFEmployeeCents)

			if tt.want.EPFEmployerCents != 0 {
				assertCents(t, "EPF employer", tt.want.EPFEmployerCents, got.EPFEmployerCents)
			}
			if tt.want.ETFCents != 0 {
				assertCents(t, "ETF", tt.want.ETFCents, got.ETFCents)
			}
			if got.PayableDays != tt.want.PayableDays {
				t.Errorf("payable days = %v, want %v", got.PayableDays, tt.want.PayableDays)
			}
			if got.OvertimeMinutes != tt.want.OvertimeMinutes {
				t.Errorf("overtime minutes = %v, want %v", got.OvertimeMinutes, tt.want.OvertimeMinutes)
			}
			if got.Status != entity.PayrollStatusDraft {
				t.Errorf("status = %q, want draft", got.Status)
			}

			/*
				The deduction total is exactly the employee-borne lines. Stated
				as an identity rather than a bound, because the thing worth
				catching is the employer's EPF share or the ETF being taken out
				of somebody's wage — which a looser check would miss whenever an
				absence deduction happened to be larger.
			*/
			employeeBorne := got.AbsenceDeductionCents + got.EPFEmployeeCents +
				got.TaxCents + got.OtherDeductionCents
			if got.DeductionCents != employeeBorne {
				t.Errorf("deductions = %d, want the employee-borne lines only (%d)",
					got.DeductionCents, employeeBorne)
			}
		})
	}
}

func TestRetotalIncludesAdHocLines(t *testing.T) {
	payslip := entity.Payslip{
		BasicSalaryCents:    10_000_000,
		BonusCents:          500_000,
		EPFEmployeeCents:    800_000,
		OtherDeductionCents: 250_000,
	}

	retotal(&payslip)

	assertCents(t, "gross", 10_500_000, payslip.GrossCents)
	assertCents(t, "deductions", 1_050_000, payslip.DeductionCents)
	assertCents(t, "net", 9_450_000, payslip.NetCents)
}

func TestPercentOf(t *testing.T) {
	tests := []struct {
		name    string
		cents   int64
		percent float64
		want    int64
	}{
		{"zero percent is nothing", 10_000_000, 0, 0},
		{"negative percent is nothing", 10_000_000, -5, 0},
		{"whole percent", 10_000_000, 8, 800_000},
		// 3% of 3,333 cents is 99.99, which must round to a whole cent rather
		// than truncate — truncation would lose a cent on every payslip.
		{"rounds to the nearest cent", 3_333, 3, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := percentOf(tt.cents, tt.percent); got != tt.want {
				t.Errorf("percentOf(%d, %v) = %d, want %d", tt.cents, tt.percent, got, tt.want)
			}
		})
	}
}

func TestMonthRangeEndsOnTheLastDayOfTheMonth(t *testing.T) {
	tests := []struct {
		name  string
		year  int
		month int
		want  string
	}{
		{"31 day month", 2026, 3, "2026-03-31"},
		{"30 day month", 2026, 4, "2026-04-30"},
		{"february", 2026, 2, "2026-02-28"},
		{"leap february", 2028, 2, "2028-02-29"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := monthRange(tt.year, tt.month)
			if formatted := got.To.Format("2006-01-02"); formatted != tt.want {
				t.Errorf("range end = %s, want %s", formatted, tt.want)
			}
		})
	}
}

func assertCents(t *testing.T, label string, want, got int64) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d cents, want %d", label, got, want)
	}
}
