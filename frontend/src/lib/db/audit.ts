import { db } from "./index";
import type { AuditLogEntry } from "@/lib/types";

// Local-only table, no server sync yet (no backend endpoint exists). Written
// from pages rather than other db modules — the acting user lives in the
// Zustand auth store, not reachable from lib/db.

export async function listAuditLogs(): Promise<AuditLogEntry[]> {
  return db.auditLogs.orderBy("created_at").reverse().toArray();
}

export async function logAudit(
  input: Omit<AuditLogEntry, "id" | "created_at">,
): Promise<void> {
  const entry: AuditLogEntry = {
    ...input,
    id: crypto.randomUUID(),
    created_at: Date.now(),
  };
  await db.auditLogs.add(entry);
}
