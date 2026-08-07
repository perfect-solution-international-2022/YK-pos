"use client";

import { useEffect, useMemo, useState } from "react";
import Image from "next/image";
import {
  ChevronLeft,
  ChevronRight,
  FileSpreadsheet,
  FileText,
  LogIn,
  LogOut,
  Pencil,
  User,
} from "lucide-react";
import {
  activeLeaveToday,
  clockIn,
  clockOut,
  listAttendanceForDate,
  listAttendanceInRange,
  listDesignations,
  listEmployees,
  listLeaveRequests,
  listShifts,
  localDateKey,
  presetToRange,
  type RangePreset,
} from "@/lib/db";
import type {
  AttendanceRecord,
  AttendanceStatus,
  Designation,
  Employee,
  Shift,
} from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable, type DataColumn } from "@/components/ui/DataTable";
import { EmptyState } from "@/components/ui/EmptyState";
import { exportExcel, exportPdf, type ExportColumn } from "@/lib/export";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { useToast } from "@/components/ui/Toast";
import { ManualAttendanceModal } from "./components/ManualAttendanceModal";
import { ROUTES } from "@/lib/types/routes";

type View = "daily" | "calendar" | "report";

const STATUS_LABELS: Record<AttendanceStatus, string> = {
  present: "Present",
  late: "Late",
  half_day: "Half day",
  absent: "Absent",
};

const STATUS_BADGE_VARIANT: Record<AttendanceStatus, "success" | "warning" | "danger"> = {
  present: "success",
  late: "warning",
  half_day: "warning",
  absent: "danger",
};

const RANGE_OPTIONS: { value: RangePreset; label: string }[] = [
  { value: "today", label: "Today" },
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
  { value: "month", label: "This month" },
];

function formatTime(ms?: number): string {
  if (!ms) return "—";
  return new Date(ms).toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
  });
}

interface SharedData {
  employees: Employee[];
  designations: Designation[];
  shifts: Shift[];
}

function useHrmReferenceData(): SharedData {
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [designations, setDesignations] = useState<Designation[]>([]);
  const [shifts, setShifts] = useState<Shift[]>([]);

  useEffect(() => {
    void (async () => {
      const [employeeList, designationList, shiftList] = await Promise.all([
        listEmployees(),
        listDesignations(),
        listShifts(),
      ]);
      setEmployees(employeeList.filter((employee) => employee.active));
      setDesignations(designationList);
      setShifts(shiftList);
    })();
  }, []);

  return { employees, designations, shifts };
}

