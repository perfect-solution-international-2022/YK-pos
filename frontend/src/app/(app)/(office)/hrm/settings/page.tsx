"use client";

import { useEffect, useState } from "react";
import { Save } from "lucide-react";
import { getHrmSettings, saveHrmSettings } from "@/lib/db";
import type { HrmSettings } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { NumberField } from "@/components/ui/NumberField";
import { Card, PageHeader, SectionHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { Switch } from "@/components/ui/Switch";
import { useToast } from "@/components/ui/Toast";
import { ROUTES } from "@/lib/types/routes";
import { DEFAULT_HRM_SETTINGS } from "@/lib/db/hrm-settings";

function wholeNonNegative(raw: string): number {
  const parsed = Number(raw);
  if (!Number.isFinite(parsed)) return 0;
  return Math.max(0, Math.floor(parsed));
}

export default function HrmSettingsPage() {
  const { showToast } = useToast();
  const [draft, setDraft] = useState<HrmSettings>(DEFAULT_HRM_SETTINGS);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    void getHrmSettings().then(setDraft);
  }, []);

  function update<K extends keyof HrmSettings>(key: K, value: HrmSettings[K]) {
    setDirty(true);
    setDraft((current) => ({ ...current, [key]: value }));
  }

  async function handleSave() {
    setSaving(true);
    try {
      await saveHrmSettings(draft);
      setDirty(false);
      showToast("Settings saved", "success");
    } catch {
      showToast("Failed to save settings", "error");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="HRM Settings"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Settings" }]}
        actions={
          <Button type="button" onClick={() => void handleSave()} disabled={saving || !dirty}>
            <Save size={16} />
            {saving ? "Saving…" : "Save settings"}
          </Button>
        }
      />

      <div className="grid gap-5 lg:grid-cols-2">
        <Card>
          <SectionHeader title="HR Preferences" />
          <div className="mt-4 flex flex-col gap-3">
            <Input
              label="HR contact email"
              type="email"
              value={draft.hr_contact_email ?? ""}
              onChange={(event) => update("hr_contact_email", event.target.value)}
            />
            <NumberField
              label="Probation period (days)"
              value={String(draft.probation_period_days)}
              onChange={(value) => update("probation_period_days", wholeNonNegative(value))}
              max={365}
            />
          </div>
        </Card>

        <Card>
          <SectionHeader title="Payroll Settings" />
          <div className="mt-4 flex flex-col gap-3">
            <Select
              label="Payroll cycle"
              value={draft.payroll_cycle}
              onChange={(event) =>
                update("payroll_cycle", event.target.value as HrmSettings["payroll_cycle"])
              }
              options={[
                { value: "monthly", label: "Monthly" },
                { value: "biweekly", label: "Biweekly" },
              ]}
            />
            <NumberField
              label="Default pay day"
              value={String(draft.default_pay_day)}
              onChange={(value) => update("default_pay_day", wholeNonNegative(value))}
              min={1}
              max={28}
              hint="Day of the month, 1-28."
            />
          </div>
        </Card>

        <Card>
          <SectionHeader title="Attendance Settings" />
          <div className="mt-4 flex flex-col gap-3">
            <NumberField
              label="Default grace period (minutes)"
              value={String(draft.default_grace_minutes)}
              onChange={(value) => update("default_grace_minutes", wholeNonNegative(value))}
              max={120}
              hint="Used as a starting point when creating a new shift."
            />
          </div>
        </Card>

        <Card>
          <SectionHeader title="Leave Settings" />
          <div className="mt-4 flex flex-col gap-3">
            <Switch
              checked={draft.carry_forward_leave}
              onChange={(checked) => update("carry_forward_leave", checked)}
              label="Carry forward unused leave"
              description="Allow employees to carry unused leave days into the next year."
            />
            {draft.carry_forward_leave && (
              <NumberField
                label="Max carry-forward days"
                value={String(draft.max_carry_forward_days)}
                onChange={(value) => update("max_carry_forward_days", wholeNonNegative(value))}
                max={365}
              />
            )}
          </div>
        </Card>
      </div>
    </div>
  );
}
