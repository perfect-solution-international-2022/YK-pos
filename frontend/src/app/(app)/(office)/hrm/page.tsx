"use client";

import { useEffect, useState } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  AlarmClock,
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
  listAttendanceInRange,
  listEmployees,
  listLeaveRequests,
  listLeaveTypes,
  listPayslips,
  listPayslipsForPeriod,
  listShifts,
  localDateKey,
  upcomingBirthdays,
  type DailyAttendanceSummary,
} from "@/lib/db";
import type {
  AttendanceRecord,
  Employee,
  LeaveRequest,
  LeaveType,
  Payslip,
  Shift,
} from "@/lib/types";
import { useSettings } from "@/lib/hooks/use-settings";
import { useTheme } from "@/components/providers/theme-provider";
import { Card, PageHeader, SectionHeader } from "@/components/ui/PageHeader";
import { EmptyState } from "@/components/ui/EmptyState";
import { StatCard } from "@/components/ui/StatCard";
import { ROUTES } from "@/lib/types/routes";

const EMPTY_SUMMARY: DailyAttendanceSummary = {
  present: 0,
  late: 0,
  halfDay: 0,
  absent: 0,
  onLeave: 0,
};

const TREND_DAYS = 14;
const PAYROLL_MONTHS = 6;

interface HrmChartPalette {
  present: string;
  late: string;
  absent: string;
  grid: string;
  ink: string;
  muted: string;
  sliceColors: string[];
}

// Same light/dark-keyed shape as the main dashboard's CHART_COLORS
// (`dashboard/page.tsx`), with hues reused from there for visual consistency
// across the two dashboards.
const HRM_CHART_COLORS: Record<"light" | "dark", HrmChartPalette> = {
  light: {
    present: "#2e7d32",
    late: "#c98500",
    absent: "#dc2626",
    grid: "#dee7da",
    ink: "#44513f",
    muted: "#8b9187",
    sliceColors: ["#2e7d32", "#c98500", "#5c9e2f", "#8a6206", "#8b9187"],
  },
  dark: {
    present: "#6cc16f",
    late: "#f2c230",
    absent: "#f87171",
    grid: "#2b3f28",
    ink: "#cbdfc6",
    muted: "#82a37c",
    sliceColors: ["#6cc16f", "#f2c230", "#a8d84f", "#d19a1f", "#82a37c"],
  },
};

function lastNDays(todayKey: string, count: number): string[] {
  const anchor = new Date(`${todayKey}T00:00:00`);
  return Array.from({ length: count }, (_, index) => {
    const day = new Date(anchor);
    day.setDate(day.getDate() - (count - 1 - index));
    return localDateKey(day.getTime());
  });
}

function shortDayLabel(dateKey: string): string {
  return new Date(`${dateKey}T00:00:00`).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
  });
}

function shortMonthLabel(period: string): string {
  const [year, month] = period.split("-").map(Number);
  return new Date(year, month - 1, 1).toLocaleDateString(undefined, {
    month: "short",
    year: "2-digit",
  });
}

function lastNPeriods(todayKey: string, count: number): string[] {
  const anchor = new Date(`${todayKey}T00:00:00`);
  return Array.from({ length: count }, (_, index) => {
    const month = new Date(anchor.getFullYear(), anchor.getMonth() - (count - 1 - index), 1);
    return `${month.getFullYear()}-${String(month.getMonth() + 1).padStart(2, "0")}`;
  });
}

interface AttendanceTrendChartProps {
  today: string;
  employees: Employee[];
  shifts: Shift[];
  records: AttendanceRecord[];
  leaveRequests: LeaveRequest[];
  colors: HrmChartPalette;
}

