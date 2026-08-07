import { db } from "./index";
import { localDateKey, type DateRange } from "./reports";
import type { AttendanceRecord, AttendanceStatus, Employee, Shift } from "@/lib/types";

// Local-only table, no server sync yet (no backend endpoint exists). One row
// per (employee_id, date), upserted — never multiple rows for the same day.

export async function getAttendanceRecord(
  employeeId: string,
  date: string,
): Promise<AttendanceRecord | undefined> {
  return db.attendance.where("[employee_id+date]").equals([employeeId, date]).first();
}

export async function listAttendanceForDate(date: string): Promise<AttendanceRecord[]> {
  return db.attendance.where("date").equals(date).toArray();
}

export async function listAttendanceInRange(range: DateRange): Promise<AttendanceRecord[]> {
  const from = localDateKey(range.from);
  const to = localDateKey(range.to);
  return db.attendance.where("date").between(from, to, true, true).toArray();
}

async function upsert(
  employeeId: string,
  date: string,
  changes: Partial<Omit<AttendanceRecord, "id" | "employee_id" | "date" | "created_at">>,
): Promise<AttendanceRecord> {
  const existing = await getAttendanceRecord(employeeId, date);
  if (existing) {
    const updated: AttendanceRecord = { ...existing, ...changes };
    await db.attendance.put(updated);
    return updated;
  }
  const created: AttendanceRecord = {
    id: crypto.randomUUID(),
    employee_id: employeeId,
    date,
    status: "present",
    created_at: Date.now(),
    ...changes,
  };
  await db.attendance.add(created);
  return created;
}

function minutesSinceMidnight(date: Date): number {
  return date.getHours() * 60 + date.getMinutes();
}

function shiftStartMinutes(shift: Shift): number {
  const [hours, minutes] = shift.start_time.split(":").map(Number);
  return hours * 60 + minutes;
}

/** "late" once `now` is past the shift's start time plus its grace window. */
function statusForClockIn(now: Date, shift?: Shift): AttendanceStatus {
  if (!shift) return "present";
  const lateAfter = shiftStartMinutes(shift) + shift.grace_minutes;
  return minutesSinceMidnight(now) > lateAfter ? "late" : "present";
}

export async function clockIn(employeeId: string, shift?: Shift): Promise<AttendanceRecord> {
  const now = new Date();
  return upsert(employeeId, localDateKey(now.getTime()), {
    clock_in: now.getTime(),
    status: statusForClockIn(now, shift),
  });
}

export async function clockOut(employeeId: string): Promise<AttendanceRecord> {
  const now = new Date();
  const date = localDateKey(now.getTime());
  const existing = await getAttendanceRecord(employeeId, date);
  if (!existing?.clock_in) throw new Error("Clock in first");
  return upsert(employeeId, date, { clock_out: now.getTime() });
}

export interface ManualAttendanceInput {
  employee_id: string;
  date: string;
  /** "HH:mm", combined with `date` into an epoch-ms timestamp. */
  clock_in?: string;
  clock_out?: string;
  status: AttendanceStatus;
  notes?: string;
}

function combineDateTime(date: string, time: string): number {
  const [hours, minutes] = time.split(":").map(Number);
  const [year, month, day] = date.split("-").map(Number);
  return new Date(year, month - 1, day, hours, minutes).getTime();
}

export async function upsertManualAttendance(
  input: ManualAttendanceInput,
): Promise<AttendanceRecord> {
  return upsert(input.employee_id, input.date, {
    clock_in: input.clock_in ? combineDateTime(input.date, input.clock_in) : undefined,
    clock_out: input.clock_out ? combineDateTime(input.date, input.clock_out) : undefined,
    status: input.status,
    notes: input.notes,
  });
}

export interface DailyAttendanceSummary {
  present: number;
  late: number;
  halfDay: number;
  absent: number;
  onLeave: number;
}

/**
 * Single source of truth for "how was today" — the Daily view, the Report,
 * and the Dashboard cards all call this so they can never disagree. An
 * employee counts as `onLeave` first (an approved leave request wins over a
 * stray attendance row), then by their record's status, then — if today is
 * one of their shift's `weekly_off` days — is excluded entirely (a day off is
 * not an absence), otherwise `absent`.
 */
export function computeDailySummary(
  date: string,
  employees: Employee[],
  shifts: Shift[],
  records: AttendanceRecord[],
  activeLeaveEmployeeIds: Set<string>,
): DailyAttendanceSummary {
  const shiftById = new Map(shifts.map((shift) => [shift.id, shift]));
  const recordByEmployee = new Map(records.map((record) => [record.employee_id, record]));
  const weekday = new Date(`${date}T00:00:00`).getDay();

  const summary: DailyAttendanceSummary = {
    present: 0,
    late: 0,
    halfDay: 0,
    absent: 0,
    onLeave: 0,
  };

  for (const employee of employees.filter((employee) => employee.active)) {
    if (activeLeaveEmployeeIds.has(employee.id)) {
      summary.onLeave += 1;
      continue;
    }
    const record = recordByEmployee.get(employee.id);
    if (record) {
      if (record.status === "present") summary.present += 1;
      else if (record.status === "late") summary.late += 1;
      else if (record.status === "half_day") summary.halfDay += 1;
      else summary.absent += 1;
      continue;
    }
    const shift = shiftById.get(employee.shift_id);
    if (shift?.weekly_off.includes(weekday)) continue;
    summary.absent += 1;
  }

  return summary;
}
