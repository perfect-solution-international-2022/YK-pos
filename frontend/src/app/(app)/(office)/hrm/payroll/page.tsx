"use client";

import { useEffect, useMemo, useState } from "react";
import {
  Check,
  CheckCircle2,
  Eye,
  FileSpreadsheet,
  FileText,
  Pencil,
  Trash2,
} from "lucide-react";
import {
  deletePayslip,
  generatePayroll,
  listDesignations,
  listEmployees,
  listPayslips,
  localDateKey,
  logAudit,
  markPayslipPaid,
  updatePayslip,
} from "@/lib/db";
import type { Designation, Employee, Payslip as PayslipRecord, PayslipStatus } from "@/lib/types";
import { useAuth } from "@/lib/hooks/use-auth";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DataTable, type DataColumn } from "@/components/ui/DataTable";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { NumberField } from "@/components/ui/NumberField";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { Payslip } from "@/components/ui/Payslip";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { useToast } from "@/components/ui/Toast";
import { useSettings } from "@/lib/hooks/use-settings";
import { exportExcel, exportPdf, type ExportColumn } from "@/lib/export";
import { ROUTES } from "@/lib/types/routes";

type View = "salary" | "history";

const STATUS_BADGE_VARIANT: Record<PayslipStatus, "neutral" | "success"> = {
  unpaid: "neutral",
  paid: "success",
};

function currentPeriod(): string {
  return localDateKey(Date.now()).slice(0, 7);
}

function periodLabel(period: string): string {
  const [year, month] = period.split("-").map(Number);
  return new Date(year, month - 1, 1).toLocaleDateString(undefined, {
    month: "long",
    year: "numeric",
  });
}

interface SharedData {
  employees: Employee[];
  designations: Designation[];
}

function SalaryListView({ employees, designations }: Readonly<SharedData>) {
  const { showToast } = useToast();
  const { money } = useSettings();
  const { staff } = useAuth();
  const [period, setPeriod] = useState(currentPeriod());
  const [generating, setGenerating] = useState(false);

  const designationById = useMemo(
    () => new Map(designations.map((designation) => [designation.id, designation.name])),
    [designations],
  );

  async function handleGenerate() {
    setGenerating(true);
    try {
      const result = await generatePayroll(period, employees);
      if (result.created > 0) {
        await logAudit({
          actor_id: staff?.id,
          actor_name: staff?.name ?? "System",
          action: "payroll.generate",
          resource: "payslip",
          resource_label: periodLabel(period),
          details: `${result.created} payslip${result.created === 1 ? "" : "s"} generated`,
        });
      }
      const payslipSuffix = result.created === 1 ? "" : "s";
      const skippedMessage =
        result.skipped > 0 ? ` (${result.skipped} already existed)` : "";
      const message =
        result.created > 0
          ? `Generated ${result.created} payslip${payslipSuffix}${skippedMessage}`
          : "Every active employee already has a payslip for this period";
      const toastVariant = result.created > 0 ? "success" : "info";
      showToast(message, toastVariant);
    } catch (error) {
      showToast(error instanceof Error ? error.message : "Could not generate payroll", "error");
    } finally {
      setGenerating(false);
    }
  }

  const columns: DataColumn<Employee>[] = [
    {
      key: "code",
      header: "Code",
      sortValue: (employee) => employee.employee_code,
      render: (employee) => (
        <span className="font-medium text-secondary dark:text-green-400">
          {employee.employee_code}
        </span>
      ),
    },
    {
      key: "name",
      header: "Name",
      sortValue: (employee) => employee.full_name,
      render: (employee) => employee.full_name,
    },
    {
      key: "designation",
      header: "Designation",
      hideOnMobile: true,
      render: (employee) => designationById.get(employee.designation_id) ?? "—",
    },
    {
      key: "salary",
      header: "Basic Salary",
      align: "right",
      sortValue: (employee) => employee.basic_salary_cents,
      render: (employee) => (
        <span className="font-semibold text-on-surface dark:text-zinc-50">
          {money(employee.basic_salary_cents)}
        </span>
      ),
    },
  ];

  return (
    <Card className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <Input
          type="month"
          label="Payroll period"
          value={period}
          onChange={(event) => setPeriod(event.target.value)}
          className="max-w-48"
        />
        <Button type="button" loading={generating} onClick={() => void handleGenerate()}>
          <Check size={16} />
          Generate payroll for {periodLabel(period)}
        </Button>
      </div>
      <DataTable
        columns={columns}
        rows={employees}
        rowKey={(employee) => employee.id}
        emptyMessage="No active employees yet."
        caption="Salary list"
        pageSizeOptions={[10, 25, 50]}
      />
    </Card>
  );
}