function AttendanceTrendChart({
  today,
  employees,
  shifts,
  records,
  leaveRequests,
  colors,
}: Readonly<AttendanceTrendChartProps>) {
  const days = today ? lastNDays(today, TREND_DAYS) : [];
  const recordsByDate = new Map<string, AttendanceRecord[]>();
  for (const record of records) {
    const bucket = recordsByDate.get(record.date) ?? [];
    bucket.push(record);
    recordsByDate.set(record.date, bucket);
  }

  const data = days.map((day) => {
    const summary = computeDailySummary(
      day,
      employees,
      shifts,
      recordsByDate.get(day) ?? [],
      activeLeaveToday(leaveRequests, day),
    );
    return {
      label: shortDayLabel(day),
      Present: summary.present + summary.halfDay,
      Late: summary.late,
      Absent: summary.absent,
    };
  });

  return (
    <Card>
      <h3 className="mb-4 text-sm font-semibold text-on-surface dark:text-zinc-50">
        Attendance Trend
      </h3>
      <div className="h-64 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} barGap={4} barCategoryGap="20%">
            <CartesianGrid vertical={false} stroke={colors.grid} />
            <XAxis
              dataKey="label"
              tick={{ fill: colors.muted, fontSize: 11 }}
              axisLine={{ stroke: colors.grid }}
              tickLine={false}
              interval={1}
            />
            <YAxis
              tick={{ fill: colors.muted, fontSize: 11 }}
              axisLine={false}
              tickLine={false}
              allowDecimals={false}
              width={28}
            />
            <Tooltip
              contentStyle={{ fontSize: 12 }}
              cursor={{ fill: colors.grid, opacity: 0.4 }}
            />
            <Legend wrapperStyle={{ fontSize: 12, color: colors.ink }} />
            <Bar dataKey="Present" stackId="a" fill={colors.present} />
            <Bar dataKey="Late" stackId="a" fill={colors.late} />
            <Bar dataKey="Absent" stackId="a" fill={colors.absent} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </Card>
  );
}

interface PayrollSummaryChartProps {
  today: string;
  payslips: Payslip[];
  colors: HrmChartPalette;
  money: (cents: number) => string;
}

