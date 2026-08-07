"use client";

import { Suspense, useState, type SubmitEvent } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/lib/hooks/use-auth";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { ROUTES } from "@/lib/types/routes";

function LoginForm() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
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
          Welcome back
        </h1>
        <p className="text-sm text-on-surface-variant dark:text-zinc-400">
          Sign in to reach your dashboard.
        </p>
      </div>

      {sessionExpired && (
        <output
          className="rounded-lg border-l-4 border-amber-500 bg-amber-500/10 px-3 py-2 text-sm text-amber-800 dark:text-amber-300"
        >
          Your session expired. Sign in again to continue.
        </output>
      )}

      <form onSubmit={handleSubmit} className="flex w-full flex-col gap-4">
        <Input
          label="Email"
          type="email"
          autoComplete="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          required
          autoFocus
        />
        <Input
          label="Password"
          type="password"
          autoComplete="current-password"
          revealToggle
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          required
        />
        <Link
          href={ROUTES.auth.forgotPassword}
          className="-mt-1 self-end text-xs font-medium text-primary hover:underline dark:text-green-400"
        >
          Forgot password?
        </Link>
        <Button
          type="submit"
          size="lg"
          fullWidth
          className="mt-1 rounded-full"
          disabled={submitting}
        >
          {submitting ? "Signing in…" : "Sign in"}
        </Button>
      </form>
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