interface EditPayslipModalProps {
  payslip: PayslipRecord;
  onClose: () => void;
  onSaved: () => void;
}

function EditPayslipModal({ payslip, onClose, onSaved }: Readonly<EditPayslipModalProps>) {
  const { showToast } = useToast();
  const { staff } = useAuth();
  const [allowances, setAllowances] = useState((payslip.allowances_cents / 100).toFixed(2));
  const [deductions, setDeductions] = useState((payslip.deductions_cents / 100).toFixed(2));
  const [notes, setNotes] = useState(payslip.notes ?? "");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  async function handleSubmit() {
    setSaving(true);
    setError(null);
    try {
      await updatePayslip(payslip.id, {
        allowances_cents: Math.round(Number(allowances) * 100) || 0,
        deductions_cents: Math.round(Number(deductions) * 100) || 0,
        notes: notes.trim() || undefined,
      });
      await logAudit({
        actor_id: staff?.id,
        actor_name: staff?.name ?? "System",
        action: "payslip.update",
        resource: "payslip",
        resource_id: payslip.id,
        resource_label: periodLabel(payslip.period),
      });
      showToast("Payslip updated", "success");
      onSaved();
      onClose();
    } catch (submitError) {
      setError(
        submitError instanceof Error ? submitError.message : "Could not update payslip",
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal open onClose={onClose} title="Edit payslip" size="sm">
      <form
        className="flex flex-col gap-4"
        onSubmit={(event) => {
          event.preventDefault();
          void handleSubmit();
        }}
      >
        <NumberField label="Allowances" value={allowances} onChange={setAllowances} precision={2} />
        <NumberField label="Deductions" value={deductions} onChange={setDeductions} precision={2} />
        <Textarea label="Notes" value={notes} onChange={(event) => setNotes(event.target.value)} rows={2} />
        {error && <p className="text-xs text-error">{error}</p>}
        <Button type="submit" loading={saving} className="self-start">
          <Check size={16} />
          Save
        </Button>
      </form>
    </Modal>
  );
}

function HistoryView({ employees, designations }: Readonly<SharedData>) {
  const { showToast } = useToast();
  const { money } = useSettings();
  const { staff } = useAuth();
  const [payslips, setPayslips] = useState<PayslipRecord[]>([]);
  const [periodFilter, setPeriodFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState<"all" | PayslipStatus>("all");
  const [viewTarget, setViewTarget] = useState<PayslipRecord | null>(null);
  const [editTarget, setEditTarget] = useState<PayslipRecord | null>(null);
  const [pendingDelete, setPendingDelete] = useState<PayslipRecord | null>(null);
  const [deleting, setDeleting] = useState(false);

  const employeeById = useMemo(
    () => new Map(employees.map((employee) => [employee.id, employee])),
    [employees],
  );
  const designationById = useMemo(
    () => new Map(designations.map((designation) => [designation.id, designation.name])),
    [designations],
  );

  function reload() {
    listPayslips().then(setPayslips);
  }

  useEffect(reload, []);

  const periods = useMemo(
    () =>
      Array.from(new Set(payslips.map((payslip) => payslip.period))).sort((a, b) =>
        b.localeCompare(a),
      ),
    [payslips],
  );

  const visible = payslips.filter((payslip) => {
    if (periodFilter !== "all" && payslip.period !== periodFilter) return false;
    if (statusFilter !== "all" && payslip.status !== statusFilter) return false;
    return true;
  });

  async function handleMarkPaid(payslip: PayslipRecord) {
    try {
      await markPayslipPaid(payslip.id);
      await logAudit({
        actor_id: staff?.id,
        actor_name: staff?.name ?? "System",
        action: "payslip.mark_paid",
        resource: "payslip",
        resource_id: payslip.id,
        resource_label: periodLabel(payslip.period),
      });
      showToast("Marked as paid", "success");
      reload();
    } catch (error) {
      showToast(error instanceof Error ? error.message : "Could not update payslip", "error");
    }
  }

  async function confirmDelete() {
    if (!pendingDelete) return;
    setDeleting(true);
    try {
      await deletePayslip(pendingDelete.id);
      await logAudit({
        actor_id: staff?.id,
        actor_name: staff?.name ?? "System",
        action: "payslip.delete",
        resource: "payslip",
        resource_id: pendingDelete.id,
        resource_label: periodLabel(pendingDelete.period),
      });
      showToast("Payslip deleted", "success");
      setPendingDelete(null);
      reload();
    } catch (error) {
      showToast(error instanceof Error ? error.message : "Could not delete payslip", "error");
    } finally {
      setDeleting(false);
    }
  }

  const exportColumns: ExportColumn<PayslipRecord>[] = [
    { key: "period", header: "Period", value: (payslip) => periodLabel(payslip.period) },
    {
      key: "employee",
      header: "Employee",
      value: (payslip) => employeeById.get(payslip.employee_id)?.full_name ?? "—",
    },
    { key: "basic", header: "Basic Salary", value: (payslip) => money(payslip.basic_salary_cents) },
    { key: "allowances", header: "Allowances", value: (payslip) => money(payslip.allowances_cents) },
    { key: "deductions", header: "Deductions", value: (payslip) => money(payslip.deductions_cents) },
    { key: "net", header: "Net Pay", value: (payslip) => money(payslip.net_cents) },
    { key: "status", header: "Status", value: (payslip) => (payslip.status === "paid" ? "Paid" : "Unpaid") },
  ];

  const columns: DataColumn<PayslipRecord>[] = [
    {
      key: "period",
      header: "Period",
      sortValue: (payslip) => payslip.period,
      render: (payslip) => periodLabel(payslip.period),
    },
    {
      key: "employee",
      header: "Employee",
      render: (payslip) => employeeById.get(payslip.employee_id)?.full_name ?? "—",
    },
    {
      key: "designation",
      header: "Designation",
      hideOnMobile: true,
      render: (payslip) => {
        const employee = employeeById.get(payslip.employee_id);
        return employee ? designationById.get(employee.designation_id) ?? "—" : "—";
      },
    },
    {
      key: "net",
      header: "Net Pay",
      align: "right",
      sortValue: (payslip) => payslip.net_cents,
      render: (payslip) => (
        <span className="font-semibold text-on-surface dark:text-zinc-50">
          {money(payslip.net_cents)}
        </span>
      ),
    },
    {
      key: "status",
      header: "Status",
      render: (payslip) => (
        <Badge variant={STATUS_BADGE_VARIANT[payslip.status]}>
          {payslip.status === "paid" ? "Paid" : "Unpaid"}
        </Badge>
      ),
    },
    {
      key: "actions",
      header: "Action",
      align: "right",
      render: (payslip) => (
        <span className="flex items-center justify-end gap-1.5">
          <button
            type="button"
            aria-label="View payslip"
            title="View"
            onClick={() => setViewTarget(payslip)}
            className="flex h-8 w-8 items-center justify-center rounded-lg border border-outline-variant text-on-surface-variant transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-surface-container active:scale-90 dark:border-zinc-700 dark:text-zinc-400 dark:hover:bg-zinc-800"
          >
            <Eye size={14} />
          </button>
          {payslip.status === "unpaid" && (
            <>
              <button
                type="button"
                aria-label="Edit payslip"
                title="Edit"
                onClick={() => setEditTarget(payslip)}
                className="flex h-8 w-8 items-center justify-center rounded-lg border border-emerald-600/30 text-emerald-600 transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-emerald-50 active:scale-90 dark:border-emerald-800 dark:text-emerald-400 dark:hover:bg-emerald-950/40"
              >
                <Pencil size={14} />
              </button>
              <button
                type="button"
                aria-label="Mark as paid"
                title="Mark as paid"
                onClick={() => void handleMarkPaid(payslip)}
                className="flex h-8 w-8 items-center justify-center rounded-lg border border-outline-variant text-on-surface-variant transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-surface-container active:scale-90 dark:border-zinc-700 dark:text-zinc-400 dark:hover:bg-zinc-800"
              >
                <CheckCircle2 size={14} />
              </button>
              <button
                type="button"
                aria-label="Delete payslip"
                title="Delete"
                onClick={() => setPendingDelete(payslip)}
                className="flex h-8 w-8 items-center justify-center rounded-lg border border-error/40 text-error transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-error/10 active:scale-90"
              >
                <Trash2 size={14} />
              </button>
            </>
          )}
        </span>
      ),
    },
  ];

  return (
    <Card className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-end gap-3">
          <Select
            label="Period"
            value={periodFilter}
            onChange={(event) => setPeriodFilter(event.target.value)}
            options={[
              { value: "all", label: "All periods" },
              ...periods.map((period) => ({ value: period, label: periodLabel(period) })),
            ]}
            className="min-w-40"
          />
          <Select
            label="Status"
            value={statusFilter}
            onChange={(event) => setStatusFilter(event.target.value as "all" | PayslipStatus)}
            options={[
              { value: "all", label: "All" },
              { value: "unpaid", label: "Unpaid" },
              { value: "paid", label: "Paid" },
            ]}
            className="min-w-36"
          />
        </div>
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => exportPdf("Payroll history", visible, exportColumns)}
          >
            <FileText size={15} />
            PDF
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => exportExcel("payroll", "Payroll history", visible, exportColumns)}
          >
            <FileSpreadsheet size={15} />
            EXCEL
          </Button>
        </div>
      </div>

      <DataTable
        columns={columns}
        rows={visible}
        rowKey={(payslip) => payslip.id}
        emptyMessage="No payslips yet — generate one from Salary List."
        caption="Payroll history"
        pageSizeOptions={[10, 25, 50]}
      />

      <Modal
        open={viewTarget !== null}
        onClose={() => setViewTarget(null)}
        title="Payslip"
        size="sm"
      >
        {viewTarget && employeeById.get(viewTarget.employee_id) && (
          <Payslip
            payslip={viewTarget}
            employee={employeeById.get(viewTarget.employee_id)!}
            designationName={designationById.get(
              employeeById.get(viewTarget.employee_id)!.designation_id,
            )}
          />
        )}
      </Modal>

      {editTarget && (
        <EditPayslipModal
          key={editTarget.id}
          payslip={editTarget}
          onClose={() => setEditTarget(null)}
          onSaved={reload}
        />
      )}

      <ConfirmDialog
        open={pendingDelete !== null}
        title="Delete payslip"
        message="This removes the payslip permanently. It can be regenerated from Salary List."
        confirmLabel="Delete"
        destructive
        busy={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </Card>
  );
}

export default function PayrollPage() {
  const [view, setView] = useState<View>("salary");
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [designations, setDesignations] = useState<Designation[]>([]);

  useEffect(() => {
    void Promise.all([listEmployees(), listDesignations()]).then(
      ([employeeList, designationList]) => {
        setEmployees(employeeList.filter((employee) => employee.active));
        setDesignations(designationList);
      },
    );
  }, []);

  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Payroll"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Payroll" }]}
      />

      <div className="flex w-fit gap-1 rounded-lg bg-surface-container p-0.5 dark:bg-zinc-800">
        {(
          [
            { value: "salary", label: "Salary List" },
            { value: "history", label: "Payroll History" },
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

      {view === "salary" && <SalaryListView employees={employees} designations={designations} />}
      {view === "history" && <HistoryView employees={employees} designations={designations} />}
    </div>
  );
}
