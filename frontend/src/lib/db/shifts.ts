import { db } from "./index";
import type { Shift } from "@/lib/types";

// Local-only table, no server sync yet (no backend endpoint exists).

export async function listShifts(): Promise<Shift[]> {
  return db.shifts.orderBy("name").toArray();
}

export async function getShift(id: string): Promise<Shift | undefined> {
  return db.shifts.get(id);
}

export interface CreateShiftInput {
  name: string;
  start_time: string;
  end_time: string;
  break_minutes: number;
  grace_minutes: number;
  weekly_off: number[];
  active?: boolean;
}

export async function createShift(input: CreateShiftInput): Promise<Shift> {
  const name = input.name.trim();
  if (!name) throw new Error("Shift name is required");
  const clash = await db.shifts
    .filter((shift) => shift.name.toLowerCase() === name.toLowerCase())
    .first();
  if (clash) throw new Error("A shift with that name already exists");

  const shift: Shift = {
    id: crypto.randomUUID(),
    name,
    start_time: input.start_time,
    end_time: input.end_time,
    break_minutes: input.break_minutes,
    grace_minutes: input.grace_minutes,
    weekly_off: input.weekly_off,
    active: input.active ?? true,
    created_at: Date.now(),
  };
  await db.shifts.add(shift);
  return shift;
}

export async function updateShift(
  id: string,
  changes: Partial<Omit<Shift, "id" | "created_at">>,
): Promise<void> {
  const name = changes.name?.trim();
  if (name === "") throw new Error("Shift name is required");
  if (name) {
    const clash = await db.shifts
      .filter((shift) => shift.id !== id && shift.name.toLowerCase() === name.toLowerCase())
      .first();
    if (clash) throw new Error("A shift with that name already exists");
  }

  await db.shifts.update(id, { ...changes, ...(name ? { name } : {}) });
}

export async function deleteShift(id: string): Promise<void> {
  await db.shifts.delete(id);
}
