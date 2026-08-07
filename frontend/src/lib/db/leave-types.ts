import { db } from "./index";
import type { LeaveType } from "@/lib/types";

// Local-only table, no server sync yet (no backend endpoint exists).

export async function listLeaveTypes(): Promise<LeaveType[]> {
  return db.leaveTypes.orderBy("name").toArray();
}

export async function getLeaveType(id: string): Promise<LeaveType | undefined> {
  return db.leaveTypes.get(id);
}

export interface CreateLeaveTypeInput {
  name: string;
  days_per_year: number;
  paid?: boolean;
  active?: boolean;
}

export async function createLeaveType(input: CreateLeaveTypeInput): Promise<LeaveType> {
  const name = input.name.trim();
  if (!name) throw new Error("Leave type name is required");
  const clash = await db.leaveTypes
    .filter((leaveType) => leaveType.name.toLowerCase() === name.toLowerCase())
    .first();
  if (clash) throw new Error("A leave type with that name already exists");

  const leaveType: LeaveType = {
    id: crypto.randomUUID(),
    name,
    days_per_year: input.days_per_year,
    paid: input.paid ?? true,
    active: input.active ?? true,
    created_at: Date.now(),
  };
  await db.leaveTypes.add(leaveType);
  return leaveType;
}

export async function updateLeaveType(
  id: string,
  changes: Partial<Pick<LeaveType, "name" | "days_per_year" | "paid" | "active">>,
): Promise<void> {
  const name = changes.name?.trim();
  if (name === "") throw new Error("Leave type name is required");
  if (name) {
    const clash = await db.leaveTypes
      .filter(
        (leaveType) => leaveType.id !== id && leaveType.name.toLowerCase() === name.toLowerCase(),
      )
      .first();
    if (clash) throw new Error("A leave type with that name already exists");
  }

  await db.leaveTypes.update(id, { ...changes, ...(name ? { name } : {}) });
}

export async function deleteLeaveType(id: string): Promise<void> {
  await db.leaveTypes.delete(id);
}
