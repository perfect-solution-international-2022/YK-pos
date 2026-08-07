"use client";

import { useEffect, useMemo, useState } from "react";
import { FileSpreadsheet, FileText, Search } from "lucide-react";
import { listAuditLogs } from "@/lib/db";
import type { AuditLogEntry } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { DataTable, type DataColumn } from "@/components/ui/DataTable";
import { Card, PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { exportExcel, exportPdf, type ExportColumn } from "@/lib/export";
import { formatDateTime } from "@/lib/format";
import { ROUTES } from "@/lib/types/routes";

export default function HrmAuditLogsPage() {
  const [logs, setLogs] = useState<AuditLogEntry[]>([]);
  const [search, setSearch] = useState("");
  const [resourceFilter, setResourceFilter] = useState("all");

  useEffect(() => {
    listAuditLogs().then(setLogs);
  }, []);

  const resources = useMemo(
    () => Array.from(new Set(logs.map((log) => log.resource))).sort(),
    [logs],
  );

  const visible = useMemo(() => {
    const term = search.trim().toLowerCase();
    return logs.filter((log) => {
      if (resourceFilter !== "all" && log.resource !== resourceFilter) return false;
      if (!term) return true;
      return [log.actor_name, log.action, log.resource, log.resource_label ?? "", log.details ?? ""]
        .join(" ")
        .toLowerCase()
        .includes(term);
    });
  }, [logs, search, resourceFilter]);

  const exportColumns: ExportColumn<AuditLogEntry>[] = [
    { key: "date", header: "Date", value: (log) => formatDateTime(log.created_at) },
    { key: "user", header: "User", value: (log) => log.actor_name },
    { key: "action", header: "Action", value: (log) => log.action },
    { key: "resource", header: "Resource", value: (log) => log.resource_label ?? log.resource },
    { key: "details", header: "Details", value: (log) => log.details ?? "" },
  ];

  const columns: DataColumn<AuditLogEntry>[] = [
    {
      key: "date",
      header: "Date",
      sortValue: (log) => log.created_at,
      render: (log) => (
        <span className="whitespace-nowrap tabular-nums">{formatDateTime(log.created_at)}</span>
      ),
    },
    { key: "user", header: "User", render: (log) => log.actor_name },
    {
      key: "action",
      header: "Action",
      render: (log) => (
        <span className="rounded-md bg-surface-container px-2 py-0.5 font-mono text-xs dark:bg-zinc-800">
          {log.action}
        </span>
      ),
    },
    {
      key: "resource",
      header: "Resource",
      hideOnMobile: true,
      render: (log) => log.resource_label ?? log.resource,
    },
    {
      key: "details",
      header: "Details",
      hideOnMobile: true,
      render: (log) => (
        <span className="block max-w-64 truncate text-on-surface-variant dark:text-zinc-400">
          {log.details || "—"}
        </span>
      ),
    },
  ];

  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="Audit Logs"
        breadcrumbs={[{ label: "HRM", href: ROUTES.hrm.dashboard }, { label: "Audit Logs" }]}
      />

      <Card>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-end gap-3">
            <div className="relative w-full max-w-xs">
              <Search
                size={15}
                aria-hidden
                className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-on-surface-variant dark:text-zinc-500"
              />
              <input
                type="search"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Search this table"
                aria-label="Search audit logs"
                className="min-h-10 w-full rounded-lg border border-outline-variant bg-surface-container-lowest py-2 pl-9 pr-3 text-sm text-on-surface outline-none transition-colors duration-[var(--duration-fast)] placeholder:text-on-surface-variant focus-visible:border-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-50 dark:placeholder:text-zinc-500"
              />
            </div>
            <Select
              label="Resource"
              value={resourceFilter}
              onChange={(event) => setResourceFilter(event.target.value)}
              options={[
                { value: "all", label: "All resources" },
                ...resources.map((resource) => ({ value: resource, label: resource })),
              ]}
              className="min-w-40"
            />
          </div>
          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => exportPdf("Audit logs", visible, exportColumns)}
            >
              <FileText size={15} />
              PDF
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => exportExcel("audit-logs", "Audit logs", visible, exportColumns)}
            >
              <FileSpreadsheet size={15} />
              EXCEL
            </Button>
          </div>
        </div>

        <div className="mt-4">
          <DataTable
            columns={columns}
            rows={visible}
            rowKey={(log) => log.id}
            emptyMessage="No activity recorded yet."
            caption="Audit logs"
            pageSizeOptions={[10, 25, 50]}
          />
        </div>
      </Card>
    </div>
  );
}
