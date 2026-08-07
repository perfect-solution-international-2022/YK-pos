"use client";

import { useEffect, useState } from "react";
import { Check, Pencil, Plus, Search, X } from "lucide-react";
import {
  createDesignation,
  deleteDesignation,
  listDesignations,
  updateDesignation,
} from "@/lib/db";
import type { Designation } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DataTable, type DataColumn } from "@/components/ui/DataTable";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { Switch } from "@/components/ui/Switch";
import { Textarea } from "@/components/ui/Textarea";
import { Badge } from "@/components/ui/Badge";
import { useToast } from "@/components/ui/Toast";
import { ROUTES } from "@/lib/types/routes";

function RequiredMark() {
  return (
    <span className="text-error" aria-hidden>
      {" "}
      *
    </span>
  );
}

const ACTION_BUTTON_CLASSES =
  "inline-flex h-8 w-8 items-center justify-center rounded-md border border-outline-variant transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:scale-105 active:scale-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary dark:border-zinc-700";

function filterDesignations(designations: Designation[], query: string): Designation[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return designations;
  return designations.filter((designation) =>
    designation.name.toLowerCase().includes(needle),
  );
}

interface DesignationFormProps {
  designation: Designation | null;
  onClose: () => void;
  onSaved: () => void;
}

/**
 * Mounted only while its `Modal` is open, so field state initialises fresh
 * from `designation` every time it opens rather than needing an effect to
 * reset it.
 */
function DesignationForm({
  designation,
  onClose,
  onSaved,
}: Readonly<DesignationFormProps>) {
  const { showToast } = useToast();
  const [name, setName] = useState(designation?.name ?? "");
  const [description, setDescription] = useState(designation?.description ?? "");
  const [active, setActive] = useState(designation?.active ?? true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  async function handleSubmit() {
    if (!name.trim()) {
      setError("Designation name is required");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      if (designation) {
        await updateDesignation(designation.id, { name, description, active });
        showToast(`${name} updated`, "success");
      } else {
        await createDesignation({ name, description, active });
        showToast(`${name} added`, "success");
      }
      onSaved();
      onClose();
    } catch (submitError) {
      setError(
        submitError instanceof Error ? submitError.message : "Could not save designation",
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
        label={
          <>
            Name
            <RequiredMark />
          </>
        }
        placeholder="e.g. Store Manager"
        autoFocus
        value={name}
        onChange={(event) => setName(event.target.value)}
      />
      <Textarea
        label="Description"
        placeholder="What this role is responsible for"
        value={description}
        onChange={(event) => setDescription(event.target.value)}
        rows={3}
      />
      <Switch
        checked={active}
        onChange={setActive}
        label="Active"
        description="Inactive designations stay on past records but drop out of new pickers."
      />
      {error && <p className="text-xs text-error">{error}</p>}
      <Button type="submit" loading={saving} className="self-start">
        <Check size={16} />
        {designation ? "Save changes" : "Submit"}
      </Button>
    </form>
  );
}

interface DesignationFormModalProps {
  open: boolean;
  designation: Designation | null;
  onClose: () => void;
  onSaved: () => void;
}

function DesignationFormModal({
  open,
  designation,
  onClose,
  onSaved,
}: Readonly<DesignationFormModalProps>) {
  return (
    <Modal open={open} onClose={onClose} title={designation ? "Edit" : "Create"} size="sm">
      <DesignationForm
        key={designation?.id ?? "new"}
        designation={designation}
        onClose={onClose}
        onSaved={onSaved}
      />
    </Modal>
  );
}

export default function DesignationsPage() {
  const { showToast } = useToast();

  const [query, setQuery] = useState("");
  const [designations, setDesignations] = useState<Designation[]>([]);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Designation | null>(null);
  const [pendingDelete, setPendingDelete] = useState<Designation | null>(null);
  const [deleting, setDeleting] = useState(false);

  function reload() {
    listDesignations().then(setDesignations);
  }

  useEffect(reload, []);

  const visible = filterDesignations(designations, query);

  async function confirmDelete() {
    if (!pendingDelete) return;
    setDeleting(true);
    try {
      await deleteDesignation(pendingDelete.id);
      showToast(`Deleted ${pendingDelete.name}`, "success");
      setSelectedIds((current) => current.filter((id) => id !== pendingDelete.id));
      setPendingDelete(null);
      reload();
    } catch (error) {
      showToast(
        error instanceof Error ? error.message : "Could not delete designation",
        "error",
      );
    } finally {
      setDeleting(false);
    }
  }

  const columns: DataColumn<Designation>[] = [
    {
      key: "name",
      header: "Name",
      sortValue: (designation) => designation.name,
      render: (designation) => (
        <span className="font-medium text-on-surface dark:text-zinc-50">
          {designation.name}
        </span>
      ),
    },
    {
      key: "description",
      header: "Description",
      hideOnMobile: true,
      render: (designation) => (
        <span className="text-on-surface-variant dark:text-zinc-400">
          {designation.description || "—"}
        </span>
      ),
    },
    {
      key: "status",
      header: "Status",
      align: "right",
      render: (designation) => (
        <Badge variant={designation.active ? "success" : "neutral"}>
          {designation.active ? "Active" : "Inactive"}
        </Badge>
      ),
    },
    {
      key: "action",
      header: "Action",
      align: "right",
      render: (designation) => (
        <span className="flex items-center justify-end gap-1.5">
          <button
            type="button"
            onClick={() => {
              setEditing(designation);
              setFormOpen(true);
            }}
            aria-label={`Edit ${designation.name}`}
            title="Edit"
            className={`${ACTION_BUTTON_CLASSES} text-emerald-600 hover:bg-surface-container dark:text-emerald-400 dark:hover:bg-zinc-800`}
          >
            <Pencil size={15} aria-hidden />
          </button>
          <button
            type="button"
            onClick={() => setPendingDelete(designation)}
            aria-label={`Delete ${designation.name}`}
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
        title="Designations"
        breadcrumbs={[
          { label: "HRM", href: ROUTES.hrm.dashboard },
          { label: "Designations" },
        ]}
      />

      <Card>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="relative w-full sm:w-72">
            <Search
              size={16}
              aria-hidden
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-on-surface-variant dark:text-zinc-500"
            />
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search this table"
              aria-label="Search designations"
              className="pl-9"
            />
          </div>
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
            rowKey={(designation) => designation.id}
            emptyMessage="No designations yet."
            caption="Designations"
            pageSizeOptions={[10, 25, 50]}
            selection={{ selectedIds, onChange: setSelectedIds }}
          />
        </div>
      </Card>

      <DesignationFormModal
        open={formOpen}
        designation={editing}
        onClose={() => setFormOpen(false)}
        onSaved={reload}
      />

      <ConfirmDialog
        open={pendingDelete !== null}
        title="Delete designation"
        message={`Delete ${pendingDelete?.name ?? "this designation"}? Employees already assigned to it keep their existing record.`}
        confirmLabel="Delete"
        destructive
        busy={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </div>
  );
}
