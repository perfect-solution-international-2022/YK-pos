import { Star } from "lucide-react";
import { EmptyState } from "@/components/ui/EmptyState";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { ROUTES } from "@/lib/types/routes";

export default function PerformancePage() {
  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Performance"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Performance" }]}
      />
      <Card>
        <EmptyState
          icon={<Star size={20} />}
          title="Performance is coming in a later phase"
          description="Employee evaluation, ratings, notes and monthly reviews will live here."
        />
      </Card>
    </div>
  );
}
