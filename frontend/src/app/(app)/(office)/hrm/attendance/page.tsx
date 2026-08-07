import { ClipboardList } from "lucide-react";
import { EmptyState } from "@/components/ui/EmptyState";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { ROUTES } from "@/lib/types/routes";

export default function AttendancePage() {
  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Attendance"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Attendance" }]}
      />
      <Card>
        <EmptyState
          icon={<ClipboardList size={20} />}
          title="Attendance is coming in a later phase"
          description="Daily attendance, clock in/out, manual attendance, the monthly calendar and attendance reports will live here."
        />
      </Card>
    </div>
  );
}
