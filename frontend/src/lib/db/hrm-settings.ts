import { db } from "./index";
import type { HrmSettings } from "@/lib/types";

const SETTINGS_KEY = "hrm_settings";

export const DEFAULT_HRM_SETTINGS: HrmSettings = {
  probation_period_days: 90,
  payroll_cycle: "monthly",
  default_pay_day: 28,
  default_grace_minutes: 10,
  carry_forward_leave: false,
  max_carry_forward_days: 0,
};

function isPartialHrmSettings(value: unknown): value is Partial<HrmSettings> {
  return typeof value === "object" && value !== null;
}

// Reads the persisted settings merged over the defaults, so a settings blob
// written by an older build is still usable after new fields are added —
// same pattern as `getStoreSettings` in `lib/db/settings.ts`.
export async function getHrmSettings(): Promise<HrmSettings> {
  const record = await db.syncMeta.get(SETTINGS_KEY);
  if (!isPartialHrmSettings(record?.value)) return DEFAULT_HRM_SETTINGS;
  return { ...DEFAULT_HRM_SETTINGS, ...record.value };
}

export async function saveHrmSettings(
  changes: Partial<HrmSettings>,
): Promise<HrmSettings> {
  const current = await getHrmSettings();
  const next: HrmSettings = { ...current, ...changes };
  await db.syncMeta.put({ key: SETTINGS_KEY, value: next });
  return next;
}
