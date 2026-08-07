-- Human Resource Management.
--
-- Every table is prefixed hrm_ and scoped by business_id, so the module is a
-- self-contained slice of the schema that shares nothing with the POS tables
-- except the tenant (businesses), the staff login (users) and the branch a
-- person is posted to. Nothing here alters an existing table.
--
-- Conventions carried over from the POS schema:
--   * UUID primary keys minted by the application (UUIDv7), never by the DB.
--   * Money is BIGINT integer cents, never NUMERIC and never a float.
--   * Calendar dates are DATE, clock times are TIME — no timezone on either,
--     because a shift starts at 08:00 wherever the shop is.
--   * Soft delete on the master data a historical row still has to resolve
--     (employees, designations, shifts); hard rows for transactional records.
--   * Unique indexes are partial (WHERE deleted_at IS NULL) so deleting a
--     record frees its natural key for reuse.

-- +goose Up

-- ---------------------------------------------------------------------------
-- Designations (job titles)
-- ---------------------------------------------------------------------------
CREATE TABLE hrm_designations (
    id            UUID PRIMARY KEY,
    business_id   UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    description   TEXT NULL,
    status        TEXT NOT NULL DEFAULT 'active'
                      CHECK (status IN ('active', 'inactive')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX hrm_designations_business_id_name_key
    ON hrm_designations (business_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX hrm_designations_business_id_idx ON hrm_designations (business_id);

CREATE TRIGGER hrm_designations_set_updated_at
    BEFORE UPDATE ON hrm_designations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Shifts
-- ---------------------------------------------------------------------------
-- end_time may be earlier than start_time: a night shift crosses midnight, and
-- rejecting that would make the schema unable to describe a real roster. The
-- service reads the pair as a window, adding a day when end <= start.
CREATE TABLE hrm_shifts (
    id             UUID PRIMARY KEY,
    business_id    UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    start_time     TIME NOT NULL,
    end_time       TIME NOT NULL,
    break_minutes  INTEGER NOT NULL DEFAULT 0 CHECK (break_minutes >= 0 AND break_minutes <= 480),
    -- Lateness allowance before an arrival counts as late.
    grace_minutes  INTEGER NOT NULL DEFAULT 0 CHECK (grace_minutes >= 0 AND grace_minutes <= 240),
    -- ISO-ish weekday numbers, 0 (Sunday) - 6 (Saturday), matching the
    -- frontend's Shift.weekly_off and JavaScript's Date#getDay().
    weekly_off     JSONB NOT NULL DEFAULT '[]',
    status         TEXT NOT NULL DEFAULT 'active'
                       CHECK (status IN ('active', 'inactive')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX hrm_shifts_business_id_name_key
    ON hrm_shifts (business_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX hrm_shifts_business_id_idx ON hrm_shifts (business_id);

CREATE TRIGGER hrm_shifts_set_updated_at
    BEFORE UPDATE ON hrm_shifts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Employees
-- ---------------------------------------------------------------------------
-- user_id links the HR record to a login. It is nullable and ON DELETE SET
-- NULL: not every employee gets a till account, and removing an account must
-- not remove the person's payroll history.
--
-- designation_id and shift_id are RESTRICT rather than CASCADE — deleting a
-- job title out from under the people holding it would silently corrupt
-- payroll. The service reports the conflict instead.
CREATE TABLE hrm_employees (
    id                      UUID PRIMARY KEY,
    business_id             UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    branch_id               UUID NULL REFERENCES branches (id) ON DELETE SET NULL,
    user_id                 UUID NULL REFERENCES users (id) ON DELETE SET NULL,

    -- Sequential per business, assigned by the service. Text, not an integer:
    -- shops number staff with prefixes and leading zeros.
    employee_code           TEXT NOT NULL,
    full_name               TEXT NOT NULL,
    nic                     TEXT NOT NULL,
    date_of_birth           DATE NULL,
    gender                  TEXT NOT NULL DEFAULT 'other'
                                CHECK (gender IN ('male', 'female', 'other')),
    phone                   TEXT NOT NULL,
    email                   TEXT NULL,
    address                 TEXT NULL,
    emergency_contact_name  TEXT NULL,
    emergency_contact_phone TEXT NULL,

    designation_id          UUID NOT NULL REFERENCES hrm_designations (id) ON DELETE RESTRICT,
    joining_date            DATE NOT NULL,
    employment_type         TEXT NOT NULL DEFAULT 'full_time'
                                CHECK (employment_type IN ('full_time', 'part_time', 'contract', 'intern')),
    shift_id                UUID NULL REFERENCES hrm_shifts (id) ON DELETE RESTRICT,

    basic_salary_cents      BIGINT NOT NULL DEFAULT 0 CHECK (basic_salary_cents >= 0),
    bank_name               TEXT NULL,
    bank_account_no         TEXT NULL,
    bank_branch             TEXT NULL,

    -- Data URL, the same way products carry their images in this app.
    photo_url               TEXT NULL,

    status                  TEXT NOT NULL DEFAULT 'active'
                                CHECK (status IN ('active', 'inactive', 'suspended', 'resigned')),
    resigned_at             DATE NULL,
    notes                   TEXT NULL,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at              TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX hrm_employees_business_id_employee_code_key
    ON hrm_employees (business_id, employee_code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX hrm_employees_business_id_nic_key
    ON hrm_employees (business_id, upper(nic)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX hrm_employees_business_id_email_key
    ON hrm_employees (business_id, lower(email)) WHERE deleted_at IS NULL AND email IS NOT NULL;
CREATE UNIQUE INDEX hrm_employees_user_id_key
    ON hrm_employees (user_id) WHERE deleted_at IS NULL AND user_id IS NOT NULL;

CREATE INDEX hrm_employees_business_id_status_idx ON hrm_employees (business_id, status);
CREATE INDEX hrm_employees_business_id_designation_idx ON hrm_employees (business_id, designation_id);
CREATE INDEX hrm_employees_business_id_shift_idx ON hrm_employees (business_id, shift_id);
-- Covers the list screen's default ordering and its birthday panel.
CREATE INDEX hrm_employees_business_id_full_name_idx ON hrm_employees (business_id, full_name);
CREATE INDEX hrm_employees_business_id_dob_idx ON hrm_employees (business_id, date_of_birth)
    WHERE date_of_birth IS NOT NULL;

CREATE TRIGGER hrm_employees_set_updated_at
    BEFORE UPDATE ON hrm_employees
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Documents are a child table rather than a JSONB column on the employee: they
-- are uploaded and deleted one at a time, and a data URL per row keeps the
-- employee row small enough for the list query to stay cheap.
CREATE TABLE hrm_employee_documents (
    id           UUID PRIMARY KEY,
    employee_id  UUID NOT NULL REFERENCES hrm_employees (id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    doc_type     TEXT NULL,
    data_url     TEXT NOT NULL,
    size_bytes   BIGINT NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
    uploaded_by  UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX hrm_employee_documents_employee_id_idx ON hrm_employee_documents (employee_id);

CREATE TRIGGER hrm_employee_documents_set_updated_at
    BEFORE UPDATE ON hrm_employee_documents
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Attendance
-- ---------------------------------------------------------------------------
-- One row per employee per working day. The unique index is what makes clock-in
-- idempotent and stops a double-tap producing two half-days.
--
-- The derived minute columns are stored rather than computed on read: payroll
-- reads them for a whole month across every employee, and recomputing shift
-- arithmetic per row in that query would make the payroll run scale with
-- history rather than with headcount.
CREATE TABLE hrm_attendance (
    id                    UUID PRIMARY KEY,
    business_id           UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    employee_id           UUID NOT NULL REFERENCES hrm_employees (id) ON DELETE CASCADE,
    shift_id              UUID NULL REFERENCES hrm_shifts (id) ON DELETE SET NULL,

    work_date             DATE NOT NULL,
    clock_in_at           TIMESTAMPTZ NULL,
    clock_out_at          TIMESTAMPTZ NULL,

    status                TEXT NOT NULL DEFAULT 'present'
                              CHECK (status IN ('present', 'late', 'half_day', 'absent', 'on_leave', 'holiday', 'weekly_off')),
    -- 'clock' for a till/kiosk punch, 'manual' for a supervisor's correction,
    -- 'system' for a row generated by leave approval or the monthly rollup.
    source                TEXT NOT NULL DEFAULT 'clock'
                              CHECK (source IN ('clock', 'manual', 'system')),

    worked_minutes        INTEGER NOT NULL DEFAULT 0 CHECK (worked_minutes >= 0),
    late_minutes          INTEGER NOT NULL DEFAULT 0 CHECK (late_minutes >= 0),
    early_leave_minutes   INTEGER NOT NULL DEFAULT 0 CHECK (early_leave_minutes >= 0),
    overtime_minutes      INTEGER NOT NULL DEFAULT 0 CHECK (overtime_minutes >= 0),

    note                  TEXT NULL,
    recorded_by           UUID NULL REFERENCES users (id) ON DELETE SET NULL,

    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT hrm_attendance_clock_order CHECK (
        clock_out_at IS NULL OR clock_in_at IS NULL OR clock_out_at >= clock_in_at
    )
);

CREATE UNIQUE INDEX hrm_attendance_employee_id_work_date_key
    ON hrm_attendance (employee_id, work_date);
CREATE INDEX hrm_attendance_business_id_work_date_idx ON hrm_attendance (business_id, work_date DESC);
CREATE INDEX hrm_attendance_business_id_status_idx ON hrm_attendance (business_id, status);

CREATE TRIGGER hrm_attendance_set_updated_at
    BEFORE UPDATE ON hrm_attendance
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Leave
-- ---------------------------------------------------------------------------
-- business_id NULL marks a system leave type available to every tenant, the
-- same pattern the roles table uses for the four global system roles. A shop
-- can add its own types without anything having to seed them per business.
CREATE TABLE hrm_leave_types (
    id                 UUID PRIMARY KEY,
    business_id        UUID NULL REFERENCES businesses (id) ON DELETE CASCADE,
    name               TEXT NOT NULL,
    code               TEXT NOT NULL,
    -- Entitlement per calendar year. 0 means "unlimited but unpaid" types such
    -- as no-pay leave, which the balance calculation reports as not applicable.
    annual_quota_days  NUMERIC(6, 2) NOT NULL DEFAULT 0 CHECK (annual_quota_days >= 0),
    is_paid            BOOLEAN NOT NULL DEFAULT true,
    is_system          BOOLEAN NOT NULL DEFAULT false,
    status             TEXT NOT NULL DEFAULT 'active'
                           CHECK (status IN ('active', 'inactive')),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at         TIMESTAMPTZ NULL
);

-- Uniqueness spans the nullable business_id, so it is expressed over a COALESCE
-- expression — the same shape roles uses, for the same reason: a bare column
-- pair would let every tenant re-register a system code.
CREATE UNIQUE INDEX hrm_leave_types_business_id_code_key
    ON hrm_leave_types (COALESCE(business_id, '00000000-0000-0000-0000-000000000000'::uuid), upper(code))
    WHERE deleted_at IS NULL;
CREATE INDEX hrm_leave_types_business_id_idx ON hrm_leave_types (business_id);

CREATE TRIGGER hrm_leave_types_set_updated_at
    BEFORE UPDATE ON hrm_leave_types
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE hrm_leave_requests (
    id             UUID PRIMARY KEY,
    business_id    UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    employee_id    UUID NOT NULL REFERENCES hrm_employees (id) ON DELETE CASCADE,
    leave_type_id  UUID NOT NULL REFERENCES hrm_leave_types (id) ON DELETE RESTRICT,

    start_date     DATE NOT NULL,
    end_date       DATE NOT NULL,
    -- Stored rather than derived: a half-day is 0.5, and weekly-off days inside
    -- the range are excluded at request time against the shift the employee held
    -- then, which a later shift change would otherwise silently rewrite.
    days           NUMERIC(6, 2) NOT NULL CHECK (days > 0),
    half_day       BOOLEAN NOT NULL DEFAULT false,
    reason         TEXT NULL,

    status         TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
    reviewed_by    UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    reviewed_at    TIMESTAMPTZ NULL,
    review_note    TEXT NULL,

    requested_by   UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT hrm_leave_requests_date_order CHECK (end_date >= start_date)
);

CREATE INDEX hrm_leave_requests_business_id_status_idx ON hrm_leave_requests (business_id, status);
CREATE INDEX hrm_leave_requests_employee_id_start_date_idx ON hrm_leave_requests (employee_id, start_date DESC);
-- Serves the balance query, which sums approved days per employee per year.
CREATE INDEX hrm_leave_requests_employee_type_status_idx
    ON hrm_leave_requests (employee_id, leave_type_id, status);

CREATE TRIGGER hrm_leave_requests_set_updated_at
    BEFORE UPDATE ON hrm_leave_requests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Payroll
-- ---------------------------------------------------------------------------
-- A run is one month's payroll for one business. Regenerating a draft replaces
-- its payslips; finalising freezes them, because a payslip a member of staff has
-- already been shown must not change under them.
CREATE TABLE hrm_payroll_runs (
    id             UUID PRIMARY KEY,
    business_id    UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    period_year    INTEGER NOT NULL CHECK (period_year BETWEEN 2000 AND 2200),
    period_month   INTEGER NOT NULL CHECK (period_month BETWEEN 1 AND 12),
    status         TEXT NOT NULL DEFAULT 'draft'
                       CHECK (status IN ('draft', 'finalized', 'paid')),
    -- Snapshot of the payroll rules the run was computed under, so a later
    -- change to settings cannot silently reinterpret a historical payslip.
    rules          JSONB NOT NULL DEFAULT '{}',
    notes          TEXT NULL,
    generated_by   UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    finalized_at   TIMESTAMPTZ NULL,
    paid_at        TIMESTAMPTZ NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX hrm_payroll_runs_business_period_key
    ON hrm_payroll_runs (business_id, period_year, period_month);

CREATE TRIGGER hrm_payroll_runs_set_updated_at
    BEFORE UPDATE ON hrm_payroll_runs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE hrm_payslips (
    id                     UUID PRIMARY KEY,
    payroll_run_id         UUID NOT NULL REFERENCES hrm_payroll_runs (id) ON DELETE CASCADE,
    business_id            UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    employee_id            UUID NOT NULL REFERENCES hrm_employees (id) ON DELETE RESTRICT,

    -- Denormalised from the run so payslip history can be filtered by period
    -- without joining, which is what every payroll report does.
    period_year            INTEGER NOT NULL,
    period_month           INTEGER NOT NULL,

    basic_salary_cents     BIGINT NOT NULL DEFAULT 0 CHECK (basic_salary_cents >= 0),
    payable_days           NUMERIC(6, 2) NOT NULL DEFAULT 0,
    present_days           NUMERIC(6, 2) NOT NULL DEFAULT 0,
    absent_days            NUMERIC(6, 2) NOT NULL DEFAULT 0,
    paid_leave_days        NUMERIC(6, 2) NOT NULL DEFAULT 0,
    unpaid_leave_days      NUMERIC(6, 2) NOT NULL DEFAULT 0,

    overtime_minutes       INTEGER NOT NULL DEFAULT 0 CHECK (overtime_minutes >= 0),
    overtime_cents         BIGINT NOT NULL DEFAULT 0 CHECK (overtime_cents >= 0),
    bonus_cents            BIGINT NOT NULL DEFAULT 0 CHECK (bonus_cents >= 0),
    allowance_cents        BIGINT NOT NULL DEFAULT 0 CHECK (allowance_cents >= 0),

    absence_deduction_cents BIGINT NOT NULL DEFAULT 0 CHECK (absence_deduction_cents >= 0),
    epf_employee_cents     BIGINT NOT NULL DEFAULT 0 CHECK (epf_employee_cents >= 0),
    epf_employer_cents     BIGINT NOT NULL DEFAULT 0 CHECK (epf_employer_cents >= 0),
    etf_cents              BIGINT NOT NULL DEFAULT 0 CHECK (etf_cents >= 0),
    tax_cents              BIGINT NOT NULL DEFAULT 0 CHECK (tax_cents >= 0),
    other_deduction_cents  BIGINT NOT NULL DEFAULT 0 CHECK (other_deduction_cents >= 0),

    gross_cents            BIGINT NOT NULL DEFAULT 0,
    deduction_cents        BIGINT NOT NULL DEFAULT 0,
    -- Net may legitimately be 0 but never negative: the calculation floors it,
    -- because paying a negative wage is a data-entry error, not a payroll rule.
    net_cents              BIGINT NOT NULL DEFAULT 0 CHECK (net_cents >= 0),

    status                 TEXT NOT NULL DEFAULT 'draft'
                               CHECK (status IN ('draft', 'finalized', 'paid')),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX hrm_payslips_run_id_employee_id_key
    ON hrm_payslips (payroll_run_id, employee_id);
CREATE INDEX hrm_payslips_employee_period_idx
    ON hrm_payslips (employee_id, period_year DESC, period_month DESC);
CREATE INDEX hrm_payslips_business_period_idx
    ON hrm_payslips (business_id, period_year DESC, period_month DESC);

CREATE TRIGGER hrm_payslips_set_updated_at
    BEFORE UPDATE ON hrm_payslips
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Ad-hoc earnings and deductions a manager adds to a draft payslip. Kept
-- relational so a payslip can print its own breakdown line by line.
CREATE TABLE hrm_payslip_items (
    id            UUID PRIMARY KEY,
    payslip_id    UUID NOT NULL REFERENCES hrm_payslips (id) ON DELETE CASCADE,
    kind          TEXT NOT NULL CHECK (kind IN ('earning', 'deduction')),
    label         TEXT NOT NULL,
    amount_cents  BIGINT NOT NULL CHECK (amount_cents >= 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX hrm_payslip_items_payslip_id_idx ON hrm_payslip_items (payslip_id);

CREATE TRIGGER hrm_payslip_items_set_updated_at
    BEFORE UPDATE ON hrm_payslip_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Performance
-- ---------------------------------------------------------------------------
CREATE TABLE hrm_performance_reviews (
    id             UUID PRIMARY KEY,
    business_id    UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    employee_id    UUID NOT NULL REFERENCES hrm_employees (id) ON DELETE CASCADE,
    period_year    INTEGER NOT NULL CHECK (period_year BETWEEN 2000 AND 2200),
    period_month   INTEGER NOT NULL CHECK (period_month BETWEEN 1 AND 12),
    -- 1.00 - 5.00, halves allowed. A rate, not currency, so not in cents.
    rating         NUMERIC(3, 2) NOT NULL CHECK (rating >= 1 AND rating <= 5),
    -- Optional sub-scores on the same 1-5 scale; absent when a shop grades on
    -- the overall rating alone.
    punctuality    NUMERIC(3, 2) NULL CHECK (punctuality IS NULL OR (punctuality >= 1 AND punctuality <= 5)),
    teamwork       NUMERIC(3, 2) NULL CHECK (teamwork IS NULL OR (teamwork >= 1 AND teamwork <= 5)),
    productivity   NUMERIC(3, 2) NULL CHECK (productivity IS NULL OR (productivity >= 1 AND productivity <= 5)),
    manager_notes  TEXT NULL,
    reviewer_id    UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One evaluation per employee per month: a second one is an edit of the first,
-- not a new record, or the average rating would depend on how many times a
-- manager saved the form.
CREATE UNIQUE INDEX hrm_performance_reviews_employee_period_key
    ON hrm_performance_reviews (employee_id, period_year, period_month);
CREATE INDEX hrm_performance_reviews_business_period_idx
    ON hrm_performance_reviews (business_id, period_year DESC, period_month DESC);

CREATE TRIGGER hrm_performance_reviews_set_updated_at
    BEFORE UPDATE ON hrm_performance_reviews
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Announcements
-- ---------------------------------------------------------------------------
CREATE TABLE hrm_announcements (
    id            UUID PRIMARY KEY,
    business_id   UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    title         TEXT NOT NULL,
    description   TEXT NOT NULL,
    publish_date  DATE NOT NULL,
    expires_on    DATE NULL,
    status        TEXT NOT NULL DEFAULT 'draft'
                      CHECK (status IN ('draft', 'published', 'archived')),
    created_by    UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ NULL,

    CONSTRAINT hrm_announcements_date_order CHECK (expires_on IS NULL OR expires_on >= publish_date)
);

CREATE INDEX hrm_announcements_business_status_publish_idx
    ON hrm_announcements (business_id, status, publish_date DESC);

CREATE TRIGGER hrm_announcements_set_updated_at
    BEFORE UPDATE ON hrm_announcements
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Seed: system leave types
-- ---------------------------------------------------------------------------
-- business_id NULL, is_system true. Every tenant sees these without a per-shop
-- seeding step; a shop that wants different quotas adds its own type and
-- deactivates the system one. Idempotent so a restored dump can be re-migrated.
INSERT INTO hrm_leave_types (id, business_id, name, code, annual_quota_days, is_paid, is_system)
VALUES
    (gen_random_uuid(), NULL, 'Annual Leave',    'ANNUAL',    14, true,  true),
    (gen_random_uuid(), NULL, 'Casual Leave',    'CASUAL',     7, true,  true),
    (gen_random_uuid(), NULL, 'Sick Leave',      'SICK',       7, true,  true),
    (gen_random_uuid(), NULL, 'Maternity Leave', 'MATERNITY', 84, true,  true),
    (gen_random_uuid(), NULL, 'No-Pay Leave',    'NOPAY',      0, false, true)
ON CONFLICT (COALESCE(business_id, '00000000-0000-0000-0000-000000000000'::uuid), upper(code))
    WHERE deleted_at IS NULL
    DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS hrm_announcements;
DROP TABLE IF EXISTS hrm_performance_reviews;
DROP TABLE IF EXISTS hrm_payslip_items;
DROP TABLE IF EXISTS hrm_payslips;
DROP TABLE IF EXISTS hrm_payroll_runs;
DROP TABLE IF EXISTS hrm_leave_requests;
DROP TABLE IF EXISTS hrm_leave_types;
DROP TABLE IF EXISTS hrm_attendance;
DROP TABLE IF EXISTS hrm_employee_documents;
DROP TABLE IF EXISTS hrm_employees;
DROP TABLE IF EXISTS hrm_shifts;
DROP TABLE IF EXISTS hrm_designations;
