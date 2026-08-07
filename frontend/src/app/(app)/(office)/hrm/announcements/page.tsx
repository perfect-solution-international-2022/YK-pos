import { Megaphone } from "lucide-react";
import { EmptyState } from "@/components/ui/EmptyState";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { ROUTES } from "@/lib/types/routes";

export default function AnnouncementsPage() {
  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Announcements"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Announcements" }]}
      />
      <Card>
        <EmptyState
          icon={<Megaphone size={20} />}
          title="Announcements are coming in a later phase"
          description="Announcement list, create, publish and archive will live here."
        />
      </Card>
    </div>
  );
}
