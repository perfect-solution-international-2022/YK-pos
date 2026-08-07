import { CalendarDays } from "lucide-react";
import { EmptyState } from "@/components/ui/EmptyState";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { ROUTES } from "@/lib/types/routes";

export default function LeaveManagementPage() {
  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Leave Management"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Leave Management" }]}
      />
      <Card>
        <EmptyState
          icon={<CalendarDays size={20} />}
          title="Leave Management is coming in a later phase"
          description="Leave types, leave requests, approve/reject, leave history and leave balance will live here."
        />
      </Card>
    </div>
  );
}
