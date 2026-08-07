"use client";

import { useEffect, useState } from "react";
import {
  AlarmClock,
  BarChart3,
  Cake,
  CalendarOff,
  Clock,
  Inbox,
  UserX,
  Users,
  Wallet,
} from "lucide-react";
import { listEmployees, upcomingBirthdays } from "@/lib/db";
import type { Employee } from "@/lib/types";
import { EmptyState } from "@/components/ui/EmptyState";
import { Card, PageHeader, SectionHeader } from "@/components/ui/PageHeader";
import { StatCard } from "@/components/ui/StatCard";
import { ROUTES } from "@/lib/types/routes";

export default function HrmDashboardPage() {
  const [employees, setEmployees] = useState<Employee[]>([]);

  useEffect(() => {
    listEmployees().then(setEmployees);
  }, []);

  const activeEmployees = employees.filter((employee) => employee.active);
  const birthdays = upcomingBirthdays(activeEmployees, 30);

  return (
    <div className="flex flex-col gap-6 pb-8">
      <PageHeader
        title="HRM Dashboard"
        description="Employees, attendance and payroll at a glance."
        breadcrumbs={[{ label: "HRM" }]}
      />

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Total Employees"
          value={String(activeEmployees.length)}
          icon={<Users size={20} />}
          accent="primary"
          href={ROUTES.hrm.employees.root}
        />
        <StatCard
          label="Upcoming Birthdays"
          value={String(birthdays.length)}
          sub="Next 30 days"
          icon={<Cake size={20} />}
          accent="secondary"
        />
        <StatCard
          label="Present Today"
          value="—"
          sub="Available once Attendance is live"
          icon={<Clock size={20} />}
          accent="success"
        />
        <StatCard
          label="Absent Today"
          value="—"
          sub="Available once Attendance is live"
          icon={<UserX size={20} />}
          accent="error"
        />
        <StatCard
          label="Late Today"
          value="—"
          sub="Available once Attendance is live"
          icon={<AlarmClock size={20} />}
          accent="warning"
        />
        <StatCard
          label="Employees On Leave"
          value="—"
          sub="Available once Leave Management is live"
          icon={<CalendarOff size={20} />}
          accent="info"
        />
        <StatCard
          label="Pending Leave Requests"
          value="—"
          sub="Available once Leave Management is live"
          icon={<Inbox size={20} />}
          accent="orange"
        />
        <StatCard
          label="Payroll Summary"
          value="—"
          sub="Available once Payroll is live"
          icon={<Wallet size={20} />}
          accent="secondary"
        />
      </div>

      {birthdays.length > 0 && (
        <div className="flex flex-col gap-3">
          <SectionHeader title="Upcoming birthdays" />
          <Card>
            <ul className="flex flex-col divide-y divide-outline-variant/50 dark:divide-zinc-800">
              {birthdays.map(({ employee, daysUntil }) => (
                <li
                  key={employee.id}
                  className="flex items-center justify-between gap-3 py-2.5 text-sm first:pt-0 last:pb-0"
                >
                  <span className="text-on-surface dark:text-zinc-50">
                    {employee.full_name}
                  </span>
                  <span className="text-on-surface-variant dark:text-zinc-400">
                    {daysUntil === 0 ? "Today" : `In ${daysUntil} day${daysUntil === 1 ? "" : "s"}`}
                  </span>
                </li>
              ))}
            </ul>
          </Card>
        </div>
      )}

      <div className="flex flex-col gap-3">
        <SectionHeader title="Charts" />
        <Card>
          <EmptyState
            icon={<BarChart3 size={20} />}
            title="Charts unlock with later phases"
            description="Attendance Trend, Payroll Summary and Leave Statistics charts will appear here once the Attendance, Payroll and Leave modules are built."
          />
        </Card>
      </div>
    </div>
  );
}
