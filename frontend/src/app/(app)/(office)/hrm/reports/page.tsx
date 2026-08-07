"use client";

import { useCallback, useEffect, useState } from "react";
import { FileSpreadsheet, FileText } from "lucide-react";
import {
  listAttendanceInRange,
  listDesignations,
  listEmployees,
  listLeaveRequests,
  listLeaveTypes,
  listPayslips,
  presetToRange,
  type RangePreset,
} from "@/lib/db";
import type {
  AttendanceRecord,
  AttendanceStatus,
  Designation,
  Employee,
  LeaveRequest,
  LeaveStatus,
  Payslip,
} from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable, type DataColumn } from "@/components/ui/DataTable";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { useSettings } from "@/lib/hooks/use-settings";
import { exportExcel, exportPdf, type ExportColumn } from "@/lib/export";
import { ROUTES } from "@/lib/types/routes";

type HrmReportKey = "employees" | "attendance" | "leave" | "payroll";

const REPORTS: { value: HrmReportKey; label: string }[] = [
  { value: "employees", label: "Employee Report" },
  { value: "attendance", label: "Attendance Report" },
  { value: "leave", label: "Leave Report" },
  { value: "payroll", label: "Payroll Report" },
];

const RANGES: { value: RangePreset; label: string }[] = [
  { value: "today", label: "Today" },
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
  { value: "90d", label: "Last 90 days" },
  { value: "month", label: "This month" },
  { value: "year", label: "This year" },
  { value: "all", label: "All time" },
];

const STATUS_BADGE_VARIANT: Record<AttendanceStatus, "success" | "warning" | "danger"> = {
  present: "success",
  late: "warning",
  half_day: "warning",
  absent: "danger",
};

const LEAVE_BADGE_VARIANT: Record<LeaveStatus, "neutral" | "success" | "danger"> = {
  pending: "neutral",
  approved: "success",
  rejected: "danger",
};

function formatTime(ms?: number): string {
  if (!ms) return "—";
  return new Date(ms).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
}

function periodLabel(period: string): string {
  const [year, month] = period.split("-").map(Number);
  return new Date(year, month - 1, 1).toLocaleDateString(undefined, {
    month: "long",
    year: "numeric",
  });
}

// Each report supplies its own row type, so the page holds one discriminated
// bundle rather than four parallel state slots that can drift out of sync —
// same shape as the main app's `reports/page.tsx`.
type ReportData =
  | { key: "employees"; rows: Employee[] }
  | { key: "attendance"; rows: AttendanceRecord[] }
  | { key: "leave"; rows: LeaveRequest[] }
  | { key: "payroll"; rows: Payslip[] };

