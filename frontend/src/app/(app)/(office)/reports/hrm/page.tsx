import { FileClock } from "lucide-react";
import { EmptyState } from "@/components/ui/EmptyState";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { ROUTES } from "@/lib/types/routes";

export default function HrmReportsPage() {
  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Reports"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Reports" }]}
      />
      <Card>
        <EmptyState
          icon={<FileClock size={20} />}
          title="HR reports are coming in a later phase"
          description="Employee, attendance, leave and payroll reports will live here."
        />
      </Card>
    </div>
  );
}