function PayrollSummaryChart({
  today,
  payslips,
  colors,
  money,
}: Readonly<PayrollSummaryChartProps>) {
  const periods = today ? lastNPeriods(today, PAYROLL_MONTHS) : [];
  const netByPeriod = new Map<string, number>();
  for (const payslip of payslips) {
    netByPeriod.set(payslip.period, (netByPeriod.get(payslip.period) ?? 0) + payslip.net_cents);
  }
  const data = periods.map((period) => ({
    label: shortMonthLabel(period),
    Net: netByPeriod.get(period) ?? 0,
  }));

  return (
    <Card>
      <h3 className="mb-4 text-sm font-semibold text-on-surface dark:text-zinc-50">
        Payroll Summary
      </h3>
      <div className="h-64 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} barCategoryGap="30%">
            <CartesianGrid vertical={false} stroke={colors.grid} />
            <XAxis
              dataKey="label"
              tick={{ fill: colors.muted, fontSize: 11 }}
              axisLine={{ stroke: colors.grid }}
              tickLine={false}
            />
            <YAxis
              tick={{ fill: colors.muted, fontSize: 11 }}
              axisLine={false}
              tickLine={false}
              tickFormatter={(value: number) => money(value)}
              width={70}
            />
            <Tooltip
              formatter={(value) => money(Number(value))}
              contentStyle={{ fontSize: 12 }}
              cursor={{ fill: colors.grid, opacity: 0.4 }}
            />
            <Bar dataKey="Net" fill={colors.present} maxBarSize={32} radius={[4, 4, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </Card>
  );
}

interface LeaveStatisticsChartProps {
  today: string;
  leaveTypes: LeaveType[];
  leaveRequests: LeaveRequest[];
  colors: HrmChartPalette;
}

function LeaveStatisticsChart({
  today,
  leaveTypes,
  leaveRequests,
  colors,
}: Readonly<LeaveStatisticsChartProps>) {
  const year = today ? Number(today.slice(0, 4)) : new Date().getFullYear();
  const data = leaveTypes
    .map((leaveType, index) => {
      const days = leaveRequests
        .filter(
          (request) =>
            request.leave_type_id === leaveType.id &&
            request.status === "approved" &&
            Number(request.start_date.slice(0, 4)) === year,
        )
        .reduce((sum, request) => sum + request.days, 0);
      return {
        name: leaveType.name,
        value: days,
        fill: colors.sliceColors[index % colors.sliceColors.length],
      };
    })
    .filter((slice) => slice.value > 0);

  return (
    <Card>
      <h3 className="mb-4 text-sm font-semibold text-on-surface dark:text-zinc-50">
        Leave Statistics
      </h3>
      {data.length === 0 ? (
        <EmptyState title="No approved leave taken this year" />
      ) : (
        <div className="h-64 w-full">
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={data}
                dataKey="value"
                nameKey="name"
                innerRadius="55%"
                outerRadius="85%"
                paddingAngle={2}
                stroke="none"
              />
              <Tooltip formatter={(value) => `${value} days`} contentStyle={{ fontSize: 12 }} />
              <Legend
                layout="vertical"
                align="right"
                verticalAlign="middle"
                wrapperStyle={{ fontSize: 12, color: colors.ink }}
              />
            </PieChart>
          </ResponsiveContainer>
        </div>
      )}
    </Card>
  );
}

export default function HrmDashboardPage() {
  const { money } = useSettings();
  const { resolvedTheme } = useTheme();
  const colors = HRM_CHART_COLORS[resolvedTheme];

  const [employees, setEmployees] = useState<Employee[]>([]);
  const [shifts, setShifts] = useState<Shift[]>([]);
  const [todayRecords, setTodayRecords] = useState<AttendanceRecord[]>([]);
  const [trendRecords, setTrendRecords] = useState<AttendanceRecord[]>([]);
  const [leaveRequests, setLeaveRequests] = useState<LeaveRequest[]>([]);
  const [leaveTypes, setLeaveTypes] = useState<LeaveType[]>([]);
  const [periodPayslips, setPeriodPayslips] = useState<Payslip[]>([]);
  const [allPayslips, setAllPayslips] = useState<Payslip[]>([]);
  const [today, setToday] = useState("");

  useEffect(() => {
    const date = localDateKey(Date.now());
    const period = date.slice(0, 7);
    const trendEnd = new Date();
    trendEnd.setHours(23, 59, 59, 999);
    const trendStart = new Date();
    trendStart.setHours(0, 0, 0, 0);
    trendStart.setDate(trendStart.getDate() - (TREND_DAYS - 1));

    void Promise.all([
      listEmployees(),
      listShifts(),
      listAttendanceForDate(date),
      listAttendanceInRange({ from: trendStart.getTime(), to: trendEnd.getTime() }),
      listLeaveRequests(),
      listLeaveTypes(),
      listPayslipsForPeriod(period),
      listPayslips(),
    ]).then(
      ([
        employeeList,
        shiftList,
        records,
        trend,
        requests,
        types,
        payslips,
        everyPayslip,
      ]) => {
        setToday(date);
        setEmployees(employeeList);
        setShifts(shiftList);
        setTodayRecords(records);
        setTrendRecords(trend);
        setLeaveRequests(requests);
        setLeaveTypes(types.filter((leaveType) => leaveType.active));
        setPeriodPayslips(payslips);
        setAllPayslips(everyPayslip);
      },
    );
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
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <AttendanceTrendChart
            today={today}
            employees={activeEmployees}
            shifts={shifts}
            records={trendRecords}
            leaveRequests={leaveRequests}
            colors={colors}
          />
          <PayrollSummaryChart today={today} payslips={allPayslips} colors={colors} money={money} />
          <LeaveStatisticsChart
            today={today}
            leaveTypes={leaveTypes}
            leaveRequests={leaveRequests}
            colors={colors}
          />
        </div>
      </div>
    </div>
  );
}
