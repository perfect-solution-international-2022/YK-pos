"use client";

import { useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import Image from "next/image";
import { Check, FileText, Trash2, UploadCloud, X } from "lucide-react";
import { PIN_PATTERN, listDesignations, listRoles, listShifts } from "@/lib/db";
import type {
  Designation,
  EmployeeDocument,
  EmployeeGender,
  EmploymentType,
  Role,
  Shift,
} from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { NumberField } from "@/components/ui/NumberField";
import { Select } from "@/components/ui/Select";
import { Switch } from "@/components/ui/Switch";
import { Textarea } from "@/components/ui/Textarea";

const MAX_PHOTO_BYTES = 2 * 1024 * 1024;
const ACCEPTED_PHOTO_TYPES = ["image/jpeg", "image/png", "image/webp"];
const MAX_DOCUMENT_BYTES = 5 * 1024 * 1024;
const MAX_DOCUMENTS = 10;

const GENDER_OPTIONS: { value: EmployeeGender; label: string }[] = [
  { value: "male", label: "Male" },
  { value: "female", label: "Female" },
  { value: "other", label: "Other" },
];

const EMPLOYMENT_TYPE_OPTIONS: { value: EmploymentType; label: string }[] = [
  { value: "full_time", label: "Full time" },
  { value: "part_time", label: "Part time" },
  { value: "contract", label: "Contract" },
  { value: "intern", label: "Intern" },
];

export interface EmployeeFormValues {
  full_name: string;
  nic: string;
  date_of_birth: string;
  gender: EmployeeGender | "";
  phone: string;
  email: string;
  address: string;
  emergency_contact_name: string;
  emergency_contact_phone: string;

  designation_id: string;
  joining_date: string;
  employment_type: EmploymentType | "";
  shift_id: string;

  basic_salary: string;
  bank_name: string;
  bank_account_no: string;
  bank_branch: string;

  enable_login: boolean;
  username: string;
  password: string;
  role_id: string;

  photo: string;
  documents: EmployeeDocument[];
  active: boolean;
}

export const EMPTY_EMPLOYEE_FORM: EmployeeFormValues = {
  full_name: "",
  nic: "",
  date_of_birth: "",
  gender: "",
  phone: "",
  email: "",
  address: "",
  emergency_contact_name: "",
  emergency_contact_phone: "",
  designation_id: "",
  joining_date: "",
  employment_type: "",
  shift_id: "",
  basic_salary: "",
  bank_name: "",
  bank_account_no: "",
  bank_branch: "",
  enable_login: false,
  username: "",
  password: "",
  role_id: "",
  photo: "",
  documents: [],
  active: true,
};

interface EmployeeFormProps {
  mode: "create" | "edit";
  initialValues: EmployeeFormValues;
  onSubmit: (values: EmployeeFormValues) => Promise<void>;
  onCancel: () => void;
}

function readAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(new Error(`Could not read ${file.name}`));
    reader.readAsDataURL(file);
  });
}

function formatMb(bytes: number): string {
  return `${(bytes / (1024 * 1024)).toFixed(0)} MB`;
}

function RequiredLabel({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <>
      {children} <span className="text-error">*</span>
    </>
  );
}

function FormSection({
  title,
  children,
}: Readonly<{ title: string; children: ReactNode }>) {
  return (
    <section className="flex flex-col rounded-xl border border-outline-variant bg-surface-container-lowest dark:border-zinc-800 dark:bg-zinc-900">
      <h2 className="rounded-t-xl bg-surface-container-low px-4 py-3 text-sm font-semibold text-on-surface dark:bg-zinc-800/60 dark:text-zinc-100">
        {title}
      </h2>
      <div className="grid gap-4 p-4 sm:grid-cols-2">{children}</div>
    </section>
  );
}

