import { Wallet } from "lucide-react";
import { EmptyState } from "@/components/ui/EmptyState";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { ROUTES } from "@/lib/types/routes";

export default function PayrollPage() {
  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Payroll"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Payroll" }]}
      />
      <Card>
        <EmptyState
          icon={<Wallet size={20} />}
          title="Payroll is coming in a later phase"
          description="Salary list, payroll history, payslip view and payslip download will live here."
        />
      </Card>
    </div>
  );
}