function DailyView({ employees, designations, shifts }: Readonly<SharedData>) {
  const { showToast } = useToast();
  const [today, setToday] = useState("");
  const [records, setRecords] = useState<AttendanceRecord[]>([]);
  const [onLeaveIds, setOnLeaveIds] = useState<Set<string>>(new Set());
  const [editTarget, setEditTarget] = useState<{
    employeeId: string;
    record: AttendanceRecord | null;
  } | null>(null);

  const designationById = useMemo(
    () => new Map(designations.map((designation) => [designation.id, designation.name])),
    [designations],
  );
  const shiftById = useMemo(() => new Map(shifts.map((shift) => [shift.id, shift])), [shifts]);

  async function refresh() {
    const date = localDateKey(Date.now());
    setToday(date);
    const [recordList, leaveRequests] = await Promise.all([
      listAttendanceForDate(date),
      listLeaveRequests(),
    ]);
    setRecords(recordList);
    setOnLeaveIds(activeLeaveToday(leaveRequests, date));
  }

  useEffect(() => {
    const timerId = window.setTimeout(() => void refresh(), 0);
    return () => window.clearTimeout(timerId);
  }, []);

  const recordByEmployee = useMemo(
    () => new Map(records.map((record) => [record.employee_id, record])),
    [records],
  );

  async function handleClockIn(employee: Employee) {
    try {
      await clockIn(employee.id, shiftById.get(employee.shift_id));
      showToast(`${employee.full_name} clocked in`, "success");
      await refresh();
    } catch (error) {
      showToast(error instanceof Error ? error.message : "Could not clock in", "error");
    }
  }

  async function handleClockOut(employee: Employee) {
    try {
      await clockOut(employee.id);
      showToast(`${employee.full_name} clocked out`, "success");
      await refresh();
    } catch (error) {
      showToast(error instanceof Error ? error.message : "Could not clock out", "error");
    }
  }

  const columns: DataColumn<Employee>[] = [
    {
      key: "photo",
      header: "",
      render: (employee) =>
        employee.photo ? (
          <Image
            src={employee.photo}
            alt=""
            width={32}
            height={32}
            unoptimized
            className="h-8 w-8 rounded-full object-cover"
          />
        ) : (
          <span className="flex h-8 w-8 items-center justify-center rounded-full bg-surface-container text-on-surface-variant dark:bg-zinc-800 dark:text-zinc-400">
            <User size={14} />
          </span>
        ),
    },
    {
      key: "name",
      header: "Name",
      sortValue: (employee) => employee.full_name,
      render: (employee) => (
        <span className="block min-w-28 max-w-48 truncate font-medium">
          {employee.full_name}
        </span>
      ),
    },
    {
      key: "designation",
      header: "Designation",
      hideOnMobile: true,
      render: (employee) => designationById.get(employee.designation_id) ?? "—",
    },
    {
      key: "clockIn",
      header: "Clock In",
      render: (employee) => (
        <span className="tabular-nums">
          {formatTime(recordByEmployee.get(employee.id)?.clock_in)}
        </span>
      ),
    },
    {
      key: "clockOut",
      header: "Clock Out",
      render: (employee) => (
        <span className="tabular-nums">
          {formatTime(recordByEmployee.get(employee.id)?.clock_out)}
        </span>
      ),
    },
    {
      key: "status",
      header: "Status",
      render: (employee) => {
        if (onLeaveIds.has(employee.id)) return <Badge variant="neutral">On leave</Badge>;
        const record = recordByEmployee.get(employee.id);
        if (!record) return <Badge variant="neutral">Not clocked in</Badge>;
        return (
          <Badge variant={STATUS_BADGE_VARIANT[record.status]}>
            {STATUS_LABELS[record.status]}
          </Badge>
        );
      },
    },
    {
      key: "actions",
      header: "Action",
      align: "right",
      render: (employee) => {
        const record = recordByEmployee.get(employee.id);
        const onLeave = onLeaveIds.has(employee.id);
        return (
          <span className="flex items-center justify-end gap-1.5">
            {!record?.clock_in ? (
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={onLeave}
                onClick={() => void handleClockIn(employee)}
              >
                <LogIn size={13} />
                In
              </Button>
            ) : !record.clock_out ? (
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => void handleClockOut(employee)}
              >
                <LogOut size={13} />
                Out
              </Button>
            ) : null}
            <button
              type="button"
              aria-label={`Edit ${employee.full_name}'s attendance`}
              title="Edit"
              onClick={() => setEditTarget({ employeeId: employee.id, record: record ?? null })}
              className="flex h-8 w-8 items-center justify-center rounded-lg border border-outline-variant text-on-surface-variant transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-surface-container active:scale-90 dark:border-zinc-700 dark:text-zinc-400 dark:hover:bg-zinc-800"
            >
              <Pencil size={13} />
            </button>
          </span>
        );
      },
    },
  ];

  return (
    <Card>
      <DataTable
        columns={columns}
        rows={employees}
        rowKey={(employee) => employee.id}
        emptyMessage="No active employees yet."
        caption="Today's attendance"
        pageSizeOptions={[10, 25, 50]}
      />
      {editTarget && (
        <ManualAttendanceModal
          key={`${editTarget.employeeId}-${today}`}
          open
          onClose={() => setEditTarget(null)}
          onSaved={refresh}
          employees={employees}
          defaultEmployeeId={editTarget.employeeId}
          defaultDate={today}
          record={editTarget.record}
        />
      )}
    </Card>
  );
}