// One form behind both /hrm/employees/new and /hrm/employees/[id]/edit — the
// two screens differ only in whether a till login is mandatory, so keeping
// this as one component stops a field added to create from quietly going
// missing on edit (see UserForm, which this mirrors).
export function EmployeeForm({
  mode,
  initialValues,
  onSubmit,
  onCancel,
}: Readonly<EmployeeFormProps>) {
  const photoInputRef = useRef<HTMLInputElement>(null);
  const documentInputRef = useRef<HTMLInputElement>(null);
  const [designations, setDesignations] = useState<Designation[]>([]);
  const [shifts, setShifts] = useState<Shift[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [values, setValues] = useState<EmployeeFormValues>(initialValues);
  const [photoName, setPhotoName] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    void (async () => {
      const [designationList, shiftList, roleList] = await Promise.all([
        listDesignations(),
        listShifts(),
        listRoles(),
      ]);
      setDesignations(designationList);
      setShifts(shiftList);
      setRoles(roleList);
    })();
  }, []);

  function set<K extends keyof EmployeeFormValues>(key: K, value: EmployeeFormValues[K]) {
    setValues((current) => ({ ...current, [key]: value }));
  }

  async function handlePhotoChange(file: File | undefined) {
    if (!file) {
      set("photo", "");
      setPhotoName("");
      return;
    }
    if (!ACCEPTED_PHOTO_TYPES.includes(file.type)) {
      setErrors((current) => ({ ...current, photo: "Use a JPG, PNG or WebP image." }));
      return;
    }
    if (file.size > MAX_PHOTO_BYTES) {
      setErrors((current) => ({
        ...current,
        photo: `Image must be under ${formatMb(MAX_PHOTO_BYTES)}.`,
      }));
      return;
    }
    setErrors((current) => ({ ...current, photo: "" }));
    set("photo", await readAsDataUrl(file));
    setPhotoName(file.name);
  }

  async function handleDocumentsChange(files: FileList | null) {
    if (!files || files.length === 0) return;
    const room = MAX_DOCUMENTS - values.documents.length;
    if (room <= 0) {
      setErrors((current) => ({
        ...current,
        documents: `Only ${MAX_DOCUMENTS} documents per employee.`,
      }));
      return;
    }

    const accepted: File[] = [];
    for (const file of Array.from(files)) {
      if (file.size > MAX_DOCUMENT_BYTES) {
        setErrors((current) => ({
          ...current,
          documents: `${file.name}: over the ${formatMb(MAX_DOCUMENT_BYTES)} limit`,
        }));
        continue;
      }
      accepted.push(file);
    }

    const usable = accepted.slice(0, room);
    if (usable.length === 0) return;
    const uploaded = await Promise.all(
      usable.map(async (file) => ({
        name: file.name,
        data_url: await readAsDataUrl(file),
        size: file.size,
      })),
    );
    setErrors((current) => ({ ...current, documents: "" }));
    set("documents", [...values.documents, ...uploaded]);
  }

  function removeDocument(index: number) {
    set(
      "documents",
      values.documents.filter((_, position) => position !== index),
    );
  }

  function validate(): Record<string, string> {
    const next: Record<string, string> = {};
    if (!values.full_name.trim()) next.full_name = "Full name is required.";
    if (!values.nic.trim()) next.nic = "NIC is required.";
    if (!values.date_of_birth) next.date_of_birth = "Date of birth is required.";
    if (!values.gender) next.gender = "Pick a gender.";
    if (!values.phone.trim()) next.phone = "Phone is required.";
    if (!values.designation_id) next.designation_id = "Pick a designation.";
    if (!values.joining_date) next.joining_date = "Joining date is required.";
    if (!values.employment_type) next.employment_type = "Pick an employment type.";
    if (!values.shift_id) next.shift_id = "Pick a shift.";
    if (!values.basic_salary.trim() || Number.isNaN(Number(values.basic_salary))) {
      next.basic_salary = "Enter a valid salary.";
    } else if (Number(values.basic_salary) < 0) {
      next.basic_salary = "Salary cannot be negative.";
    }

    if (values.enable_login) {
      if (!values.email.trim()) next.email = "Email is required to create a login.";
      if (!values.username.trim()) next.username = "Username is required.";
      if (!values.role_id) next.role_id = "Pick a role.";
      if (mode === "create" || values.password) {
        if (!PIN_PATTERN.test(values.password)) next.password = "Use 4 to 6 digits.";
      }
    }

    return next;
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    const found = validate();
    setErrors(found);
    if (Object.keys(found).length > 0) return;

    setSaving(true);
    try {
      await onSubmit(values);
    } catch (caught) {
      setErrors({
        submit: caught instanceof Error ? caught.message : "Failed to save employee",
      });
    } finally {
      setSaving(false);
    }
  }

  return (
    <form
      onSubmit={handleSubmit}
      noValidate
      className="flex flex-col gap-4 rounded-2xl border border-outline-variant bg-surface-container-lowest p-4 sm:p-6 dark:border-zinc-800 dark:bg-zinc-900"
    >
      <FormSection title="Personal">
        <Input
          name="full_name"
          label={<RequiredLabel>Full Name</RequiredLabel>}
          value={values.full_name}
          onChange={(event) => set("full_name", event.target.value)}
          error={errors.full_name}
          autoFocus
        />
        <Input
          name="nic"
          label={<RequiredLabel>NIC</RequiredLabel>}
          value={values.nic}
          onChange={(event) => set("nic", event.target.value)}
          error={errors.nic}
        />
        <Input
          type="date"
          name="date_of_birth"
          label={<RequiredLabel>Date of Birth</RequiredLabel>}
          value={values.date_of_birth}
          onChange={(event) => set("date_of_birth", event.target.value)}
          error={errors.date_of_birth}
        />
        <Select
          name="gender"
          label={<RequiredLabel>Gender</RequiredLabel>}
          placeholder="Please select"
          value={values.gender}
          onChange={(event) => set("gender", event.target.value as EmployeeGender)}
          options={GENDER_OPTIONS}
          error={errors.gender}
        />
        <Input
          type="tel"
          inputMode="tel"
          name="phone"
          label={<RequiredLabel>Phone</RequiredLabel>}
          value={values.phone}
          onChange={(event) => set("phone", event.target.value)}
          error={errors.phone}
        />
        <Input
          type="email"
          name="email"
          label="Email"
          value={values.email}
          onChange={(event) => set("email", event.target.value)}
          error={errors.email}
          hint="Required only if this employee also gets a till login."
        />
        <Textarea
          label="Address"
          value={values.address}
          onChange={(event) => set("address", event.target.value)}
          rows={2}
          className="sm:col-span-2"
        />
        <Input
          name="emergency_contact_name"
          label="Emergency Contact Name"
          value={values.emergency_contact_name}
          onChange={(event) => set("emergency_contact_name", event.target.value)}
        />
        <Input
          type="tel"
          inputMode="tel"
          name="emergency_contact_phone"
          label="Emergency Contact Phone"
          value={values.emergency_contact_phone}
          onChange={(event) => set("emergency_contact_phone", event.target.value)}
        />
      </FormSection>

      <FormSection title="Employment">
        <Select
          name="designation_id"
          label={<RequiredLabel>Designation</RequiredLabel>}
          placeholder="Please select"
          value={values.designation_id}
          onChange={(event) => set("designation_id", event.target.value)}
          options={designations.map((designation) => ({
            value: designation.id,
            label: designation.name,
          }))}
          error={errors.designation_id}
        />
        <Input
          type="date"
          name="joining_date"
          label={<RequiredLabel>Joining Date</RequiredLabel>}
          value={values.joining_date}
          onChange={(event) => set("joining_date", event.target.value)}
          error={errors.joining_date}
        />
        <Select
          name="employment_type"
          label={<RequiredLabel>Employment Type</RequiredLabel>}
          placeholder="Please select"
          value={values.employment_type}
          onChange={(event) => set("employment_type", event.target.value as EmploymentType)}
          options={EMPLOYMENT_TYPE_OPTIONS}
          error={errors.employment_type}
        />
        <Select
          name="shift_id"
          label={<RequiredLabel>Shift</RequiredLabel>}
          placeholder="Please select"
          value={values.shift_id}
          onChange={(event) => set("shift_id", event.target.value)}
          options={shifts.map((shift) => ({ value: shift.id, label: shift.name }))}
          error={errors.shift_id}
        />
      </FormSection>

      <FormSection title="Salary">
        <NumberField
          label="Basic Salary"
          value={values.basic_salary}
          onChange={(value) => set("basic_salary", value)}
          precision={2}
          error={errors.basic_salary}
        />
        <Input
          name="bank_name"
          label="Bank Name"
          value={values.bank_name}
          onChange={(event) => set("bank_name", event.target.value)}
        />
        <Input
          name="bank_account_no"
          label="Bank Account No."
          value={values.bank_account_no}
          onChange={(event) => set("bank_account_no", event.target.value)}
        />
        <Input
          name="bank_branch"
          label="Bank Branch"
          value={values.bank_branch}
          onChange={(event) => set("bank_branch", event.target.value)}
        />
      </FormSection>

      <FormSection title="System">
        <div className="sm:col-span-2">
          <Switch
            checked={values.enable_login}
            onChange={(checked) => set("enable_login", checked)}
            label="Create a till / back-office login"
            description="Optional — skip this for employees who never sign in to the POS."
          />
        </div>
        {values.enable_login && (
          <>
            <Input
              name="username"
              label={<RequiredLabel>Username</RequiredLabel>}
              autoComplete="off"
              value={values.username}
              onChange={(event) => set("username", event.target.value)}
              error={errors.username}
            />
            <Select
              name="role_id"
              label={<RequiredLabel>Role</RequiredLabel>}
              placeholder="Please select"
              value={values.role_id}
              onChange={(event) => set("role_id", event.target.value)}
              options={roles.map((role) => ({ value: role.id, label: role.name }))}
              error={errors.role_id}
            />
            <Input
              name="password"
              label={
                mode === "create" ? (
                  <RequiredLabel>Password</RequiredLabel>
                ) : (
                  "New Password"
                )
              }
              placeholder={
                mode === "edit" ? "Leave blank to keep the current PIN" : undefined
              }
              type="password"
              inputMode="numeric"
              maxLength={6}
              revealToggle
              autoComplete="new-password"
              value={values.password}
              onChange={(event) =>
                set("password", event.target.value.replace(/\D/g, "").slice(0, 6))
              }
              error={errors.password}
              hint="4 to 6 digits — this is the till sign-in PIN."
            />
          </>
        )}
        <div className="flex items-end">
          <Switch checked={values.active} onChange={(checked) => set("active", checked)} label="Status: Active" />
        </div>
      </FormSection>

      <FormSection title="Other">
        <div className="flex flex-col gap-1">
          <span className="text-sm font-medium text-on-surface-variant dark:text-zinc-300">
            Photo
          </span>
          <div className="flex items-center gap-3">
            {values.photo && (
              <Image
                src={values.photo}
                alt=""
                width={48}
                height={48}
                unoptimized
                className="h-12 w-12 shrink-0 rounded-full object-cover"
              />
            )}
            <input
              ref={photoInputRef}
              type="file"
              accept={ACCEPTED_PHOTO_TYPES.join(",")}
              aria-label="Employee photo"
              onChange={(event) => void handlePhotoChange(event.target.files?.[0])}
              className="min-h-10 w-full cursor-pointer rounded-lg border border-outline-variant bg-surface-container-lowest px-3 py-2 text-sm text-on-surface-variant outline-none file:mr-3 file:cursor-pointer file:rounded-md file:border-0 file:bg-surface-container file:px-3 file:py-1.5 file:text-sm file:text-on-surface hover:border-outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-400 dark:file:bg-zinc-800 dark:file:text-zinc-100"
            />
            {values.photo && (
              <button
                type="button"
                aria-label="Remove photo"
                onClick={() => {
                  set("photo", "");
                  setPhotoName("");
                  if (photoInputRef.current) photoInputRef.current.value = "";
                }}
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-error/40 text-error transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-error/10 active:scale-90"
              >
                <X size={14} />
              </button>
            )}
          </div>
          {errors.photo ? (
            <span className="animate-fade-in text-xs text-error">{errors.photo}</span>
          ) : (
            <span className="text-xs text-on-surface-variant dark:text-zinc-500">
              {photoName || "No file chosen"} · JPG, PNG or WebP, under{" "}
              {formatMb(MAX_PHOTO_BYTES)}.
            </span>
          )}
        </div>

        <div className="flex flex-col gap-2">
          <span className="text-sm font-medium text-on-surface-variant dark:text-zinc-300">
            Documents
          </span>
          <input
            ref={documentInputRef}
            type="file"
            multiple
            aria-label="Employee documents"
            onChange={(event) => {
              void handleDocumentsChange(event.target.files);
              event.target.value = "";
            }}
            className="sr-only"
            disabled={values.documents.length >= MAX_DOCUMENTS}
          />
          <button
            type="button"
            onClick={() => documentInputRef.current?.click()}
            disabled={values.documents.length >= MAX_DOCUMENTS}
            className="inline-flex min-h-10 items-center gap-2 self-start rounded-lg border border-outline-variant px-4 text-sm font-medium text-on-surface transition-all duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:bg-surface-container active:scale-[0.97] disabled:pointer-events-none disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-100 dark:hover:bg-zinc-800"
          >
            <UploadCloud size={16} />
            Add documents
          </button>
          {values.documents.length > 0 && (
            <ul className="flex flex-col gap-1.5">
              {values.documents.map((document, index) => (
                <li
                  key={`${document.name}-${index}`}
                  className="flex items-center justify-between gap-2 rounded-lg border border-outline-variant bg-surface-container-lowest px-3 py-2 text-xs dark:border-zinc-700 dark:bg-zinc-900"
                >
                  <span className="flex min-w-0 items-center gap-2 text-on-surface dark:text-zinc-100">
                    <FileText size={14} className="shrink-0" aria-hidden />
                    <span className="truncate">{document.name}</span>
                  </span>
                  <button
                    type="button"
                    onClick={() => removeDocument(index)}
                    aria-label={`Remove ${document.name}`}
                    className="shrink-0 text-error transition-transform duration-[var(--duration-fast)] hover:scale-110 active:scale-90"
                  >
                    <Trash2 size={14} />
                  </button>
                </li>
              ))}
            </ul>
          )}
          {errors.documents ? (
            <span className="animate-fade-in text-xs text-error">{errors.documents}</span>
          ) : (
            <span className="text-xs text-on-surface-variant dark:text-zinc-500">
              Up to {MAX_DOCUMENTS} files, {formatMb(MAX_DOCUMENT_BYTES)} each.
            </span>
          )}
        </div>
      </FormSection>

      {errors.submit && (
        <p className="text-sm text-error" role="alert">
          {errors.submit}
        </p>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <Button type="submit" loading={saving}>
          <Check size={16} />
          Submit
        </Button>
        <Button type="button" variant="outline" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  );
}
