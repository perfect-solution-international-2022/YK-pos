"use client";

import { use, useEffect, useState, type ReactNode } from "react";
import Image from "next/image";
import Link from "next/link";
import { Pencil, User } from "lucide-react";
import {
  displayUsername,
  getDesignation,
  getEmployee,
  getShift,
  getStaffUser,
} from "@/lib/db";
import type { Designation, Employee, Shift, StaffUser } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";
import { Card, PageHeader, SectionHeader } from "@/components/ui/PageHeader";
import { Skeleton } from "@/components/ui/Skeleton";
import { useSettings } from "@/lib/hooks/use-settings";
import { formatDateTime } from "@/lib/format";
import { ROUTES } from "@/lib/types/routes";

const GENDER_LABELS: Record<Employee["gender"], string> = {
  male: "Male",
  female: "Female",
  other: "Other",
};

const EMPLOYMENT_TYPE_LABELS: Record<Employee["employment_type"], string> = {
  full_time: "Full time",
  part_time: "Part time",
  contract: "Contract",
  intern: "Intern",
};

function Field({ label, value }: Readonly<{ label: string; value: ReactNode }>) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-on-surface-variant dark:text-zinc-500">{label}</span>
      <span className="text-sm text-on-surface dark:text-zinc-50">{value || "—"}</span>
    </div>
  );
}

export default function EmployeeDetailsPage({
  params,
}: Readonly<{ params: Promise<{ id: string }> }>) {
  const { id } = use(params);
  const { money } = useSettings();
  const [employee, setEmployee] = useState<Employee | null>(null);
  const [designation, setDesignation] = useState<Designation | null>(null);
  const [shift, setShift] = useState<Shift | null>(null);
  const [linkedUser, setLinkedUser] = useState<StaffUser | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const found = await getEmployee(id);
      if (cancelled) return;
      setEmployee(found ?? null);
      if (found) {
        const [designationRecord, shiftRecord, staffUser] = await Promise.all([
          getDesignation(found.designation_id),
          getShift(found.shift_id),
          found.staff_user_id ? getStaffUser(found.staff_user_id) : Promise.resolve(undefined),
        ]);
        if (cancelled) return;
        setDesignation(designationRecord ?? null);
        setShift(shiftRecord ?? null);
        setLinkedUser(staffUser ?? null);
      }
      setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, [id]);

  const breadcrumbs = [
    { label: "HRM", href: ROUTES.hrm.dashboard },
    { label: "Employees", href: ROUTES.hrm.employees.root },
    { label: employee?.full_name ?? "Details" },
  ];

  if (loading) {
    return (
      <div className="flex flex-col gap-6 pb-8">
        <PageHeader title="Employee details" breadcrumbs={breadcrumbs} />
        <div className="grid gap-4 sm:grid-cols-2">
          {Array.from({ length: 4 }, (_, index) => (
            <Skeleton key={index} className="h-32 w-full rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  if (!employee) {
    return (
      <div className="flex flex-col gap-6 pb-8">
        <PageHeader title="Employee details" breadcrumbs={breadcrumbs} />
        <EmptyState icon={<User size={20} />} title="That employee no longer exists." />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title={employee.full_name}
        description={`#${employee.employee_code}`}
        breadcrumbs={breadcrumbs}
        actions={
          <Link
            href={ROUTES.hrm.employees.edit(employee.id)}
            className="inline-flex min-h-10 items-center gap-2 rounded-lg border border-outline-variant px-4 text-sm font-medium text-on-surface transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-surface-container active:scale-[0.97] dark:border-zinc-700 dark:text-zinc-100 dark:hover:bg-zinc-800"
          >
            <Pencil size={15} />
            Edit
          </Link>
        }
      />

      <Card className="flex flex-col gap-4 sm:flex-row sm:items-center">
        {employee.photo ? (
          <Image
            src={employee.photo}
            alt=""
            width={64}
            height={64}
            unoptimized
            className="h-16 w-16 shrink-0 rounded-full object-cover"
          />
        ) : (
          <span className="flex h-16 w-16 shrink-0 items-center justify-center rounded-full bg-surface-container text-on-surface-variant dark:bg-zinc-800 dark:text-zinc-400">
            <User size={24} />
          </span>
        )}
        <div className="flex flex-1 flex-wrap items-center gap-3">
          <div>
            <p className="text-base font-semibold text-on-surface dark:text-zinc-50">
              {employee.full_name}
            </p>
            <p className="text-sm text-on-surface-variant dark:text-zinc-400">
              {designation?.name ?? "—"} · {shift?.name ?? "—"}
            </p>
          </div>
          <Badge variant={employee.active ? "success" : "neutral"}>
            {employee.active ? "Active" : "Inactive"}
          </Badge>
        </div>
      </Card>

      <div className="flex flex-col gap-3">
        <SectionHeader title="Personal" />
        <Card className="grid gap-4 sm:grid-cols-3">
          <Field label="NIC" value={employee.nic} />
          <Field label="Date of Birth" value={employee.date_of_birth} />
          <Field label="Gender" value={GENDER_LABELS[employee.gender]} />
          <Field label="Phone" value={employee.phone} />
          <Field label="Email" value={employee.email} />
          <Field label="Address" value={employee.address} />
          <Field label="Emergency Contact" value={employee.emergency_contact_name} />
          <Field label="Emergency Phone" value={employee.emergency_contact_phone} />
        </Card>
      </div>

      <div className="flex flex-col gap-3">
        <SectionHeader title="Employment" />
        <Card className="grid gap-4 sm:grid-cols-3">
          <Field label="Designation" value={designation?.name} />
          <Field label="Joining Date" value={employee.joining_date} />
          <Field
            label="Employment Type"
            value={EMPLOYMENT_TYPE_LABELS[employee.employment_type]}
          />
          <Field label="Shift" value={shift?.name} />
        </Card>
      </div>

      <div className="flex flex-col gap-3">
        <SectionHeader title="Salary" />
        <Card className="grid gap-4 sm:grid-cols-3">
          <Field label="Basic Salary" value={money(employee.basic_salary_cents)} />
          <Field label="Bank Name" value={employee.bank_name} />
          <Field label="Bank Account No." value={employee.bank_account_no} />
          <Field label="Bank Branch" value={employee.bank_branch} />
        </Card>
      </div>

      <div className="flex flex-col gap-3">
        <SectionHeader title="System" />
        <Card className="grid gap-4 sm:grid-cols-3">
          <Field
            label="Login"
            value={linkedUser ? displayUsername(linkedUser) : "No till login"}
          />
          <Field label="Created" value={formatDateTime(employee.created_at)} />
        </Card>
      </div>

      {employee.documents.length > 0 && (
        <div className="flex flex-col gap-3">
          <SectionHeader title="Documents" />
          <Card>
            <ul className="flex flex-col gap-2">
              {employee.documents.map((document) => (
                <li key={document.name}>
                  <a
                    href={document.data_url}
                    download={document.name}
                    className="text-sm text-primary hover:underline dark:text-green-400"
                  >
                    {document.name}
                  </a>
                </li>
              ))}
            </ul>
          </Card>
        </div>
      )}
    </div>
  );
}
