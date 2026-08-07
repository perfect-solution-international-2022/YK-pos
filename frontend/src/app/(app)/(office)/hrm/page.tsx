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
import {
  activeLeaveToday,
  computeDailySummary,
  listAttendanceForDate,
  listEmployees,
  listLeaveRequests,
  listPayslipsForPeriod,
  listShifts,
  localDateKey,
  upcomingBirthdays,
  type DailyAttendanceSummary,
} from "@/lib/db";
import type { AttendanceRecord, Employee, LeaveRequest, Payslip, Shift } from "@/lib/types";
import { useSettings } from "@/lib/hooks/use-settings";
import { EmptyState } from "@/components/ui/EmptyState";
import { Card, PageHeader, SectionHeader } from "@/components/ui/PageHeader";
import { StatCard } from "@/components/ui/StatCard";
import { ROUTES } from "@/lib/types/routes";

const EMPTY_SUMMARY: DailyAttendanceSummary = {
  present: 0,
  late: 0,
  halfDay: 0,
  absent: 0,
  onLeave: 0,
};

export default function HrmDashboardPage() {
  const { money } = useSettings();
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [shifts, setShifts] = useState<Shift[]>([]);
  const [todayRecords, setTodayRecords] = useState<AttendanceRecord[]>([]);
  const [leaveRequests, setLeaveRequests] = useState<LeaveRequest[]>([]);
  const [periodPayslips, setPeriodPayslips] = useState<Payslip[]>([]);
  const [today, setToday] = useState("");

  useEffect(() => {
    const date = localDateKey(Date.now());
    const period = date.slice(0, 7);
    void Promise.all([
      listEmployees(),
      listShifts(),
      listAttendanceForDate(date),
      listLeaveRequests(),
      listPayslipsForPeriod(period),
    ]).then(([employeeList, shiftList, records, requests, payslips]) => {
      setToday(date);
      setEmployees(employeeList);
      setShifts(shiftList);
      setTodayRecords(records);
      setLeaveRequests(requests);
      setPeriodPayslips(payslips);
    });
  }, []);

  const activeEmployees = employees.filter((employee) => employee.active);
  const birthdays = upcomingBirthdays(activeEmployees, 30);
  const onLeaveIds = activeLeaveToday(leaveRequests, today);
  const summary =
    activeEmployees.length > 0
      ? computeDailySummary(today, activeEmployees, shifts, todayRecords, onLeaveIds)
      : EMPTY_SUMMARY;
  const pendingLeaveCount = leaveRequests.filter((request) => request.status === "pending").length;
  const payrollTotalCents = periodPayslips.reduce((sum, payslip) => sum + payslip.net_cents, 0);
  const payslipSuffix = periodPayslips.length === 1 ? "" : "s";
  const payrollValue = periodPayslips.length > 0 ? money(payrollTotalCents) : "—";
  const payrollSub =
    periodPayslips.length > 0
      ? `${periodPayslips.length} payslip${payslipSuffix} this month`
      : "No payslips generated for this month yet";

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
          value={String(summary.present + summary.late + summary.halfDay)}
          icon={<Clock size={20} />}
          accent="success"
          href={ROUTES.hrm.attendance}
        />
        <StatCard
          label="Absent Today"
          value={String(summary.absent)}
          icon={<UserX size={20} />}
          accent="error"
          href={ROUTES.hrm.attendance}
        />
        <StatCard
          label="Late Today"
          value={String(summary.late)}
          icon={<AlarmClock size={20} />}
          accent="warning"
          href={ROUTES.hrm.attendance}
        />
        <StatCard
          label="Employees On Leave"
          value={String(summary.onLeave)}
          icon={<CalendarOff size={20} />}
          accent="info"
          href={ROUTES.hrm.leave}
        />
        <StatCard
          label="Pending Leave Requests"
          value={String(pendingLeaveCount)}
          icon={<Inbox size={20} />}
          accent="orange"
          href={ROUTES.hrm.leave}
        />
        <StatCard
          label="Payroll Summary"
          value={payrollValue}
          sub={payrollSub}
          icon={<Wallet size={20} />}
          accent="secondary"
          href={ROUTES.hrm.payroll}
        />
      </div>

      {birthdays.length > 0 && (
        <div className="flex flex-col gap-3">
          <SectionHeader title="Upcoming birthdays" />
          <Card>
            <ul className="flex flex-col divide-y divide-outline-variant/50 dark:divide-zinc-800">
              {birthdays.map(({ employee, daysUntil }) => {
                const daySuffix = daysUntil === 1 ? "" : "s";
                const birthdayTiming =
                  daysUntil === 0 ? "Today" : `In ${daysUntil} day${daySuffix}`;
                return (
                  <li
                    key={employee.id}
                    className="flex items-center justify-between gap-3 py-2.5 text-sm first:pt-0 last:pb-0"
                  >
                    <span className="text-on-surface dark:text-zinc-50">
                      {employee.full_name}
                    </span>
                    <span className="text-on-surface-variant dark:text-zinc-400">
                      {birthdayTiming}
                    </span>
                  </li>
                );
              })}
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
