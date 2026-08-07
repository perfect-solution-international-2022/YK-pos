"use client";

import { useEffect, useMemo, useState } from "react";
import { Check, Pencil, Plus, X } from "lucide-react";
import {
  createLeaveRequest,
  createLeaveType,
  decideLeaveRequest,
  deleteLeaveType,
  leaveBalance,
  listEmployees,
  listLeaveRequests,
  listLeaveTypes,
  updateLeaveType,
} from "@/lib/db";
import type { Employee, LeaveRequest, LeaveStatus, LeaveType } from "@/lib/types";
import { useAuth } from "@/lib/hooks/use-auth";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DataTable, type DataColumn } from "@/components/ui/DataTable";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { NumberField } from "@/components/ui/NumberField";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { Switch } from "@/components/ui/Switch";
import { Textarea } from "@/components/ui/Textarea";
import { useToast } from "@/components/ui/Toast";
import { ROUTES } from "@/lib/types/routes";

type View = "requests" | "types" | "balance";

const STATUS_BADGE_VARIANT: Record<LeaveStatus, "neutral" | "success" | "danger"> = {
  pending: "neutral",
  approved: "success",
  rejected: "danger",
};

const CURRENT_YEAR = new Date().getFullYear();

interface RequestFormProps {
  employees: Employee[];
  leaveTypes: LeaveType[];
  onClose: () => void;
  onSaved: () => void;
}

