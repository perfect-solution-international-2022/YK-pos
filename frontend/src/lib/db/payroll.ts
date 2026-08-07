import { db } from "./index";
import type { Employee, Payslip } from "@/lib/types";

// Local-only table, no server sync yet (no backend endpoint exists).

export async function listPayslips(): Promise<Payslip[]> {
  return db.payslips.orderBy("generated_at").reverse().toArray();
}

export async function getPayslip(id: string): Promise<Payslip | undefined> {
  return db.payslips.get(id);
}

export async function listPayslipsForPeriod(period: string): Promise<Payslip[]> {
  return db.payslips.where("period").equals(period).toArray();
}

export interface GeneratePayrollResult {
  created: number;
  skipped: number;
}

/**
 * One payslip per active employee per period, skipping anyone who already
 * has one for that period rather than overwriting it — regenerating must
 * never silently erase edits already made to an existing slip.
 */
export async function generatePayroll(
  period: string,
  employees: Employee[],
): Promise<GeneratePayrollResult> {
  const existing = await listPayslipsForPeriod(period);
  const existingEmployeeIds = new Set(existing.map((payslip) => payslip.employee_id));

  const toCreate = employees
    .filter((employee) => employee.active && !existingEmployeeIds.has(employee.id))
    .map((employee) => {
      const payslip: Payslip = {
        id: crypto.randomUUID(),
        employee_id: employee.id,
        period,
        basic_salary_cents: employee.basic_salary_cents,
        allowances_cents: 0,
        deductions_cents: 0,
        net_cents: employee.basic_salary_cents,
        status: "unpaid",
        generated_at: Date.now(),
      };
      return payslip;
    });

  if (toCreate.length > 0) await db.payslips.bulkAdd(toCreate);

  return {
    created: toCreate.length,
    skipped: employees.filter((employee) => employee.active).length - toCreate.length,
  };
}

export async function updatePayslip(
  id: string,
  changes: Partial<Pick<Payslip, "allowances_cents" | "deductions_cents" | "notes">>,
): Promise<void> {
  const payslip = await db.payslips.get(id);
  if (!payslip) throw new Error("That payslip no longer exists");

  const allowances = changes.allowances_cents ?? payslip.allowances_cents;
  const deductions = changes.deductions_cents ?? payslip.deductions_cents;

  await db.payslips.update(id, {
    ...changes,
    net_cents: payslip.basic_salary_cents + allowances - deductions,
  });
}

export async function markPayslipPaid(id: string): Promise<void> {
  await db.payslips.update(id, { status: "paid", paid_at: Date.now() });
}

export async function deletePayslip(id: string): Promise<void> {
  await db.payslips.delete(id);
}
