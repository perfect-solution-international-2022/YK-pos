"use client";

import { useEffect, useMemo, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import {
  Eye,
  FileSpreadsheet,
  FileText,
  Pencil,
  Plus,
  Search,
  SlidersHorizontal,
  User,
  X,
} from "lucide-react";
import { deleteEmployee, listDesignations, listEmployees } from "@/lib/db";
import type { Designation, Employee, EmploymentType } from "@/lib/types";
import { exportExcel, exportPdf, type ExportColumn } from "@/lib/export";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Select } from "@/components/ui/Select";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DataTable, type DataColumn } from "@/components/ui/DataTable";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/components/ui/Toast";
import { useSettings } from "@/lib/hooks/use-settings";
import { ROUTES } from "@/lib/types/routes";

type StatusFilter = "all" | "active" | "inactive";

const EMPLOYMENT_TYPE_LABELS: Record<EmploymentType, string> = {
  full_time: "Full time",
  part_time: "Part time",
  contract: "Contract",
  intern: "Intern",
};

export default function EmployeesPage() {
  const { showToast } = useToast();
  const { money } = useSettings();
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [designations, setDesignations] = useState<Designation[]>([]);

  const [search, setSearch] = useState("");
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [designationFilter, setDesignationFilter] = useState("all");
  const [employmentTypeFilter, setEmploymentTypeFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");

  const [pendingDelete, setPendingDelete] = useState<Employee | null>(null);
  const [deleting, setDeleting] = useState(false);

  const designationName = useMemo(() => {
    const byId = new Map(designations.map((designation) => [designation.id, designation.name]));
    return (id: string) => byId.get(id) ?? "—";
  }, [designations]);

  const visibleEmployees = useMemo(() => {
    const term = search.trim().toLowerCase();
    return employees.filter((employee) => {
      if (designationFilter !== "all" && employee.designation_id !== designationFilter) {
        return false;
      }
      if (employmentTypeFilter !== "all" && employee.employment_type !== employmentTypeFilter) {
        return false;
      }
      if (statusFilter === "active" && !employee.active) return false;
      if (statusFilter === "inactive" && employee.active) return false;
      if (!term) return true;
      return [
        employee.employee_code,
        employee.full_name,
        employee.phone,
        employee.email ?? "",
        designationName(employee.designation_id),
      ]
        .join(" ")
        .toLowerCase()
        .includes(term);
    });
  }, [employees, search, designationFilter, employmentTypeFilter, statusFilter, designationName]);

  const activeFilterCount =
    (designationFilter === "all" ? 0 : 1) +
    (employmentTypeFilter === "all" ? 0 : 1) +
    (statusFilter === "all" ? 0 : 1);

  const exportColumns: ExportColumn<Employee>[] = [
    { key: "code", header: "Code", value: (employee) => employee.employee_code },
    { key: "name", header: "Name", value: (employee) => employee.full_name },
    {
      key: "designation",
      header: "Designation",
      value: (employee) => designationName(employee.designation_id),
    },
    { key: "phone", header: "Phone", value: (employee) => employee.phone },
    {
      key: "salary",
      header: "Basic Salary",
      value: (employee) => money(employee.basic_salary_cents),
    },
    {
      key: "status",
      header: "Status",
      value: (employee) => (employee.active ? "Active" : "Inactive"),
    },
  ];

  async function refresh() {
    const [employeeList, designationList] = await Promise.all([
      listEmployees(),
      listDesignations(),
    ]);
    setEmployees(employeeList);
    setDesignations(designationList);
  }

  useEffect(() => {
    const refreshTimerId = window.setTimeout(() => void refresh(), 0);
    return () => window.clearTimeout(refreshTimerId);
  }, []);

  async function handleDeleteEmployee() {
    if (!pendingDelete) return;
    setDeleting(true);
    try {
      await deleteEmployee(pendingDelete.id);
      showToast("Employee removed", "success");
      setPendingDelete(null);
      await refresh();
    } catch (error) {
      showToast(
        error instanceof Error ? error.message : "Could not delete employee",
        "error",
      );
    } finally {
      setDeleting(false);
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
      sortValue: (employee) => designationName(employee.designation_id),
      render: (employee) => designationName(employee.designation_id),
    },
    {
      key: "phone",
      header: "Phone",
      hideOnMobile: true,
      render: (employee) => <span className="tabular-nums">{employee.phone}</span>,
    },
    {
      key: "status",
      header: "Status",
      align: "right",
      render: (employee) => (
        <Badge variant={employee.active ? "success" : "neutral"}>
          {employee.active ? "Active" : "Inactive"}
        </Badge>
      ),
    },
    {
      key: "actions",
      header: "Action",
      align: "right",
      render: (employee) => (
        <span className="flex min-w-24 items-center justify-end gap-2">
          <Link
            href={ROUTES.hrm.employees.detail(employee.id)}
            aria-label={`View ${employee.full_name}`}
            className="flex h-8 w-8 items-center justify-center rounded-lg border border-outline-variant text-on-surface-variant transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-surface-container active:scale-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary dark:border-zinc-700 dark:text-zinc-400 dark:hover:bg-zinc-800"
          >
            <Eye size={14} />
          </Link>
          <Link
            href={ROUTES.hrm.employees.edit(employee.id)}
            aria-label={`Edit ${employee.full_name}`}
            className="flex h-8 w-8 items-center justify-center rounded-lg border border-emerald-600/30 text-emerald-600 transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-emerald-50 active:scale-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary dark:border-emerald-800 dark:text-emerald-400 dark:hover:bg-emerald-950/40"
          >
            <Pencil size={14} />
          </Link>
          <button
            type="button"
            aria-label={`Remove ${employee.full_name}`}
            onClick={() => setPendingDelete(employee)}
            className="flex h-8 w-8 items-center justify-center rounded-lg border border-error/40 text-error transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-error/10 active:scale-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
          >
            <X size={14} />
          </button>
        </span>
      ),
    },
  ];

  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Employees"
        breadcrumbs={[
          { label: "HRM", href: ROUTES.hrm.dashboard },
          { label: "Employees" },
        ]}
      />

      <div className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="relative w-full max-w-xs">
            <Search
              size={15}
              aria-hidden
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-on-surface-variant dark:text-zinc-500"
            />
            <input
              type="search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search this table"
              aria-label="Search employees"
              className="min-h-10 w-full rounded-lg border border-outline-variant bg-surface-container-lowest py-2 pl-9 pr-3 text-sm text-on-surface outline-none transition-colors duration-[var(--duration-fast)] placeholder:text-on-surface-variant focus-visible:border-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-50 dark:placeholder:text-zinc-500"
            />
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              aria-expanded={filtersOpen}
              onClick={() => setFiltersOpen((open) => !open)}
            >
              <SlidersHorizontal size={15} />
              Filter
              {activeFilterCount > 0 && (
                <span className="ml-1 rounded-full bg-primary px-1.5 text-[11px] font-semibold text-on-primary">
                  {activeFilterCount}
                </span>
              )}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => exportPdf("Employees", visibleEmployees, exportColumns)}
            >
              <FileText size={15} />
              PDF
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() =>
                exportExcel("employees", "Employees", visibleEmployees, exportColumns)
              }
            >
              <FileSpreadsheet size={15} />
              EXCEL
            </Button>
            <Link
              href={ROUTES.hrm.employees.new}
              className="inline-flex min-h-9 items-center gap-2 rounded-xl bg-secondary px-3 py-1.5 text-xs font-medium text-on-secondary transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-secondary/90 hover:shadow-elevated active:scale-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary dark:bg-white dark:text-zinc-900 dark:hover:bg-zinc-200"
            >
              <Plus size={15} />
              Create
            </Link>
          </div>
        </div>

        {filtersOpen && (
          <div className="animate-fade-in flex flex-wrap items-end gap-3 rounded-xl border border-outline-variant bg-surface-container-lowest p-3 dark:border-zinc-800 dark:bg-zinc-900">
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
            <Select
              label="Employment Type"
              value={employmentTypeFilter}
              onChange={(event) => setEmploymentTypeFilter(event.target.value)}
              options={[
                { value: "all", label: "All types" },
                ...Object.entries(EMPLOYMENT_TYPE_LABELS).map(([value, label]) => ({
                  value,
                  label,
                })),
              ]}
              className="min-w-40"
            />
            <Select
              label="Status"
              value={statusFilter}
              onChange={(event) => setStatusFilter(event.target.value as StatusFilter)}
              options={[
                { value: "all", label: "All" },
                { value: "active", label: "Active" },
                { value: "inactive", label: "Inactive" },
              ]}
              className="min-w-40"
            />
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => {
                setDesignationFilter("all");
                setEmploymentTypeFilter("all");
                setStatusFilter("all");
              }}
            >
              Clear filters
            </Button>
          </div>
        )}

        <DataTable
          columns={columns}
          rows={visibleEmployees}
          rowKey={(employee) => employee.id}
          emptyMessage="No employees match this search. Create one to get started."
          caption="Employees"
          pageSizeOptions={[10, 25, 50]}
        />
      </div>

      <ConfirmDialog
        open={pendingDelete !== null}
        title="Remove employee?"
        message={`${pendingDelete?.full_name ?? "This employee"} will be removed from the HR record. A linked till login, if any, is kept — remove it separately from Users if needed.`}
        confirmLabel="Remove employee"
        destructive
        busy={deleting}
        onConfirm={handleDeleteEmployee}
        onCancel={() => setPendingDelete(null)}
      />
    </div>
  );
}
