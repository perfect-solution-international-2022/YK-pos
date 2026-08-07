import { db } from "./index";
import type { LeaveRequest, LeaveStatus, LeaveType } from "@/lib/types";

// Local-only table, no server sync yet (no backend endpoint exists).

const MS_PER_DAY = 86_400_000;

function daysInclusive(startDate: string, endDate: string): number {
  const start = new Date(`${startDate}T00:00:00`);
  const end = new Date(`${endDate}T00:00:00`);
  return Math.round((end.getTime() - start.getTime()) / MS_PER_DAY) + 1;
}

export async function listLeaveRequests(): Promise<LeaveRequest[]> {
  return db.leaveRequests.orderBy("requested_at").reverse().toArray();
}

export async function getLeaveRequest(id: string): Promise<LeaveRequest | undefined> {
  return db.leaveRequests.get(id);
}

export interface CreateLeaveRequestInput {
  employee_id: string;
  leave_type_id: string;
  start_date: string;
  end_date: string;
  reason?: string;
}

export async function createLeaveRequest(
  input: CreateLeaveRequestInput,
): Promise<LeaveRequest> {
  const days = daysInclusive(input.start_date, input.end_date);
  if (days < 1) throw new Error("End date must be on or after the start date");

  const request: LeaveRequest = {
    id: crypto.randomUUID(),
    employee_id: input.employee_id,
    leave_type_id: input.leave_type_id,
    start_date: input.start_date,
    end_date: input.end_date,
    days,
    reason: input.reason,
    status: "pending",
    requested_at: Date.now(),
  };
  await db.leaveRequests.add(request);
  return request;
}

export async function decideLeaveRequest(
  id: string,
  status: Extract<LeaveStatus, "approved" | "rejected">,
  decidedBy: string,
  note?: string,
): Promise<void> {
  await db.leaveRequests.update(id, {
    status,
    decided_at: Date.now(),
    decided_by: decidedBy,
    decision_note: note,
  });
}

export interface LeaveBalance {
  allocated: number;
  used: number;
  remaining: number;
}

export function leaveBalance(
  employeeId: string,
  leaveTypeId: string,
  year: number,
  requests: LeaveRequest[],
  leaveType: LeaveType,
): LeaveBalance {
  const used = requests
    .filter(
      (request) =>
        request.employee_id === employeeId &&
        request.leave_type_id === leaveTypeId &&
        request.status === "approved" &&
        new Date(`${request.start_date}T00:00:00`).getFullYear() === year,
    )
    .reduce((sum, request) => sum + request.days, 0);

  return {
    allocated: leaveType.days_per_year,
    used,
    remaining: leaveType.days_per_year - used,
  };
}

/** Everyone with an approved request covering `date` (yyyy-mm-dd). */
export function activeLeaveToday(requests: LeaveRequest[], date: string): Set<string> {
  return new Set(
    requests
      .filter(
        (request) =>
          request.status === "approved" &&
          request.start_date <= date &&
          date <= request.end_date,
      )
      .map((request) => request.employee_id),
  );
}
