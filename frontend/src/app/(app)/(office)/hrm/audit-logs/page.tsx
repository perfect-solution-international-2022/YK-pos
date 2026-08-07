import { History } from "lucide-react";
import { EmptyState } from "@/components/ui/EmptyState";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { ROUTES } from "@/lib/types/routes";

export default function HrmAuditLogsPage() {
  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Audit Logs"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Audit Logs" }]}
      />
      <Card>
        <EmptyState
          icon={<History size={20} />}
          title="Audit logs are coming in a later phase"
          description="Who did what, when — searchable and filterable — will live here."
        />
      </Card>
    </div>
  );
}
