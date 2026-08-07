"use client";

import { useRef } from "react";
import { Printer } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { useSettings } from "@/lib/hooks/use-settings";
import { formatDate } from "@/lib/format";
import { printHtml } from "@/lib/export";
import type { Employee, Payslip as PayslipRecord } from "@/lib/types";

interface PayslipProps {
  payslip: PayslipRecord;
  employee: Employee;
  designationName?: string;
  /** Hides the print/download control when embedded read-only. */
  showActions?: boolean;
}

function periodLabel(period: string): string {
  const [year, month] = period.split("-").map(Number);
  return new Date(year, month - 1, 1).toLocaleDateString(undefined, {
    month: "long",
    year: "numeric",
  });
}

// Mirrors Receipt.tsx's pattern: render on-screen, then hand the same markup
// to printHtml for the browser's print-to-PDF dialog — "Print" and "Download"
// are the same action throughout this app, no PDF library involved.
export function Payslip({
  payslip,
  employee,
  designationName,
  showActions = true,
}: Readonly<PayslipProps>) {
  const { money, settings } = useSettings();
  const printableRef = useRef<HTMLDivElement>(null);

  function handlePrint() {
    const markup = printableRef.current?.innerHTML;
    if (!markup) return;
    printHtml(
      `Payslip ${employee.full_name} ${payslip.period}`,
      markup,
      `body{width:320px;font-size:12px}
       .payslip-row{display:flex;justify-content:space-between;gap:8px}
       .payslip-divider{border-top:1px dashed #999;margin:6px 0}
       .payslip-center{text-align:center}
       .payslip-total{font-weight:700;font-size:14px}`,
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <div
        ref={printableRef}
        className="mx-auto w-full max-w-sm rounded-2xl border border-outline-variant bg-surface-container-lowest p-6 font-mono text-sm dark:border-zinc-800 dark:bg-zinc-900"
      >
        <p className="payslip-center text-center text-base font-semibold text-on-surface dark:text-zinc-50">
          {settings.store_name}
        </p>
        <p className="payslip-center text-center text-xs text-on-surface-variant dark:text-zinc-400">
          Payslip — {periodLabel(payslip.period)}
        </p>

        <div className="payslip-divider my-4 flex flex-col gap-1 text-on-surface dark:text-zinc-50">
          <div className="payslip-row flex justify-between">
            <span>Employee</span>
            <span>{employee.full_name}</span>
          </div>
          <div className="payslip-row flex justify-between text-on-surface-variant dark:text-zinc-400">
            <span>Code</span>
            <span>{employee.employee_code}</span>
          </div>
          {designationName && (
            <div className="payslip-row flex justify-between text-on-surface-variant dark:text-zinc-400">
              <span>Designation</span>
              <span>{designationName}</span>
            </div>
          )}
          {payslip.paid_at && (
            <div className="payslip-row flex justify-between text-on-surface-variant dark:text-zinc-400">
              <span>Paid on</span>
              <span>{formatDate(payslip.paid_at, settings.locale)}</span>
            </div>
          )}
        </div>

        <div className="payslip-divider border-t border-dashed border-outline-variant pt-2 dark:border-zinc-700">
          <div className="payslip-row flex justify-between text-on-surface dark:text-zinc-50">
            <span>Basic Salary</span>
            <span>{money(payslip.basic_salary_cents)}</span>
          </div>
          {payslip.allowances_cents > 0 && (
            <div className="payslip-row flex justify-between text-on-surface-variant dark:text-zinc-400">
              <span>Allowances</span>
              <span>+{money(payslip.allowances_cents)}</span>
            </div>
          )}
          {payslip.deductions_cents > 0 && (
            <div className="payslip-row flex justify-between text-on-surface-variant dark:text-zinc-400">
              <span>Deductions</span>
              <span>-{money(payslip.deductions_cents)}</span>
            </div>
          )}
          <div className="payslip-row payslip-total mt-1 flex justify-between border-t border-dashed border-outline-variant pt-2 text-base font-semibold text-on-surface dark:border-zinc-700 dark:text-zinc-50">
            <span>Net Pay</span>
            <span>{money(payslip.net_cents)}</span>
          </div>
        </div>

        {payslip.notes && (
          <p className="payslip-center mt-4 text-center text-xs text-on-surface-variant dark:text-zinc-400">
            {payslip.notes}
          </p>
        )}

        <p className="payslip-center mt-3 text-center text-xs font-semibold uppercase text-on-surface-variant dark:text-zinc-400">
          {payslip.status === "paid" ? "Paid" : "Unpaid"}
        </p>
      </div>

      {showActions && (
        <div className="mx-auto flex w-full max-w-sm gap-2">
          <Button type="button" variant="outline" className="flex-1" onClick={handlePrint}>
            <Printer size={16} />
            Print / Download
          </Button>
        </div>
      )}
    </div>
  );
}
