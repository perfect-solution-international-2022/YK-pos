"use client";

import Image from "next/image";

const FEATURES = [
  "Real-time inventory tracking",
  "Multi-location POS support",
  "Advanced reporting & analytics",
];

function CheckIcon() {
  return (
    <svg
      aria-hidden
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="h-3 w-3"
    >
      <path d="M20 6 9 17l-5-5" />
    </svg>
  );
}

export default function AuthLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <main className="flex min-h-screen w-full">
      {/* visual panel (left) */}
      <aside className="relative hidden w-1/2 flex-col justify-center gap-8 overflow-hidden bg-gradient-to-br from-primary to-secondary p-16 text-on-primary md:flex">
        <div
          aria-hidden
          className="pointer-events-none absolute -top-24 -right-16 h-72 w-72 rounded-full bg-white/10 blur-3xl"
        />
        <div
          aria-hidden
          className="pointer-events-none absolute -bottom-24 -left-16 h-80 w-80 rounded-full bg-white/10 blur-3xl"
        />

        <span className="relative w-max rounded-full bg-white/15 px-3 py-1 text-xs font-medium backdrop-blur">
          <span className="mr-1.5 inline-block h-1.5 w-1.5 rounded-full bg-emerald-300 align-middle" />
          Secure &amp; Reliable
        </span>

        <div className="relative flex flex-col gap-3">
          <h1 className="font-display text-4xl font-bold tracking-tight">
            Welcome back!
          </h1>
          <p className="max-w-sm text-on-primary/80">
            Sign in to access your account and keep your operations in sync.
          </p>
        </div>

        <ul className="relative flex flex-col gap-3">
          {FEATURES.map((feature) => (
            <li key={feature} className="flex items-center gap-3 text-sm">
              <span className="grid h-5 w-5 shrink-0 place-items-center rounded-full bg-white/20">
                <CheckIcon />
              </span>
              {feature}
            </li>
          ))}
        </ul>
      </aside>

      {/* form panel (right) */}
      <section className="relative flex w-full flex-1 items-center justify-center overflow-hidden p-4 sm:p-8">
        <Image
          src="/login%20bg.jpeg"
          alt=""
          fill
          priority
          className="object-cover object-center"
        />
        <div
          aria-hidden
          className="absolute inset-0 bg-surface/70 dark:bg-black/70"
        />
        <div className="relative w-full max-w-md rounded-3xl border border-outline-variant/60 bg-surface-container-lowest p-10 shadow-elevated dark:border-zinc-800 dark:bg-zinc-950">
          <div className="mb-6 flex justify-center">
            <Image
              src="/logo.png"
              alt="Logo"
              width={48}
              height={48}
              className="rounded-full"
            />
          </div>
          {children}
        </div>
      </section>
    </main>
  );
}
