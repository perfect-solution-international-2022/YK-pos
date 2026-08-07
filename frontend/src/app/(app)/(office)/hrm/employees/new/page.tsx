"use client";

import { useRouter } from "next/navigation";
import { createEmployee, logAudit } from "@/lib/db";
import {
  EMPTY_EMPLOYEE_FORM,
  EmployeeForm,
  type EmployeeFormValues,
} from "../components/EmployeeForm";
import { useAuth } from "@/lib/hooks/use-auth";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/components/ui/Toast";
import { ROUTES } from "@/lib/types/routes";

const CREATE_EMPLOYEE_BREADCRUMBS = [
  { label: "HRM", href: ROUTES.hrm.dashboard },
  { label: "Employees", href: ROUTES.hrm.employees.root },
  { label: "Create" },
];

function optionalText(value: string): string | undefined {
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

export default function CreateEmployeePage() {
  const router = useRouter();
  const { showToast } = useToast();
  const { staff } = useAuth();

  async function handleSubmit(values: EmployeeFormValues) {
    const employee = await createEmployee({
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
      action: "employee.create",
      resource: "employee",
      resource_id: employee.id,
      resource_label: employee.full_name,
    });
    showToast("Employee created", "success");
    router.push(ROUTES.hrm.employees.root);
  }

  function handleCancel() {
    router.push(ROUTES.hrm.employees.root);
  }

  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader title="Create employee" breadcrumbs={CREATE_EMPLOYEE_BREADCRUMBS} />
      <EmployeeForm
        mode="create"
        initialValues={EMPTY_EMPLOYEE_FORM}
        onSubmit={handleSubmit}
        onCancel={handleCancel}
      />
    </div>
  );
}