function RequestForm({ employees, leaveTypes, onClose, onSaved }: Readonly<RequestFormProps>) {
  const { showToast } = useToast();
  const [employeeId, setEmployeeId] = useState("");
  const [leaveTypeId, setLeaveTypeId] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  async function handleSubmit() {
    if (!employeeId || !leaveTypeId || !startDate || !endDate) {
      setError("Fill in every required field");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await createLeaveRequest({
        employee_id: employeeId,
        leave_type_id: leaveTypeId,
        start_date: startDate,
        end_date: endDate,
        reason: reason.trim() || undefined,
      });
      showToast("Leave request submitted", "success");
      onSaved();
      onClose();
    } catch (submitError) {
      setError(
        submitError instanceof Error ? submitError.message : "Could not submit request",
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
      <Select
        label="Employee"
        placeholder="Please select"
        value={employeeId}
        onChange={(event) => setEmployeeId(event.target.value)}
        options={employees.map((employee) => ({ value: employee.id, label: employee.full_name }))}
      />
      <Select
        label="Leave Type"
        placeholder="Please select"
        value={leaveTypeId}
        onChange={(event) => setLeaveTypeId(event.target.value)}
        options={leaveTypes.map((leaveType) => ({ value: leaveType.id, label: leaveType.name }))}
      />
      <div className="grid gap-4 sm:grid-cols-2">
        <Input
          type="date"
          label="Start Date"
          value={startDate}
          onChange={(event) => setStartDate(event.target.value)}
        />
        <Input
          type="date"
          label="End Date"
          value={endDate}
          onChange={(event) => setEndDate(event.target.value)}
        />
      </div>
      <Textarea
        label="Reason"
        value={reason}
        onChange={(event) => setReason(event.target.value)}
        rows={3}
      />
      {error && <p className="text-xs text-error">{error}</p>}
      <Button type="submit" loading={saving} className="self-start">
        <Check size={16} />
        Submit
      </Button>
    </form>
  );
}

function RequestsView({ employees }: Readonly<{ employees: Employee[] }>) {
  const { showToast } = useToast();
  const { staff } = useAuth();
  const [requests, setRequests] = useState<LeaveRequest[]>([]);
  const [leaveTypes, setLeaveTypes] = useState<LeaveType[]>([]);
  const [statusFilter, setStatusFilter] = useState<"all" | LeaveStatus>("all");
  const [employeeFilter, setEmployeeFilter] = useState("all");
  const [formOpen, setFormOpen] = useState(false);

  const employeeById = useMemo(
    () => new Map(employees.map((employee) => [employee.id, employee.full_name])),
    [employees],
  );
  const leaveTypeById = useMemo(
    () => new Map(leaveTypes.map((leaveType) => [leaveType.id, leaveType.name])),
    [leaveTypes],
  );

  async function refresh() {
    const [requestList, leaveTypeList] = await Promise.all([listLeaveRequests(), listLeaveTypes()]);
    setRequests(requestList);
    setLeaveTypes(leaveTypeList);
  }

  useEffect(() => {
    const timerId = window.setTimeout(() => void refresh(), 0);
    return () => window.clearTimeout(timerId);
  }, []);

  const visible = requests.filter((request) => {
    if (statusFilter !== "all" && request.status !== statusFilter) return false;
    if (employeeFilter !== "all" && request.employee_id !== employeeFilter) return false;
    return true;
  });

  async function handleDecide(request: LeaveRequest, status: "approved" | "rejected") {
    try {
      await decideLeaveRequest(request.id, status, staff?.id ?? "");
      showToast(`Request ${status}`, "success");
      await refresh();
    } catch (error) {
      showToast(error instanceof Error ? error.message : "Could not update request", "error");
    }
  }

  const columns: DataColumn<LeaveRequest>[] = [
    {
      key: "employee",
      header: "Employee",
      render: (request) => employeeById.get(request.employee_id) ?? "—",
    },
    {
      key: "type",
      header: "Leave Type",
      render: (request) => leaveTypeById.get(request.leave_type_id) ?? "—",
    },
    {
      key: "dates",
      header: "Dates",
      hideOnMobile: true,
      render: (request) => (
        <span className="tabular-nums">
          {request.start_date} – {request.end_date}
        </span>
      ),
    },
    { key: "days", header: "Days", align: "right", render: (request) => request.days },
    {
      key: "reason",
      header: "Reason",
      hideOnMobile: true,
      render: (request) => (
        <span className="block max-w-48 truncate">{request.reason || "—"}</span>
      ),
    },
    {
      key: "status",
      header: "Status",
      render: (request) => (
        <Badge variant={STATUS_BADGE_VARIANT[request.status]}>
          {request.status[0].toUpperCase() + request.status.slice(1)}
        </Badge>
      ),
    },
    {
      key: "actions",
      header: "Action",
      align: "right",
      render: (request) =>
        request.status === "pending" ? (
          <span className="flex items-center justify-end gap-1.5">
            <button
              type="button"
              aria-label="Approve"
              title="Approve"
              onClick={() => void handleDecide(request, "approved")}
              className="flex h-8 w-8 items-center justify-center rounded-lg border border-emerald-600/30 text-emerald-600 transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-emerald-50 active:scale-90 dark:border-emerald-800 dark:text-emerald-400 dark:hover:bg-emerald-950/40"
            >
              <Check size={14} />
            </button>
            <button
              type="button"
              aria-label="Reject"
              title="Reject"
              onClick={() => void handleDecide(request, "rejected")}
              className="flex h-8 w-8 items-center justify-center rounded-lg border border-error/40 text-error transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-error/10 active:scale-90"
            >
              <X size={14} />
            </button>
          </span>
        ) : (
          <span className="text-xs text-on-surface-variant dark:text-zinc-500">—</span>
        ),
    },
  ];

  return (
    <Card className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-end gap-3">
          <Select
            label="Status"
            value={statusFilter}
            onChange={(event) => setStatusFilter(event.target.value as "all" | LeaveStatus)}
            options={[
              { value: "all", label: "All" },
              { value: "pending", label: "Pending" },
              { value: "approved", label: "Approved" },
              { value: "rejected", label: "Rejected" },
            ]}
            className="min-w-36"
          />
          <Select
            label="Employee"
            value={employeeFilter}
            onChange={(event) => setEmployeeFilter(event.target.value)}
            options={[
              { value: "all", label: "All employees" },
              ...employees.map((employee) => ({ value: employee.id, label: employee.full_name })),
            ]}
            className="min-w-40"
          />
        </div>
        <Button type="button" onClick={() => setFormOpen(true)}>
          <Plus size={16} />
          Request
        </Button>
      </div>

      <DataTable
        columns={columns}
        rows={visible}
        rowKey={(request) => request.id}
        emptyMessage="No leave requests match this filter."
        caption="Leave requests"
        pageSizeOptions={[10, 25, 50]}
      />

      <Modal open={formOpen} onClose={() => setFormOpen(false)} title="Request leave" size="sm">
        <RequestForm
          employees={employees}
          leaveTypes={leaveTypes}
          onClose={() => setFormOpen(false)}
          onSaved={refresh}
        />
      </Modal>
    </Card>
  );
}

interface LeaveTypeFormProps {
  leaveType: LeaveType | null;
  onClose: () => void;
  onSaved: () => void;
}

function LeaveTypeForm({ leaveType, onClose, onSaved }: Readonly<LeaveTypeFormProps>) {
  const { showToast } = useToast();
  const [name, setName] = useState(leaveType?.name ?? "");
  const [daysPerYear, setDaysPerYear] = useState(String(leaveType?.days_per_year ?? 14));
  const [paid, setPaid] = useState(leaveType?.paid ?? true);
  const [active, setActive] = useState(leaveType?.active ?? true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  async function handleSubmit() {
    if (!name.trim()) {
      setError("Leave type name is required");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const payload = { name, days_per_year: Number(daysPerYear) || 0, paid, active };
      if (leaveType) {
        await updateLeaveType(leaveType.id, payload);
        showToast(`${name} updated`, "success");
      } else {
        await createLeaveType(payload);
        showToast(`${name} added`, "success");
      }
      onSaved();
      onClose();
    } catch (submitError) {
      setError(
        submitError instanceof Error ? submitError.message : "Could not save leave type",
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
        label="Name"
        placeholder="e.g. Annual Leave"
        autoFocus
        value={name}
        onChange={(event) => setName(event.target.value)}
      />
      <NumberField
        label="Days per Year"
        value={daysPerYear}
        onChange={setDaysPerYear}
        max={365}
      />
      <Switch checked={paid} onChange={setPaid} label="Paid" />
      <Switch checked={active} onChange={setActive} label="Active" />
      {error && <p className="text-xs text-error">{error}</p>}
      <Button type="submit" loading={saving} className="self-start">
        <Check size={16} />
        {leaveType ? "Save changes" : "Submit"}
      </Button>
    </form>
  );
}

function LeaveTypesView() {
  const { showToast } = useToast();
  const [leaveTypes, setLeaveTypes] = useState<LeaveType[]>([]);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<LeaveType | null>(null);
  const [pendingDelete, setPendingDelete] = useState<LeaveType | null>(null);
  const [deleting, setDeleting] = useState(false);

  function reload() {
    listLeaveTypes().then(setLeaveTypes);
  }

  useEffect(reload, []);

  async function confirmDelete() {
    if (!pendingDelete) return;
    setDeleting(true);
    try {
      await deleteLeaveType(pendingDelete.id);
      showToast(`Deleted ${pendingDelete.name}`, "success");
      setPendingDelete(null);
      reload();
    } catch (error) {
      showToast(error instanceof Error ? error.message : "Could not delete leave type", "error");
    } finally {
      setDeleting(false);
    }
  }

  const columns: DataColumn<LeaveType>[] = [
    { key: "name", header: "Name", sortValue: (leaveType) => leaveType.name, render: (leaveType) => leaveType.name },
    {
      key: "days",
      header: "Days / Year",
      align: "right",
      render: (leaveType) => leaveType.days_per_year,
    },
    {
      key: "paid",
      header: "Paid",
      render: (leaveType) => (
        <Badge variant={leaveType.paid ? "success" : "neutral"}>
          {leaveType.paid ? "Paid" : "Unpaid"}
        </Badge>
      ),
    },
    {
      key: "status",
      header: "Status",
      render: (leaveType) => (
        <Badge variant={leaveType.active ? "success" : "neutral"}>
          {leaveType.active ? "Active" : "Inactive"}
        </Badge>
      ),
    },
    {
      key: "action",
      header: "Action",
      align: "right",
      render: (leaveType) => (
        <span className="flex items-center justify-end gap-1.5">
          <button
            type="button"
            onClick={() => {
              setEditing(leaveType);
              setFormOpen(true);
            }}
            aria-label={`Edit ${leaveType.name}`}
            className="flex h-8 w-8 items-center justify-center rounded-lg border border-emerald-600/30 text-emerald-600 transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-surface-container active:scale-90 dark:border-emerald-800 dark:text-emerald-400 dark:hover:bg-zinc-800"
          >
            <Pencil size={15} aria-hidden />
          </button>
          <button
            type="button"
            onClick={() => setPendingDelete(leaveType)}
            aria-label={`Delete ${leaveType.name}`}
            className="flex h-8 w-8 items-center justify-center rounded-lg border border-error/40 text-error transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-surface-container active:scale-90 dark:hover:bg-zinc-800"
          >
            <X size={15} aria-hidden />
          </button>
        </span>
      ),
    },
  ];

  return (
    <Card>
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-sm font-semibold text-on-surface dark:text-zinc-50">Leave Types</h2>
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
          rows={leaveTypes}
          rowKey={(leaveType) => leaveType.id}
          emptyMessage="No leave types yet."
          caption="Leave types"
          pageSizeOptions={[10, 25, 50]}
        />
      </div>

      <Modal
        open={formOpen}
        onClose={() => setFormOpen(false)}
        title={editing ? "Edit" : "Create"}
        size="sm"
      >
        <LeaveTypeForm
          key={editing?.id ?? "new"}
          leaveType={editing}
          onClose={() => setFormOpen(false)}
          onSaved={reload}
        />
      </Modal>

      <ConfirmDialog
        open={pendingDelete !== null}
        title="Delete leave type"
        message={`Delete ${pendingDelete?.name ?? "this leave type"}? Past requests keep their existing record.`}
        confirmLabel="Delete"
        destructive
        busy={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </Card>
  );
}

function BalanceView({ employees }: Readonly<{ employees: Employee[] }>) {
  const [leaveTypes, setLeaveTypes] = useState<LeaveType[]>([]);
  const [requests, setRequests] = useState<LeaveRequest[]>([]);
  const [employeeFilter, setEmployeeFilter] = useState("all");

  useEffect(() => {
    void Promise.all([listLeaveTypes(), listLeaveRequests()]).then(
      ([leaveTypeList, requestList]) => {
        setLeaveTypes(leaveTypeList.filter((leaveType) => leaveType.active));
        setRequests(requestList);
      },
    );
  }, []);

  interface BalanceRow {
    key: string;
    employeeName: string;
    leaveTypeName: string;
    allocated: number;
    used: number;
    remaining: number;
  }

  const rows: BalanceRow[] = employees
    .filter((employee) => employeeFilter === "all" || employee.id === employeeFilter)
    .flatMap((employee) =>
      leaveTypes.map((leaveType) => {
        const balance = leaveBalance(employee.id, leaveType.id, CURRENT_YEAR, requests, leaveType);
        return {
          key: `${employee.id}-${leaveType.id}`,
          employeeName: employee.full_name,
          leaveTypeName: leaveType.name,
          ...balance,
        };
      }),
    );

  const columns: DataColumn<BalanceRow>[] = [
    { key: "employee", header: "Employee", render: (row) => row.employeeName },
    { key: "type", header: "Leave Type", render: (row) => row.leaveTypeName },
    { key: "allocated", header: "Allocated", align: "right", render: (row) => row.allocated },
    { key: "used", header: "Used", align: "right", render: (row) => row.used },
    {
      key: "remaining",
      header: "Remaining",
      align: "right",
      render: (row) => (
        <span className={row.remaining < 0 ? "font-semibold text-error" : "font-semibold"}>
          {row.remaining}
        </span>
      ),
    },
  ];

  return (
    <Card className="flex flex-col gap-4">
      <Select
        label="Employee"
        value={employeeFilter}
        onChange={(event) => setEmployeeFilter(event.target.value)}
        options={[
          { value: "all", label: "All employees" },
          ...employees.map((employee) => ({ value: employee.id, label: employee.full_name })),
        ]}
        className="min-w-48"
      />
      <DataTable
        columns={columns}
        rows={rows}
        rowKey={(row) => row.key}
        emptyMessage="No leave types yet — add one under Leave Types."
        caption={`Leave balance, ${CURRENT_YEAR}`}
        pageSizeOptions={[10, 25, 50]}
      />
    </Card>
  );
}

export default function LeaveManagementPage() {
  const [view, setView] = useState<View>("requests");
  const [employees, setEmployees] = useState<Employee[]>([]);

  useEffect(() => {
    void listEmployees().then((list) => setEmployees(list.filter((employee) => employee.active)));
  }, []);

  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Leave Management"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Leave Management" }]}
      />

      <div className="flex w-fit gap-1 rounded-lg bg-surface-container p-0.5 dark:bg-zinc-800">
        {(
          [
            { value: "requests", label: "Requests" },
            { value: "types", label: "Leave Types" },
            { value: "balance", label: "Balance" },
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

      {view === "requests" && <RequestsView employees={employees} />}
      {view === "types" && <LeaveTypesView />}
      {view === "balance" && <BalanceView employees={employees} />}
    </div>
  );
}
