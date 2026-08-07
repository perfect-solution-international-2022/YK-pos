"use client";

import { useEffect, useState } from "react";
import { Archive, Check, Pencil, Plus, Send, X } from "lucide-react";
import {
  archiveAnnouncement,
  createAnnouncement,
  deleteAnnouncement,
  listAnnouncements,
  logAudit,
  publishAnnouncement,
  updateAnnouncement,
} from "@/lib/db";
import type { Announcement, AnnouncementStatus } from "@/lib/types";
import { useAuth } from "@/lib/hooks/use-auth";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DataTable, type DataColumn } from "@/components/ui/DataTable";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { useToast } from "@/components/ui/Toast";
import { formatDateTime } from "@/lib/format";
import { ROUTES } from "@/lib/types/routes";

const STATUS_BADGE_VARIANT: Record<AnnouncementStatus, "neutral" | "success" | "warning"> = {
  draft: "neutral",
  published: "success",
  archived: "warning",
};

const ACTION_BUTTON_CLASSES =
  "inline-flex h-8 w-8 items-center justify-center rounded-md border border-outline-variant transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:scale-105 active:scale-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary dark:border-zinc-700";

interface AnnouncementFormProps {
  announcement: Announcement | null;
  onClose: () => void;
  onSaved: () => void;
}

function AnnouncementForm({ announcement, onClose, onSaved }: Readonly<AnnouncementFormProps>) {
  const { showToast } = useToast();
  const { staff } = useAuth();
  const [title, setTitle] = useState(announcement?.title ?? "");
  const [body, setBody] = useState(announcement?.body ?? "");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  async function handleSubmit() {
    if (!title.trim()) {
      setError("Title is required");
      return;
    }
    if (!body.trim()) {
      setError("Body is required");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      if (announcement) {
        await updateAnnouncement(announcement.id, { title, body });
        await logAudit({
          actor_id: staff?.id,
          actor_name: staff?.name ?? "System",
          action: "announcement.update",
          resource: "announcement",
          resource_id: announcement.id,
          resource_label: title,
        });
        showToast("Announcement updated", "success");
      } else {
        const created = await createAnnouncement({ title, body, created_by: staff?.id });
        await logAudit({
          actor_id: staff?.id,
          actor_name: staff?.name ?? "System",
          action: "announcement.create",
          resource: "announcement",
          resource_id: created.id,
          resource_label: title,
        });
        showToast("Announcement created", "success");
      }
      onSaved();
      onClose();
    } catch (submitError) {
      setError(
        submitError instanceof Error ? submitError.message : "Could not save announcement",
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        event.preventDefault();
        void handleSubmit();
      }}
    >
      <Input
        label="Title"
        placeholder="e.g. Office closed for the holiday"
        autoFocus
        value={title}
        onChange={(event) => setTitle(event.target.value)}
      />
      <Textarea
        label="Body"
        value={body}
        onChange={(event) => setBody(event.target.value)}
        rows={5}
      />
      {error && <p className="text-xs text-error">{error}</p>}
      <Button type="submit" loading={saving} className="self-start">
        <Check size={16} />
        {announcement ? "Save changes" : "Save as draft"}
      </Button>
    </form>
  );
}

