"use client";

import { useState } from "react";
import { Check } from "lucide-react";
import { upsertManualAttendance } from "@/lib/db";
import type { AttendanceRecord, AttendanceStatus, Employee } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { useToast } from "@/components/ui/Toast";

const STATUS_OPTIONS: { value: AttendanceStatus; label: string }[] = [
  { value: "present", label: "Present" },
  { value: "late", label: "Late" },
  { value: "half_day", label: "Half day" },
  { value: "absent", label: "Absent" },
];

function toTimeString(ms?: number): string {
  if (!ms) return "";
  const date = new Date(ms);
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  return `${hours}:${minutes}`;
}

interface ManualAttendanceModalProps {
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
  employees: Employee[];
  defaultEmployeeId?: string;
  defaultDate: string;
  record?: AttendanceRecord | null;
}

/**
 * Field state is seeded once, from `useState` initialisers — callers must
 * pass a `key` that changes with the target (employee+date, or the record's
 * id) so opening it for a different day/employee remounts it fresh instead
 * of reusing stale state (same convention as `DesignationForm`/`ShiftForm`).
 */
export function ManualAttendanceModal({
  open,
  onClose,
  onSaved,
  employees,
  defaultEmployeeId,
  defaultDate,
  record,
}: Readonly<ManualAttendanceModalProps>) {
  const { showToast } = useToast();
  const [employeeId, setEmployeeId] = useState(
    record?.employee_id ?? defaultEmployeeId ?? "",
  );
  const [date, setDate] = useState(record?.date ?? defaultDate);
  const [clockIn, setClockIn] = useState(toTimeString(record?.clock_in));
  const [clockOut, setClockOut] = useState(toTimeString(record?.clock_out));
  const [status, setStatus] = useState<AttendanceStatus>(record?.status ?? "present");
  const [notes, setNotes] = useState(record?.notes ?? "");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  async function handleSubmit() {
    if (!employeeId) {
      setError("Pick an employee");
      return;
    }
    if (!date) {
      setError("Pick a date");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await upsertManualAttendance({
        employee_id: employeeId,
        date,
        clock_in: clockIn || undefined,
        clock_out: clockOut || undefined,
        status,
        notes: notes.trim() || undefined,
      });
      showToast("Attendance saved", "success");
      onSaved();
      onClose();
    } catch (submitError) {
      setError(
        submitError instanceof Error ? submitError.message : "Could not save attendance",
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="Manual attendance" size="sm">
      <form
        className="flex flex-col gap-4"
        onSubmit={(event) => {
          event.preventDefault();
          void handleSubmit();
        }}
      >
        {!defaultEmployeeId && (
          <Select
            label="Employee"
            placeholder="Please select"
            value={employeeId}
            onChange={(event) => setEmployeeId(event.target.value)}
            options={employees.map((employee) => ({
              value: employee.id,
              label: employee.full_name,
            }))}
          />
        )}
        <Input
          type="date"
          label="Date"
          value={date}
          onChange={(event) => setDate(event.target.value)}
        />
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            type="time"
            label="Clock In"
            value={clockIn}
            onChange={(event) => setClockIn(event.target.value)}
          />
          <Input
            type="time"
            label="Clock Out"
            value={clockOut}
            onChange={(event) => setClockOut(event.target.value)}
          />
        </div>
        <Select
          label="Status"
          value={status}
          onChange={(event) => setStatus(event.target.value as AttendanceStatus)}
          options={STATUS_OPTIONS}
        />
        <Textarea
          label="Notes"
          value={notes}
          onChange={(event) => setNotes(event.target.value)}
          rows={2}
        />
        {error && <p className="text-xs text-error">{error}</p>}
        <Button type="submit" loading={saving} className="self-start">
          <Check size={16} />
          Save
        </Button>
      </form>
    </Modal>
  );
}
