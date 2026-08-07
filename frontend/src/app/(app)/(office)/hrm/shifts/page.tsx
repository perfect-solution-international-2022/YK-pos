"use client";

import { useEffect, useState } from "react";
import { Check, Pencil, Plus, Search, X } from "lucide-react";
import { createShift, deleteShift, listShifts, logAudit, updateShift } from "@/lib/db";
import type { Shift } from "@/lib/types";
import { useAuth } from "@/lib/hooks/use-auth";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DataTable, type DataColumn } from "@/components/ui/DataTable";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { NumberField } from "@/components/ui/NumberField";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { Switch } from "@/components/ui/Switch";
import { useToast } from "@/components/ui/Toast";
import { ROUTES } from "@/lib/types/routes";

function RequiredMark() {
  return (
    <span className="text-error" aria-hidden>
      {" "}
      *
    </span>
  );
}

const ACTION_BUTTON_CLASSES =
  "inline-flex h-8 w-8 items-center justify-center rounded-md border border-outline-variant transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:scale-105 active:scale-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary dark:border-zinc-700";

const DAY_LABELS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

function formatWeeklyOff(weeklyOff: number[]): string {
  if (weeklyOff.length === 0) return "None";
  return [...weeklyOff]
    .sort((a, b) => a - b)
    .map((day) => DAY_LABELS[day])
    .join(", ");
}

function filterShifts(shifts: Shift[], query: string): Shift[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return shifts;
  return shifts.filter((shift) => shift.name.toLowerCase().includes(needle));
}

interface ShiftFormProps {
  shift: Shift | null;
  onClose: () => void;
  onSaved: () => void;
}

