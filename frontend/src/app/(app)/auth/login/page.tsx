"use client";

import { Suspense, useState, type SubmitEvent } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/lib/hooks/use-auth";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { ROUTES } from "@/lib/types/routes";

function MailIcon() {
  return (
    <svg
      aria-hidden
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="h-4 w-4"
    >
      <rect x="2" y="4" width="20" height="16" rx="2" />
      <path d="m2 7 10 6 10-6" />
    </svg>
  );
}

function LockIcon() {
  return (
    <svg
      aria-hidden
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="h-4 w-4"
    >
      <rect x="3" y="11" width="18" height="10" rx="2" />
      <path d="M7 11V7a5 5 0 0 1 10 0v4" />
    </svg>
  );
}

function ArrowIcon() {
  return (
    <svg
      aria-hidden
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="h-4 w-4"
    >
      <path d="M5 12h14M13 6l6 6-6 6" />
    </svg>
  );
}

function LoginForm() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [rememberMe, setRememberMe] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const { login } = useAuth();
  const router = useRouter();
  const searchParams = useSearchParams();
  const { showToast } = useToast();

  // RequireAuth appends ?reason=expired when it ends a stale session, so the
  // user is told why they landed back here rather than left guessing.
  const sessionExpired = searchParams.get("reason") === "expired";

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    try {
      await login(email, password);
      router.push(ROUTES.dashboard);
    } catch {
      showToast("Login failed — check email and password", "error");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="flex w-full flex-col gap-6">
      <div className="flex flex-col gap-1 text-center">
        <h1 className="font-display text-2xl font-semibold tracking-tight text-on-surface dark:text-zinc-50">
          Sign In
        </h1>
        <p className="text-sm text-on-surface-variant dark:text-zinc-400">
          Access your dashboard and manage everything from one place.
        </p>
      </div>

      {sessionExpired && (
        <output className="rounded-lg border-l-4 border-amber-500 bg-amber-500/10 px-3 py-2 text-sm text-amber-800 dark:text-amber-300">
          Your session expired. Sign in again to continue.
        </output>
      )}

      <form onSubmit={handleSubmit} className="flex w-full flex-col gap-4">
        <Input
          label="Email address"
          type="email"
          autoComplete="email"
          leading={<MailIcon />}
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          required
          autoFocus
        />
        <Input
          label="Password"
          type="password"
          autoComplete="current-password"
          leading={<LockIcon />}
          revealToggle
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          required
        />

        <div className="flex items-center justify-between text-sm">
          <label className="flex items-center gap-2 text-on-surface-variant dark:text-zinc-400">
            <input
              type="checkbox"
              checked={rememberMe}
              onChange={(event) => setRememberMe(event.target.checked)}
              className="h-4 w-4 rounded border-outline-variant text-primary focus:ring-primary/40 dark:border-zinc-700"
            />
            Remember me
          </label>
          <Link
            href={ROUTES.auth.forgotPassword}
            className="font-medium text-primary hover:underline dark:text-green-400"
          >
            Forgot password?
          </Link>
        </div>

        <Button
          type="submit"
          size="lg"
          fullWidth
          className="mt-1"
          disabled={submitting}
        >
          {submitting ? "Signing in…" : "Sign in"}
          {!submitting && <ArrowIcon />}
        </Button>
      </form>

      <p className="text-center text-xs text-on-surface-variant dark:text-zinc-500">
        © {new Date().getFullYear()} YK Grocery Mart. All rights reserved.
      </p>
    </div>
  );
}

export default function LoginPage() {
  // useSearchParams needs a Suspense boundary so this route can still be
  // statically prerendered.
  return (
    <Suspense
      fallback={
        <p className="text-sm text-on-surface-variant dark:text-zinc-400">
          Loading…
        </p>
      }
    >
      <LoginForm />
    </Suspense>
  );
}