export default function AnnouncementsPage() {
  const { showToast } = useToast();
  const { staff } = useAuth();
  const [announcements, setAnnouncements] = useState<Announcement[]>([]);
  const [statusFilter, setStatusFilter] = useState<"all" | AnnouncementStatus>("all");
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Announcement | null>(null);
  const [pendingDelete, setPendingDelete] = useState<Announcement | null>(null);
  const [deleting, setDeleting] = useState(false);

  function reload() {
    listAnnouncements().then(setAnnouncements);
  }

  useEffect(reload, []);

  const visible = announcements.filter(
    (announcement) => statusFilter === "all" || announcement.status === statusFilter,
  );

  async function handlePublish(announcement: Announcement) {
    try {
      await publishAnnouncement(announcement.id);
      await logAudit({
        actor_id: staff?.id,
        actor_name: staff?.name ?? "System",
        action: "announcement.publish",
        resource: "announcement",
        resource_id: announcement.id,
        resource_label: announcement.title,
      });
      showToast("Announcement published", "success");
      reload();
    } catch (error) {
      showToast(
        error instanceof Error ? error.message : "Could not publish announcement",
        "error",
      );
    }
  }

  async function handleArchive(announcement: Announcement) {
    try {
      await archiveAnnouncement(announcement.id);
      await logAudit({
        actor_id: staff?.id,
        actor_name: staff?.name ?? "System",
        action: "announcement.archive",
        resource: "announcement",
        resource_id: announcement.id,
        resource_label: announcement.title,
      });
      showToast("Announcement archived", "success");
      reload();
    } catch (error) {
      showToast(
        error instanceof Error ? error.message : "Could not archive announcement",
        "error",
      );
    }
  }

  async function confirmDelete() {
    if (!pendingDelete) return;
    setDeleting(true);
    try {
      await deleteAnnouncement(pendingDelete.id);
      await logAudit({
        actor_id: staff?.id,
        actor_name: staff?.name ?? "System",
        action: "announcement.delete",
        resource: "announcement",
        resource_id: pendingDelete.id,
        resource_label: pendingDelete.title,
      });
      showToast("Announcement deleted", "success");
      setPendingDelete(null);
      reload();
    } catch (error) {
      showToast(
        error instanceof Error ? error.message : "Could not delete announcement",
        "error",
      );
    } finally {
      setDeleting(false);
    }
  }

  const columns: DataColumn<Announcement>[] = [
    {
      key: "title",
      header: "Title",
      sortValue: (announcement) => announcement.title,
      render: (announcement) => (
        <span className="block max-w-64 truncate font-medium">{announcement.title}</span>
      ),
    },
    {
      key: "status",
      header: "Status",
      render: (announcement) => (
        <Badge variant={STATUS_BADGE_VARIANT[announcement.status]}>
          {announcement.status[0].toUpperCase() + announcement.status.slice(1)}
        </Badge>
      ),
    },
    {
      key: "created",
      header: "Created",
      hideOnMobile: true,
      sortValue: (announcement) => announcement.created_at,
      render: (announcement) => formatDateTime(announcement.created_at),
    },
    {
      key: "published",
      header: "Published",
      hideOnMobile: true,
      render: (announcement) =>
        announcement.published_at ? formatDateTime(announcement.published_at) : "—",
    },
    {
      key: "action",
      header: "Action",
      align: "right",
      render: (announcement) => (
        <span className="flex items-center justify-end gap-1.5">
          {announcement.status === "draft" && (
            <>
              <button
                type="button"
                onClick={() => {
                  setEditing(announcement);
                  setFormOpen(true);
                }}
                aria-label={`Edit ${announcement.title}`}
                title="Edit"
                className={`${ACTION_BUTTON_CLASSES} text-emerald-600 hover:bg-surface-container dark:text-emerald-400 dark:hover:bg-zinc-800`}
              >
                <Pencil size={15} aria-hidden />
              </button>
              <button
                type="button"
                onClick={() => void handlePublish(announcement)}
                aria-label={`Publish ${announcement.title}`}
                title="Publish"
                className={`${ACTION_BUTTON_CLASSES} text-secondary hover:bg-surface-container dark:text-green-400 dark:hover:bg-zinc-800`}
              >
                <Send size={15} aria-hidden />
              </button>
            </>
          )}
          {announcement.status === "published" && (
            <button
              type="button"
              onClick={() => void handleArchive(announcement)}
              aria-label={`Archive ${announcement.title}`}
              title="Archive"
              className={`${ACTION_BUTTON_CLASSES} text-amber-600 hover:bg-surface-container dark:text-amber-400 dark:hover:bg-zinc-800`}
            >
              <Archive size={15} aria-hidden />
            </button>
          )}
          <button
            type="button"
            onClick={() => setPendingDelete(announcement)}
            aria-label={`Delete ${announcement.title}`}
            title="Delete"
            className={`${ACTION_BUTTON_CLASSES} text-error hover:bg-surface-container dark:text-red-400 dark:hover:bg-zinc-800`}
          >
            <X size={15} aria-hidden />
          </button>
        </span>
      ),
    },
  ];

  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Announcements"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Announcements" }]}
      />

      <Card>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <Select
            label="Status"
            value={statusFilter}
            onChange={(event) => setStatusFilter(event.target.value as "all" | AnnouncementStatus)}
            options={[
              { value: "all", label: "All" },
              { value: "draft", label: "Draft" },
              { value: "published", label: "Published" },
              { value: "archived", label: "Archived" },
            ]}
            className="min-w-40"
          />
          <Button
            type="button"
            onClick={() => {
              setEditing(null);
              setFormOpen(true);
            }}
          >
            <Plus size={16} />
            Create
          </Button>
        </div>

        <div className="mt-4">
          <DataTable
            columns={columns}
            rows={visible}
            rowKey={(announcement) => announcement.id}
            emptyMessage="No announcements yet."
            caption="Announcements"
            pageSizeOptions={[10, 25, 50]}
          />
        </div>
      </Card>

      <Modal
        open={formOpen}
        onClose={() => setFormOpen(false)}
        title={editing ? "Edit" : "Create"}
        size="md"
      >
        <AnnouncementForm
          key={editing?.id ?? "new"}
          announcement={editing}
          onClose={() => setFormOpen(false)}
          onSaved={reload}
        />
      </Modal>

      <ConfirmDialog
        open={pendingDelete !== null}
        title="Delete announcement"
        message={`Delete "${pendingDelete?.title ?? "this announcement"}"? This cannot be undone.`}
        confirmLabel="Delete"
        destructive
        busy={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </div>
  );
}