function CalendarView({ employees, shifts }: Readonly<SharedData>) {
  const [selectedEmployeeId, setSelectedEmployeeId] = useState("");
  const employeeId = selectedEmployeeId || employees[0]?.id || "";
  const [monthCursor, setMonthCursor] = useState(() => {
    const now = new Date();
    return new Date(now.getFullYear(), now.getMonth(), 1);
  });
  const [records, setRecords] = useState<AttendanceRecord[]>([]);
  const [editDate, setEditDate] = useState<string | null>(null);

  const monthLabel = monthCursor.toLocaleDateString(undefined, {
    month: "long",
    year: "numeric",
  });

  async function refresh() {
    if (!employeeId) return;
    const from = monthCursor.getTime();
    const to = new Date(monthCursor.getFullYear(), monthCursor.getMonth() + 1, 0, 23, 59, 59, 999).getTime();
    const monthRecords = await listAttendanceInRange({ from, to });
    setRecords(monthRecords.filter((record) => record.employee_id === employeeId));
  }

  useEffect(() => {
    const timerId = window.setTimeout(() => void refresh(), 0);
    return () => window.clearTimeout(timerId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [employeeId, monthCursor]);

  const recordByDate = useMemo(
    () => new Map(records.map((record) => [record.date, record])),
    [records],
  );

  const employee = employees.find((entry) => entry.id === employeeId);
  const shift = employee ? shifts.find((entry) => entry.id === employee.shift_id) : undefined;

  const firstWeekday = monthCursor.getDay();
  const daysInMonth = new Date(monthCursor.getFullYear(), monthCursor.getMonth() + 1, 0).getDate();
  const cells: (number | string)[] = [
    ...Array.from({ length: firstWeekday }, (_, offset) => `blank-${offset}`),
    ...Array.from({ length: daysInMonth }, (_, index) => index + 1),
  ];

  return (
    <Card className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Select
          label="Employee"
          value={employeeId}
          onChange={(event) => setSelectedEmployeeId(event.target.value)}
          options={employees.map((entry) => ({ value: entry.id, label: entry.full_name }))}
          className="min-w-48"
        />
        <div className="flex items-center gap-2">
          <button
            type="button"
            aria-label="Previous month"
            onClick={() =>
              setMonthCursor((cursor) => new Date(cursor.getFullYear(), cursor.getMonth() - 1, 1))
            }
            className="flex h-9 w-9 items-center justify-center rounded-lg border border-outline-variant text-on-surface-variant transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-surface-container dark:border-zinc-700 dark:text-zinc-400 dark:hover:bg-zinc-800"
          >
            <ChevronLeft size={16} />
          </button>
          <span className="min-w-32 text-center text-sm font-medium text-on-surface dark:text-zinc-50">
            {monthLabel}
          </span>
          <button
            type="button"
            aria-label="Next month"
            onClick={() =>
              setMonthCursor((cursor) => new Date(cursor.getFullYear(), cursor.getMonth() + 1, 1))
            }
            className="flex h-9 w-9 items-center justify-center rounded-lg border border-outline-variant text-on-surface-variant transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-surface-container dark:border-zinc-700 dark:text-zinc-400 dark:hover:bg-zinc-800"
          >
            <ChevronRight size={16} />
          </button>
        </div>
      </div>

      {!employee ? (
        <EmptyState icon={<User size={20} />} title="No employees yet" />
      ) : (
        <>
          <div className="grid grid-cols-7 gap-1.5 text-center text-xs font-semibold text-on-surface-variant dark:text-zinc-500">
            {["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"].map((label) => (
              <span key={label}>{label}</span>
            ))}
          </div>
          <div className="grid grid-cols-7 gap-1.5">
            {cells.map((cell) => {
              if (typeof cell === "string") return <span key={cell} />;
              const day = cell;
              const dateKey = `${monthCursor.getFullYear()}-${String(monthCursor.getMonth() + 1).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
              const record = recordByDate.get(dateKey);
              const weekday = new Date(monthCursor.getFullYear(), monthCursor.getMonth(), day).getDay();
              const isWeeklyOff = shift?.weekly_off.includes(weekday) ?? false;
              const variant = record ? STATUS_BADGE_VARIANT[record.status] : undefined;
              let toneClass =
                "text-on-surface-variant hover:bg-surface-container dark:text-zinc-400 dark:hover:bg-zinc-800";
              if (isWeeklyOff) {
                toneClass =
                  "bg-surface-container text-on-surface-variant/50 dark:bg-zinc-800/60 dark:text-zinc-600";
              }
              if (record) {
                if (variant === "success") {
                  toneClass =
                    "bg-[#0d7a3d]/15 text-[#0d7a3d] dark:bg-green-900/30 dark:text-green-400";
                } else if (variant === "warning") {
                  toneClass =
                    "bg-amber-500/15 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400";
                } else {
                  toneClass =
                    "bg-error/10 text-error dark:bg-red-900/30 dark:text-red-400";
                }
              }
              return (
                <button
                  key={dateKey}
                  type="button"
                  onClick={() => setEditDate(dateKey)}
                  className={`flex aspect-square flex-col items-center justify-center rounded-lg text-xs font-medium transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:scale-105 active:scale-95 ${toneClass}`}
                >
                  {day}
                </button>
              );
            })}
          </div>
          <div className="flex flex-wrap gap-3 text-xs text-on-surface-variant dark:text-zinc-500">
            <span className="flex items-center gap-1.5">
              <span className="h-2.5 w-2.5 rounded-full bg-[#0d7a3d]" /> Present / Late
            </span>
            <span className="flex items-center gap-1.5">
              <span className="h-2.5 w-2.5 rounded-full bg-error" /> Absent
            </span>
            <span className="flex items-center gap-1.5">
              <span className="h-2.5 w-2.5 rounded-full bg-surface-container-high dark:bg-zinc-700" />{" "}
              Weekly off
            </span>
          </div>
        </>
      )}

      {editDate && employee && (
        <ManualAttendanceModal
          key={`${employee.id}-${editDate}`}
          open
          onClose={() => setEditDate(null)}
          onSaved={refresh}
          employees={employees}
          defaultEmployeeId={employee.id}
          defaultDate={editDate}
          record={recordByDate.get(editDate) ?? null}
        />
      )}
    </Card>
  );
}

function ReportView({ employees, designations }: Readonly<SharedData>) {
  const [preset, setPreset] = useState<RangePreset>("30d");
  const [designationFilter, setDesignationFilter] = useState("all");
  const [records, setRecords] = useState<AttendanceRecord[]>([]);

  const employeeById = useMemo(
    () => new Map(employees.map((employee) => [employee.id, employee])),
    [employees],
  );
  const designationById = useMemo(
    () => new Map(designations.map((designation) => [designation.id, designation.name])),
    [designations],
  );

  useEffect(() => {
    void listAttendanceInRange(presetToRange(preset)).then(setRecords);
  }, [preset]);

  const visible = records.filter((record) => {
    if (designationFilter === "all") return true;
    return employeeById.get(record.employee_id)?.designation_id === designationFilter;
  });

  const exportColumns: ExportColumn<AttendanceRecord>[] = [
    { key: "date", header: "Date", value: (record) => record.date },
    {
      key: "employee",
      header: "Employee",
      value: (record) => employeeById.get(record.employee_id)?.full_name ?? "—",
    },
    {
      key: "designation",
      header: "Designation",
      value: (record) => {
        const employee = employeeById.get(record.employee_id);
        return employee ? designationById.get(employee.designation_id) ?? "—" : "—";
      },
    },
    { key: "clockIn", header: "Clock In", value: (record) => formatTime(record.clock_in) },
    { key: "clockOut", header: "Clock Out", value: (record) => formatTime(record.clock_out) },
    { key: "status", header: "Status", value: (record) => STATUS_LABELS[record.status] },
  ];

  const columns: DataColumn<AttendanceRecord>[] = [
    { key: "date", header: "Date", sortValue: (record) => record.date, render: (record) => record.date },
    {
      key: "employee",
      header: "Employee",
      render: (record) => employeeById.get(record.employee_id)?.full_name ?? "—",
    },
    {
      key: "designation",
      header: "Designation",
      hideOnMobile: true,
      render: (record) => {
        const employee = employeeById.get(record.employee_id);
        return employee ? designationById.get(employee.designation_id) ?? "—" : "—";
      },
    },
    {
      key: "clockIn",
      header: "Clock In",
      render: (record) => <span className="tabular-nums">{formatTime(record.clock_in)}</span>,
    },
    {
      key: "clockOut",
      header: "Clock Out",
      render: (record) => <span className="tabular-nums">{formatTime(record.clock_out)}</span>,
    },
    {
      key: "status",
      header: "Status",
      align: "right",
      render: (record) => (
        <Badge variant={STATUS_BADGE_VARIANT[record.status]}>{STATUS_LABELS[record.status]}</Badge>
      ),
    },
  ];

  return (
    <Card className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-end gap-3">
          <Select
            label="Range"
            value={preset}
            onChange={(event) => setPreset(event.target.value as RangePreset)}
            options={RANGE_OPTIONS}
            className="min-w-40"
          />
          <Select
            label="Designation"
            value={designationFilter}
            onChange={(event) => setDesignationFilter(event.target.value)}
            options={[
              { value: "all", label: "All designations" },
              ...designations.map((designation) => ({
                value: designation.id,
                label: designation.name,
              })),
            ]}
            className="min-w-40"
          />
        </div>
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => exportPdf("Attendance report", visible, exportColumns)}
          >
            <FileText size={15} />
            PDF
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => exportExcel("attendance", "Attendance report", visible, exportColumns)}
          >
            <FileSpreadsheet size={15} />
            EXCEL
          </Button>
        </div>
      </div>
      <DataTable
        columns={columns}
        rows={visible}
        rowKey={(record) => record.id}
        emptyMessage="No attendance records in this range."
        caption="Attendance report"
        pageSizeOptions={[10, 25, 50]}
      />
    </Card>
  );
}

export default function AttendancePage() {
  const [view, setView] = useState<View>("daily");
  const data = useHrmReferenceData();

  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Attendance"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Attendance" }]}
      />

      <div className="flex w-fit gap-1 rounded-lg bg-surface-container p-0.5 dark:bg-zinc-800">
        {(
          [
            { value: "daily", label: "Daily" },
            { value: "calendar", label: "Calendar" },
            { value: "report", label: "Report" },
          ] as const
        ).map((tab) => (
          <button
            key={tab.value}
            type="button"
            onClick={() => setView(tab.value)}
            className={`rounded-md px-3 py-1.5 text-sm font-medium transition-all duration-[var(--duration-fast)] ${
              view === tab.value
                ? "bg-surface-container-lowest text-on-surface shadow-sm dark:bg-zinc-900 dark:text-zinc-50"
                : "text-on-surface-variant hover:text-on-surface dark:text-zinc-400 dark:hover:text-zinc-100"
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {view === "daily" && <DailyView {...data} />}
      {view === "calendar" && <CalendarView {...data} />}
      {view === "report" && <ReportView {...data} />}
    </div>
  );
}
