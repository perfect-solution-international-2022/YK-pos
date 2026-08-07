"use client";

import { useEffect, useMemo, useState } from "react";
import { Check, Pencil, Plus, X } from "lucide-react";
import {
  createReview,
  deleteReview,
  listEmployees,
  listReviews,
  updateReview,
} from "@/lib/db";
import type { Employee, PerformanceReview } from "@/lib/types";
import { useAuth } from "@/lib/hooks/use-auth";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DataTable, type DataColumn } from "@/components/ui/DataTable";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { StarRating } from "@/components/ui/StarRating";
import { Textarea } from "@/components/ui/Textarea";
import { useToast } from "@/components/ui/Toast";
import { ROUTES } from "@/lib/types/routes";

function periodLabel(period: string): string {
  const [year, month] = period.split("-").map(Number);
  return new Date(year, month - 1, 1).toLocaleDateString(undefined, {
    month: "long",
    year: "numeric",
  });
}

function currentPeriod(): string {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
}

interface ReviewFormProps {
  review: PerformanceReview | null;
  employees: Employee[];
  defaultEmployeeId?: string;
  onClose: () => void;
  onSaved: () => void;
}

function ReviewForm({
  review,
  employees,
  defaultEmployeeId,
  onClose,
  onSaved,
}: Readonly<ReviewFormProps>) {
  const { showToast } = useToast();
  const { staff } = useAuth();
  const [employeeId, setEmployeeId] = useState(review?.employee_id ?? defaultEmployeeId ?? "");
  const [period, setPeriod] = useState(review?.period ?? currentPeriod());
  const [rating, setRating] = useState(review?.rating ?? 0);
  const [notes, setNotes] = useState(review?.notes ?? "");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  async function handleSubmit() {
    if (!employeeId) {
      setError("Pick an employee");
      return;
    }
    if (!period) {
      setError("Pick a period");
      return;
    }
    if (rating < 1) {
      setError("Pick a rating");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      if (review) {
        await updateReview(review.id, { period, rating, notes: notes.trim() || undefined });
        showToast("Review updated", "success");
      } else {
        await createReview({
          employee_id: employeeId,
          period,
          rating,
          notes: notes.trim() || undefined,
          reviewed_by: staff?.id,
        });
        showToast("Review submitted", "success");
      }
      onSaved();
      onClose();
    } catch (submitError) {
      setError(
        submitError instanceof Error ? submitError.message : "Could not save review",
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
      {!review && (
        <Select
          label="Employee"
          placeholder="Please select"
          value={employeeId}
          onChange={(event) => setEmployeeId(event.target.value)}
          options={employees.map((employee) => ({
            value: employee.id,
            label: employee.full_name,
          }))}
        />
      )}
      <Input
        type="month"
        label="Period"
        value={period}
        onChange={(event) => setPeriod(event.target.value)}
      />
      <div className="flex flex-col gap-1.5">
        <span className="text-sm font-medium text-on-surface-variant dark:text-zinc-300">
          Rating
        </span>
        <StarRating value={rating} onChange={setRating} size={26} />
      </div>
      <Textarea
        label="Notes"
        value={notes}
        onChange={(event) => setNotes(event.target.value)}
        rows={3}
      />
      {error && <p className="text-xs text-error">{error}</p>}
      <Button type="submit" loading={saving} className="self-start">
        <Check size={16} />
        {review ? "Save changes" : "Submit"}
      </Button>
    </form>
  );
}

export default function PerformancePage() {
  const { showToast } = useToast();
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [reviews, setReviews] = useState<PerformanceReview[]>([]);
  const [employeeFilter, setEmployeeFilter] = useState("all");
  const [periodFilter, setPeriodFilter] = useState("all");
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<PerformanceReview | null>(null);
  const [pendingDelete, setPendingDelete] = useState<PerformanceReview | null>(null);
  const [deleting, setDeleting] = useState(false);

  const employeeById = useMemo(
    () => new Map(employees.map((employee) => [employee.id, employee.full_name])),
    [employees],
  );

  function reload() {
    listReviews().then(setReviews);
  }

  useEffect(() => {
    void listEmployees().then((list) => setEmployees(list.filter((employee) => employee.active)));
    reload();
  }, []);

  const periods = useMemo(
    () => Array.from(new Set(reviews.map((review) => review.period))).sort().reverse(),
    [reviews],
  );

  const visible = reviews.filter((review) => {
    if (employeeFilter !== "all" && review.employee_id !== employeeFilter) return false;
    if (periodFilter !== "all" && review.period !== periodFilter) return false;
    return true;
  });

  async function confirmDelete() {
    if (!pendingDelete) return;
    setDeleting(true);
    try {
      await deleteReview(pendingDelete.id);
      showToast("Review deleted", "success");
      setPendingDelete(null);
      reload();
    } catch (error) {
      showToast(error instanceof Error ? error.message : "Could not delete review", "error");
    } finally {
      setDeleting(false);
    }
  }

  const columns: DataColumn<PerformanceReview>[] = [
    {
      key: "employee",
      header: "Employee",
      render: (review) => employeeById.get(review.employee_id) ?? "—",
    },
    {
      key: "period",
      header: "Period",
      sortValue: (review) => review.period,
      render: (review) => periodLabel(review.period),
    },
    {
      key: "rating",
      header: "Rating",
      sortValue: (review) => review.rating,
      render: (review) => <StarRating value={review.rating} size={15} />,
    },
    {
      key: "notes",
      header: "Notes",
      hideOnMobile: true,
      render: (review) => (
        <span className="block max-w-64 truncate">{review.notes || "—"}</span>
      ),
    },
    {
      key: "actions",
      header: "Action",
      align: "right",
      render: (review) => (
        <span className="flex items-center justify-end gap-1.5">
          <button
            type="button"
            aria-label="Edit review"
            title="Edit"
            onClick={() => {
              setEditing(review);
              setFormOpen(true);
            }}
            className="flex h-8 w-8 items-center justify-center rounded-lg border border-emerald-600/30 text-emerald-600 transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-emerald-50 active:scale-90 dark:border-emerald-800 dark:text-emerald-400 dark:hover:bg-emerald-950/40"
          >
            <Pencil size={14} />
          </button>
          <button
            type="button"
            aria-label="Delete review"
            title="Delete"
            onClick={() => setPendingDelete(review)}
            className="flex h-8 w-8 items-center justify-center rounded-lg border border-error/40 text-error transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-error/10 active:scale-90"
          >
            <X size={14} />
          </button>
        </span>
      ),
    },
  ];

  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Performance"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Performance" }]}
      />

      <Card className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-end gap-3">
            <Select
              label="Employee"
              value={employeeFilter}
              onChange={(event) => setEmployeeFilter(event.target.value)}
              options={[
                { value: "all", label: "All employees" },
                ...employees.map((employee) => ({ value: employee.id, label: employee.full_name })),
              ]}
              className="min-w-40"
            />
            <Select
              label="Period"
              value={periodFilter}
              onChange={(event) => setPeriodFilter(event.target.value)}
              options={[
                { value: "all", label: "All periods" },
                ...periods.map((period) => ({ value: period, label: periodLabel(period) })),
              ]}
              className="min-w-40"
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
            New Review
          </Button>
        </div>

        <DataTable
          columns={columns}
          rows={visible}
          rowKey={(review) => review.id}
          emptyMessage="No reviews yet."
          caption="Performance reviews"
          pageSizeOptions={[10, 25, 50]}
        />
      </Card>

      <Modal
        open={formOpen}
        onClose={() => setFormOpen(false)}
        title={editing ? "Edit review" : "New review"}
        size="sm"
      >
        <ReviewForm
          key={editing?.id ?? "new"}
          review={editing}
          employees={employees}
          defaultEmployeeId={employeeFilter !== "all" ? employeeFilter : undefined}
          onClose={() => setFormOpen(false)}
          onSaved={reload}
        />
      </Modal>

      <ConfirmDialog
        open={pendingDelete !== null}
        title="Delete review"
        message="This removes the review permanently."
        confirmLabel="Delete"
        destructive
        busy={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </div>
  );
}