function ShiftForm({ shift, onClose, onSaved }: Readonly<ShiftFormProps>) {
  const { showToast } = useToast();
  const { staff } = useAuth();
  const [name, setName] = useState(shift?.name ?? "");
  const [startTime, setStartTime] = useState(shift?.start_time ?? "09:00");
  const [endTime, setEndTime] = useState(shift?.end_time ?? "18:00");
  const [breakMinutes, setBreakMinutes] = useState(String(shift?.break_minutes ?? 60));
  const [graceMinutes, setGraceMinutes] = useState(String(shift?.grace_minutes ?? 10));
  const [weeklyOff, setWeeklyOff] = useState<number[]>(shift?.weekly_off ?? [0]);
  const [active, setActive] = useState(shift?.active ?? true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  function toggleDay(day: number) {
    setWeeklyOff((current) =>
      current.includes(day) ? current.filter((entry) => entry !== day) : [...current, day],
    );
  }

  async function handleSubmit() {
    if (!name.trim()) {
      setError("Shift name is required");
      return;
    }
    if (!startTime || !endTime) {
      setError("Start and end time are required");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const payload = {
        name,
        start_time: startTime,
        end_time: endTime,
        break_minutes: Number(breakMinutes) || 0,
        grace_minutes: Number(graceMinutes) || 0,
        weekly_off: weeklyOff,
        active,
      };
      if (shift) {
        await updateShift(shift.id, payload);
        await logAudit({
          actor_id: staff?.id,
          actor_name: staff?.name ?? "System",
          action: "shift.update",
          resource: "shift",
          resource_id: shift.id,
          resource_label: name,
        });
        showToast(`${name} updated`, "success");
      } else {
        const created = await createShift(payload);
        await logAudit({
          actor_id: staff?.id,
          actor_name: staff?.name ?? "System",
          action: "shift.create",
          resource: "shift",
          resource_id: created.id,
          resource_label: name,
        });
        showToast(`${name} added`, "success");
      }
      onSaved();
      onClose();
    } catch (submitError) {
      setError(
        submitError instanceof Error ? submitError.message : "Could not save shift",
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        event.preventDefault();
        void handleSubmit();
      }}
    >
      <Input
        label={
          <>
            Shift Name
            <RequiredMark />
          </>
        }
        placeholder="e.g. Morning Shift"
        autoFocus
        value={name}
        onChange={(event) => setName(event.target.value)}
      />
      <div className="grid gap-4 sm:grid-cols-2">
        <Input
          type="time"
          label={
            <>
              Start Time
              <RequiredMark />
            </>
          }
          value={startTime}
          onChange={(event) => setStartTime(event.target.value)}
        />
        <Input
          type="time"
          label={
            <>
              End Time
              <RequiredMark />
            </>
          }
          value={endTime}
          onChange={(event) => setEndTime(event.target.value)}
        />
        <NumberField
          label="Break Time"
          value={breakMinutes}
          onChange={setBreakMinutes}
          suffix="min"
          max={480}
        />
        <NumberField
          label="Grace Time"
          value={graceMinutes}
          onChange={setGraceMinutes}
          suffix="min"
          max={120}
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <span className="text-sm font-medium text-on-surface-variant dark:text-zinc-300">
          Weekly Off
        </span>
        <div className="flex flex-wrap gap-1.5">
          {DAY_LABELS.map((label, day) => {
            const selected = weeklyOff.includes(day);
            return (
              <button
                key={label}
                type="button"
                onClick={() => toggleDay(day)}
                aria-pressed={selected}
                className={`min-h-9 min-w-11 rounded-lg border px-2.5 text-xs font-medium transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] active:scale-95 ${
                  selected
                    ? "border-secondary bg-secondary text-on-secondary dark:border-green-600 dark:bg-green-600 dark:text-white"
                    : "border-outline-variant text-on-surface-variant hover:bg-surface-container dark:border-zinc-700 dark:text-zinc-400 dark:hover:bg-zinc-800"
                }`}
              >
                {label}
              </button>
            );
          })}
        </div>
      </div>

      <Switch checked={active} onChange={setActive} label="Active" />

      {error && <p className="text-xs text-error">{error}</p>}
      <Button type="submit" loading={saving} className="self-start">
        <Check size={16} />
        {shift ? "Save changes" : "Submit"}
      </Button>
    </form>
  );
}

interface ShiftFormModalProps {
  open: boolean;
  shift: Shift | null;
  onClose: () => void;
  onSaved: () => void;
}

function ShiftFormModal({ open, shift, onClose, onSaved }: Readonly<ShiftFormModalProps>) {
  return (
    <Modal open={open} onClose={onClose} title={shift ? "Edit" : "Create"} size="md">
      <ShiftForm key={shift?.id ?? "new"} shift={shift} onClose={onClose} onSaved={onSaved} />
    </Modal>
  );
}

export default function ShiftsPage() {
  const { showToast } = useToast();
  const { staff } = useAuth();

  const [query, setQuery] = useState("");
  const [shifts, setShifts] = useState<Shift[]>([]);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Shift | null>(null);
  const [pendingDelete, setPendingDelete] = useState<Shift | null>(null);
  const [deleting, setDeleting] = useState(false);

  function reload() {
    listShifts().then(setShifts);
  }

  useEffect(reload, []);

  const visible = filterShifts(shifts, query);

  async function confirmDelete() {
    if (!pendingDelete) return;
    setDeleting(true);
    try {
      await deleteShift(pendingDelete.id);
      await logAudit({
        actor_id: staff?.id,
        actor_name: staff?.name ?? "System",
        action: "shift.delete",
        resource: "shift",
        resource_id: pendingDelete.id,
        resource_label: pendingDelete.name,
      });
      showToast(`Deleted ${pendingDelete.name}`, "success");
      setSelectedIds((current) => current.filter((id) => id !== pendingDelete.id));
      setPendingDelete(null);
      reload();
    } catch (error) {
      showToast(error instanceof Error ? error.message : "Could not delete shift", "error");
    } finally {
      setDeleting(false);
    }
  }

  const columns: DataColumn<Shift>[] = [
    {
      key: "name",
      header: "Shift Name",
      sortValue: (shift) => shift.name,
      render: (shift) => (
        <span className="font-medium text-on-surface dark:text-zinc-50">{shift.name}</span>
      ),
    },
    {
      key: "time",
      header: "Time",
      render: (shift) => (
        <span className="tabular-nums text-on-surface-variant dark:text-zinc-400">
          {shift.start_time} – {shift.end_time}
        </span>
      ),
    },
    {
      key: "break",
      header: "Break / Grace",
      hideOnMobile: true,
      render: (shift) => (
        <span className="tabular-nums text-on-surface-variant dark:text-zinc-400">
          {shift.break_minutes}m / {shift.grace_minutes}m
        </span>
      ),
    },
    {
      key: "weeklyOff",
      header: "Weekly Off",
      hideOnMobile: true,
      render: (shift) => formatWeeklyOff(shift.weekly_off),
    },
    {
      key: "status",
      header: "Status",
      align: "right",
      render: (shift) => (
        <Badge variant={shift.active ? "success" : "neutral"}>
          {shift.active ? "Active" : "Inactive"}
        </Badge>
      ),
    },
    {
      key: "action",
      header: "Action",
      align: "right",
      render: (shift) => (
        <span className="flex items-center justify-end gap-1.5">
          <button
            type="button"
            onClick={() => {
              setEditing(shift);
              setFormOpen(true);
            }}
            aria-label={`Edit ${shift.name}`}
            title="Edit"
            className={`${ACTION_BUTTON_CLASSES} text-emerald-600 hover:bg-surface-container dark:text-emerald-400 dark:hover:bg-zinc-800`}
          >
            <Pencil size={15} aria-hidden />
          </button>
          <button
            type="button"
            onClick={() => setPendingDelete(shift)}
            aria-label={`Delete ${shift.name}`}
            title="Delete"
            className={`${ACTION_BUTTON_CLASSES} text-error hover:bg-surface-container dark:text-red-400 dark:hover:bg-zinc-800`}
          >
            <X size={15} aria-hidden />
          </button>
        </span>
      ),
    },
  ];

  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Shift Management"
        breadcrumbs={[
          { label: "HRM", href: ROUTES.hrm.dashboard },
          { label: "Shift Management" },
        ]}
      />

      <Card>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="relative w-full sm:w-72">
            <Search
              size={16}
              aria-hidden
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-on-surface-variant dark:text-zinc-500"
            />
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search this table"
              aria-label="Search shifts"
              className="pl-9"
            />
          </div>
          <Button
            type="button"
            onClick={() => {
              setEditing(null);
              setFormOpen(true);
            }}
          >
            <Plus size={16} />
            Create
          </Button>
        </div>

        <div className="mt-4">
          <DataTable
            columns={columns}
            rows={visible}
            rowKey={(shift) => shift.id}
            emptyMessage="No shifts yet."
            caption="Shifts"
            pageSizeOptions={[10, 25, 50]}
            selection={{ selectedIds, onChange: setSelectedIds }}
          />
        </div>
      </Card>

      <ShiftFormModal
        open={formOpen}
        shift={editing}
        onClose={() => setFormOpen(false)}
        onSaved={reload}
      />

      <ConfirmDialog
        open={pendingDelete !== null}
        title="Delete shift"
        message={`Delete ${pendingDelete?.name ?? "this shift"}? Employees already assigned to it keep their existing record.`}
        confirmLabel="Delete"
        destructive
        busy={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </div>
  );
}
