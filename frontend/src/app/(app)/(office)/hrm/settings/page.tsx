import { Settings } from "lucide-react";
import { EmptyState } from "@/components/ui/EmptyState";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { ROUTES } from "@/lib/types/routes";

export default function HrmSettingsPage() {
  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="HRM Settings"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Settings" }]}
      />
      <Card>
        <EmptyState
          icon={<Settings size={20} />}
          title="HRM settings are coming in a later phase"
          description="HR preferences, payroll settings, attendance settings and leave settings will live here."
        />
      </Card>
    </div>
  );
}
