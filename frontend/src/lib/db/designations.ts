import { db } from "./index";
import type { Designation } from "@/lib/types";

// Local-only table, no server sync yet (no backend endpoint exists).

export async function listDesignations(): Promise<Designation[]> {
  return db.designations.orderBy("name").toArray();
}

export async function getDesignation(id: string): Promise<Designation | undefined> {
  return db.designations.get(id);
}

export interface CreateDesignationInput {
  name: string;
  description?: string;
  active?: boolean;
}

export async function createDesignation(
  input: CreateDesignationInput,
): Promise<Designation> {
  const name = input.name.trim();
  if (!name) throw new Error("Designation name is required");
  const clash = await db.designations
    .filter((designation) => designation.name.toLowerCase() === name.toLowerCase())
    .first();
  if (clash) throw new Error("A designation with that name already exists");

  const designation: Designation = {
    id: crypto.randomUUID(),
    name,
    description: input.description?.trim() || undefined,
    active: input.active ?? true,
    created_at: Date.now(),
  };
  await db.designations.add(designation);
  return designation;
}

export async function updateDesignation(
  id: string,
  changes: Partial<Pick<Designation, "name" | "description" | "active">>,
): Promise<void> {
  const name = changes.name?.trim();
  if (name === "") throw new Error("Designation name is required");
  if (name) {
    const clash = await db.designations
      .filter(
        (designation) =>
          designation.id !== id && designation.name.toLowerCase() === name.toLowerCase(),
      )
      .first();
    if (clash) throw new Error("A designation with that name already exists");
  }

  await db.designations.update(id, {
    ...(name ? { name } : {}),
    ...(changes.description !== undefined
      ? { description: changes.description.trim() || undefined }
      : {}),
    ...(changes.active !== undefined ? { active: changes.active } : {}),
  });
}

export async function deleteDesignation(id: string): Promise<void> {
  await db.designations.delete(id);
}
