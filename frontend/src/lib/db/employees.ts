import { db } from "./index";
import { createStaffUser, setStaffPin, updateStaffUser } from "./users";
import type {
  Employee,
  EmployeeDocument,
  EmployeeGender,
  EmploymentType,
} from "@/lib/types";

// Local-only table, no server sync yet (no backend endpoint exists).

export async function listEmployees(): Promise<Employee[]> {
  return db.employees.orderBy("employee_code").toArray();
}

export async function getEmployee(id: string): Promise<Employee | undefined> {
  return db.employees.get(id);
}

async function nextEmployeeCode(): Promise<string> {
  const employees = await db.employees.toArray();
  const highest = employees.reduce((max, employee) => {
    const value = Number(employee.employee_code);
    return Number.isFinite(value) && value > max ? value : max;
  }, 1000);
  return String(highest + 1);
}

/** Present only when the form's "System" section was filled in. */
export interface EmployeeLoginInput {
  username: string;
  password: string;
  role_id: string;
}

export interface CreateEmployeeInput {
  full_name: string;
  nic: string;
  date_of_birth: string;
  gender: EmployeeGender;
  phone: string;
  email?: string;
  address?: string;
  emergency_contact_name?: string;
  emergency_contact_phone?: string;
  designation_id: string;
  joining_date: string;
  employment_type: EmploymentType;
  shift_id: string;
  basic_salary_cents: number;
  bank_name?: string;
  bank_account_no?: string;
  bank_branch?: string;
  photo?: string;
  documents: EmployeeDocument[];
  active: boolean;
  login?: EmployeeLoginInput;
}

export async function createEmployee(input: CreateEmployeeInput): Promise<Employee> {
  const { login, ...rest } = input;

  let staff_user_id: string | undefined;
  if (login) {
    if (!rest.email) throw new Error("Email is required to create a login");
    const staffUser = await createStaffUser({
      name: rest.full_name,
      username: login.username,
      email: rest.email,
      phone: rest.phone,
      role_id: login.role_id,
      pin: login.password,
      active: rest.active,
    });
    staff_user_id = staffUser.id;
  }

  const employee: Employee = {
    ...rest,
    id: crypto.randomUUID(),
    employee_code: await nextEmployeeCode(),
    staff_user_id,
    created_at: Date.now(),
  };
  await db.employees.add(employee);
  return employee;
}

export interface UpdateEmployeeInput extends Partial<Omit<CreateEmployeeInput, "login">> {
  login?: EmployeeLoginInput;
}

/**
 * `login` creates the linked `StaffUser` on first use and updates it on
 * every call after that — the employee record only ever stores the id.
 */
export async function updateEmployee(
  id: string,
  changes: UpdateEmployeeInput,
): Promise<void> {
  const employee = await db.employees.get(id);
  if (!employee) throw new Error("That employee no longer exists");
  const { login, ...rest } = changes;

  let staff_user_id = employee.staff_user_id;
  if (login) {
    const email = rest.email ?? employee.email;
    if (!email) throw new Error("Email is required to create a login");

    if (staff_user_id) {
      await updateStaffUser(staff_user_id, {
        name: rest.full_name ?? employee.full_name,
        username: login.username,
        email,
        phone: rest.phone ?? employee.phone,
        role_id: login.role_id,
      });
      if (login.password) await setStaffPin(staff_user_id, login.password);
    } else {
      if (!login.password) throw new Error("Password is required to create a login");
      const staffUser = await createStaffUser({
        name: rest.full_name ?? employee.full_name,
        username: login.username,
        email,
        phone: rest.phone ?? employee.phone,
        role_id: login.role_id,
        pin: login.password,
        active: rest.active ?? employee.active,
      });
      staff_user_id = staffUser.id;
    }
  }

  await db.employees.update(id, { ...rest, staff_user_id });
}

/**
 * Removes only the HR record. A linked till/back-office login is left in
 * place — losing an HR file must not silently lock someone out of the till;
 * that stays an explicit action on the Users screen.
 */
export async function deleteEmployee(id: string): Promise<void> {
  await db.employees.delete(id);
}

export interface UpcomingBirthday {
  employee: Employee;
  daysUntil: number;
}

function startOfDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function daysUntilNextBirthday(dateOfBirth: string, from: Date): number {
  const birth = new Date(dateOfBirth);
  if (Number.isNaN(birth.getTime())) return Number.POSITIVE_INFINITY;

  const today = startOfDay(from);
  const next = new Date(today.getFullYear(), birth.getMonth(), birth.getDate());
  if (next < today) next.setFullYear(today.getFullYear() + 1);
  return Math.round((next.getTime() - today.getTime()) / 86_400_000);
}

export function upcomingBirthdays(
  employees: Employee[],
  withinDays = 30,
  from: Date = new Date(),
): UpcomingBirthday[] {
  return employees
    .map((employee) => ({
      employee,
      daysUntil: daysUntilNextBirthday(employee.date_of_birth, from),
    }))
    .filter((entry) => entry.daysUntil <= withinDays)
    .sort((a, b) => a.daysUntil - b.daysUntil);
}
