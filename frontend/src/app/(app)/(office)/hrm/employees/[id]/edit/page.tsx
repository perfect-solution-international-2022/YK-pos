"use client";

import { use, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { displayUsername, getEmployee, getStaffUser, logAudit, updateEmployee } from "@/lib/db";
import type { Employee, StaffUser } from "@/lib/types";
import {
  EMPTY_EMPLOYEE_FORM,
  EmployeeForm,
  type EmployeeFormValues,
} from "../../components/EmployeeForm";
import { useAuth } from "@/lib/hooks/use-auth";
import { PageHeader } from "@/components/ui/PageHeader";
import { Skeleton } from "@/components/ui/Skeleton";
import { useToast } from "@/components/ui/Toast";
import { ROUTES } from "@/lib/types/routes";

const EDIT_EMPLOYEE_BREADCRUMBS = [
  { label: "HRM", href: ROUTES.hrm.dashboard },
  { label: "Employees", href: ROUTES.hrm.employees.root },
  { label: "Edit" },
];

function optionalText(value: string): string | undefined {
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

export default function EditEmployeePage({
  params,
}: Readonly<{ params: Promise<{ id: string }> }>) {
  const { id } = use(params);
  const router = useRouter();
  const { showToast } = useToast();
  const { staff } = useAuth();
  const [employee, setEmployee] = useState<Employee | null>(null);
  const [linkedUser, setLinkedUser] = useState<StaffUser | null>(null);
  const [notFound, setNotFound] = useState(false);

  useEffect(() => {
    void (async () => {
      const found = await getEmployee(id);
      if (!found) {
        setNotFound(true);
        return;
      }
      setEmployee(found);
      if (found.staff_user_id) {
        setLinkedUser((await getStaffUser(found.staff_user_id)) ?? null);
      }
    })();
  }, [id]);

  const initialValues = useMemo<EmployeeFormValues>(() => {
    if (!employee) return EMPTY_EMPLOYEE_FORM;
    return {
      full_name: employee.full_name,
      nic: employee.nic,
      date_of_birth: employee.date_of_birth,
      gender: employee.gender,
      phone: employee.phone,
      email: employee.email ?? "",
      address: employee.address ?? "",
      emergency_contact_name: employee.emergency_contact_name ?? "",
      emergency_contact_phone: employee.emergency_contact_phone ?? "",
      designation_id: employee.designation_id,
      joining_date: employee.joining_date,
      employment_type: employee.employment_type,
      shift_id: employee.shift_id,
      basic_salary: (employee.basic_salary_cents / 100).toFixed(2),
      bank_name: employee.bank_name ?? "",
      bank_account_no: employee.bank_account_no ?? "",
      bank_branch: employee.bank_branch ?? "",
      enable_login: Boolean(linkedUser),
      username: linkedUser ? displayUsername(linkedUser) : "",
      // Always blank: the stored PIN is a hash, so an empty field means
      // "leave the current PIN alone" (see UserForm's edit page).
      password: "",
      role_id: linkedUser?.role_id ?? "",
      photo: employee.photo ?? "",
      documents: employee.documents,
      active: employee.active,
    };
  }, [employee, linkedUser]);

  async function handleSubmit(values: EmployeeFormValues) {
    if (!employee) return;
    await updateEmployee(employee.id, {
      full_name: values.full_name.trim(),
      nic: values.nic.trim(),
      date_of_birth: values.date_of_birth,
      gender: values.gender || "other",
      phone: values.phone.trim(),
      email: optionalText(values.email.toLowerCase()),
      address: optionalText(values.address),
      emergency_contact_name: optionalText(values.emergency_contact_name),
      emergency_contact_phone: optionalText(values.emergency_contact_phone),
      designation_id: values.designation_id,
      joining_date: values.joining_date,
      employment_type: values.employment_type || "full_time",
      shift_id: values.shift_id,
      basic_salary_cents: Math.round(Number(values.basic_salary) * 100),
      bank_name: optionalText(values.bank_name),
      bank_account_no: optionalText(values.bank_account_no),
      bank_branch: optionalText(values.bank_branch),
      photo: optionalText(values.photo),
      documents: values.documents,
      active: values.active,
      login: values.enable_login
        ? {
            username: values.username.trim(),
            password: values.password,
            role_id: values.role_id,
          }
        : undefined,
    });
    await logAudit({
      actor_id: staff?.id,
      actor_name: staff?.name ?? "System",
      action: "employee.update",
      resource: "employee",
      resource_id: employee.id,
      resource_label: employee.full_name,
    });
    showToast("Employee updated", "success");
    router.push(ROUTES.hrm.employees.root);
  }

  if (notFound) {
    return (
      <div className="flex flex-col gap-6 pb-8">
        <PageHeader title="Edit employee" breadcrumbs={EDIT_EMPLOYEE_BREADCRUMBS} />
        <p className="rounded-xl border border-dashed border-outline-variant py-10 text-center text-sm text-on-surface-variant dark:border-zinc-800 dark:text-zinc-400">
          That employee no longer exists.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader title="Edit employee" breadcrumbs={EDIT_EMPLOYEE_BREADCRUMBS} />
      {employee ? (
        <EmployeeForm
          key={employee.id}
          mode="edit"
          initialValues={initialValues}
          onSubmit={handleSubmit}
          onCancel={() => router.push(ROUTES.hrm.employees.root)}
        />
      ) : (
        <div className="flex flex-col gap-4 rounded-2xl border border-outline-variant p-4 sm:p-6 dark:border-zinc-800">
          <div className="grid gap-4 sm:grid-cols-2">
            {Array.from({ length: 8 }, (_, index) => (
              <Skeleton key={index} className="h-16 w-full rounded-lg" />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