export default function HrmReportsPage() {
  const { money } = useSettings();
  const [report, setReport] = useState<HrmReportKey>("employees");
  const [preset, setPreset] = useState<RangePreset>("30d");
  const [data, setData] = useState<ReportData | null>(null);
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [designations, setDesignations] = useState<Designation[]>([]);
  const [leaveTypeNames, setLeaveTypeNames] = useState<Map<string, string>>(new Map());

  useEffect(() => {
    void Promise.all([listEmployees(), listDesignations(), listLeaveTypes()]).then(
      ([employeeList, designationList, leaveTypeList]) => {
        setEmployees(employeeList);
        setDesignations(designationList);
        setLeaveTypeNames(
          new Map(leaveTypeList.map((leaveType) => [leaveType.id, leaveType.name])),
        );
      },
    );
  }, []);

  const load = useCallback(async () => {
    const range = presetToRange(preset);
    switch (report) {
      case "employees":
        setData({ key: "employees", rows: await listEmployees() });
        break;
      case "attendance":
        setData({ key: "attendance", rows: await listAttendanceInRange(range) });
        break;
      case "leave": {
        const requests = await listLeaveRequests();
        setData({
          key: "leave",
          rows: requests.filter(
            (request) => request.requested_at >= range.from && request.requested_at <= range.to,
          ),
        });
        break;
      }
      case "payroll": {
        const payslips = await listPayslips();
        const fromPeriod = new Date(range.from).toISOString().slice(0, 7);
        const toPeriod = new Date(range.to).toISOString().slice(0, 7);
        setData({
          key: "payroll",
          rows: payslips.filter(
            (payslip) => payslip.period >= fromPeriod && payslip.period <= toPeriod,
          ),
        });
        break;
      }
    }
  }, [report, preset]);

  useEffect(() => {
    void load();
  }, [load]);

  const designationById = new Map(
    designations.map((designation) => [designation.id, designation.name]),
  );
  const employeeById = new Map(employees.map((employee) => [employee.id, employee.full_name]));

  function employeeName(employeeId: string): string {
    return employeeById.get(employeeId) ?? "—";
  }

  // Column definitions do double duty: the on-screen table and the exporters
  // read from the same shape, so an export can never drift from what's shown.
  function buildColumns(): {
    table: DataColumn<never>[];
    exports: ExportColumn<never>[];
  } {
    switch (data?.key) {
      case "employees": {
        const table: DataColumn<Employee>[] = [
          { key: "code", header: "Code", sortValue: (r) => r.employee_code, render: (r) => r.employee_code },
          { key: "name", header: "Name", sortValue: (r) => r.full_name, render: (r) => r.full_name },
          {
            key: "designation",
            header: "Designation",
            hideOnMobile: true,
            render: (r) => designationById.get(r.designation_id) ?? "—",
          },
          {
            key: "type",
            header: "Employment Type",
            hideOnMobile: true,
            render: (r) => r.employment_type.replace("_", " "),
          },
          { key: "joined", header: "Joining Date", sortValue: (r) => r.joining_date, render: (r) => r.joining_date },
          {
            key: "status",
            header: "Status",
            align: "right",
            render: (r) => <Badge variant={r.active ? "success" : "neutral"}>{r.active ? "Active" : "Inactive"}</Badge>,
          },
        ];
        const exports: ExportColumn<Employee>[] = [
          { key: "code", header: "Code", value: (r) => r.employee_code },
          { key: "name", header: "Name", value: (r) => r.full_name },
          { key: "designation", header: "Designation", value: (r) => designationById.get(r.designation_id) ?? "—" },
          { key: "type", header: "Employment Type", value: (r) => r.employment_type.replace("_", " ") },
          { key: "joined", header: "Joining Date", value: (r) => r.joining_date },
          { key: "status", header: "Status", value: (r) => (r.active ? "Active" : "Inactive") },
        ];
        return { table: table as DataColumn<never>[], exports: exports as ExportColumn<never>[] };
      }
      case "attendance": {
        const table: DataColumn<AttendanceRecord>[] = [
          { key: "date", header: "Date", sortValue: (r) => r.date, render: (r) => r.date },
          { key: "employee", header: "Employee", render: (r) => employeeName(r.employee_id) },
          {
            key: "clockIn",
            header: "Clock In",
            render: (r) => <span className="tabular-nums">{formatTime(r.clock_in)}</span>,
          },
          {
            key: "clockOut",
            header: "Clock Out",
            render: (r) => <span className="tabular-nums">{formatTime(r.clock_out)}</span>,
          },
          {
            key: "status",
            header: "Status",
            align: "right",
            render: (r) => (
              <Badge variant={STATUS_BADGE_VARIANT[r.status]}>{r.status.replace("_", " ")}</Badge>
            ),
          },
        ];
        const exports: ExportColumn<AttendanceRecord>[] = [
          { key: "date", header: "Date", value: (r) => r.date },
          { key: "employee", header: "Employee", value: (r) => employeeName(r.employee_id) },
          { key: "clockIn", header: "Clock In", value: (r) => formatTime(r.clock_in) },
          { key: "clockOut", header: "Clock Out", value: (r) => formatTime(r.clock_out) },
          { key: "status", header: "Status", value: (r) => r.status.replace("_", " ") },
        ];
        return { table: table as DataColumn<never>[], exports: exports as ExportColumn<never>[] };
      }
      case "leave": {
        const table: DataColumn<LeaveRequest>[] = [
          { key: "employee", header: "Employee", render: (r) => employeeName(r.employee_id) },
          { key: "type", header: "Leave Type", render: (r) => leaveTypeNames.get(r.leave_type_id) ?? "—" },
          {
            key: "dates",
            header: "Dates",
            hideOnMobile: true,
            render: (r) => (
              <span className="tabular-nums">
                {r.start_date} – {r.end_date}
              </span>
            ),
          },
          { key: "days", header: "Days", align: "right", sortValue: (r) => r.days, render: (r) => r.days },
          {
            key: "status",
            header: "Status",
            align: "right",
            render: (r) => (
              <Badge variant={LEAVE_BADGE_VARIANT[r.status]}>
                {r.status[0].toUpperCase() + r.status.slice(1)}
              </Badge>
            ),
          },
        ];
        const exports: ExportColumn<LeaveRequest>[] = [
          { key: "employee", header: "Employee", value: (r) => employeeName(r.employee_id) },
          { key: "type", header: "Leave Type", value: (r) => leaveTypeNames.get(r.leave_type_id) ?? "—" },
          { key: "dates", header: "Dates", value: (r) => `${r.start_date} - ${r.end_date}` },
          { key: "days", header: "Days", value: (r) => r.days },
          { key: "status", header: "Status", value: (r) => r.status },
        ];
        return { table: table as DataColumn<never>[], exports: exports as ExportColumn<never>[] };
      }
      case "payroll": {
        const table: DataColumn<Payslip>[] = [
          { key: "period", header: "Period", sortValue: (r) => r.period, render: (r) => periodLabel(r.period) },
          { key: "employee", header: "Employee", render: (r) => employeeName(r.employee_id) },
          {
            key: "net",
            header: "Net Pay",
            align: "right",
            sortValue: (r) => r.net_cents,
            render: (r) => <span className="font-semibold">{money(r.net_cents)}</span>,
          },
          {
            key: "status",
            header: "Status",
            align: "right",
            render: (r) => (
              <Badge variant={r.status === "paid" ? "success" : "neutral"}>
                {r.status === "paid" ? "Paid" : "Unpaid"}
              </Badge>
            ),
          },
        ];
        const exports: ExportColumn<Payslip>[] = [
          { key: "period", header: "Period", value: (r) => periodLabel(r.period) },
          { key: "employee", header: "Employee", value: (r) => employeeName(r.employee_id) },
          { key: "net", header: "Net Pay", value: (r) => money(r.net_cents) },
          { key: "status", header: "Status", value: (r) => (r.status === "paid" ? "Paid" : "Unpaid") },
        ];
        return { table: table as DataColumn<never>[], exports: exports as ExportColumn<never>[] };
      }
      default:
        return { table: [], exports: [] };
    }
  }

  const { table, exports } = buildColumns();
  const rows = (data?.rows ?? []) as never[];
  const reportLabel = REPORTS.find((entry) => entry.value === report)?.label ?? "Report";

  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Reports"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Reports" }]}
      />

      <Card className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-end gap-3">
            <Select
              label="Report"
              value={report}
              onChange={(event) => setReport(event.target.value as HrmReportKey)}
              options={REPORTS}
              className="min-w-48"
            />
            {report !== "employees" && (
              <Select
                label="Range"
                value={preset}
                onChange={(event) => setPreset(event.target.value as RangePreset)}
                options={RANGES}
                className="min-w-40"
              />
            )}
          </div>
          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={rows.length === 0}
              onClick={() => exportPdf(reportLabel, rows, exports)}
            >
              <FileText size={15} />
              PDF
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={rows.length === 0}
              onClick={() => exportExcel(report, reportLabel, rows, exports)}
            >
              <FileSpreadsheet size={15} />
              EXCEL
            </Button>
          </div>
        </div>

        <DataTable
          columns={table}
          rows={rows}
          rowKey={(row) => (row as { id: string }).id}
          emptyMessage="No data in this range."
          caption={reportLabel}
          pageSizeOptions={[10, 25, 50]}
        />
      </Card>
    </div>
  );
}
