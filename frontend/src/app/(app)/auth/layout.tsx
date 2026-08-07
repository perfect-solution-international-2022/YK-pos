"use client";

import Image from "next/image";

export default function AuthLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <main className="relative flex flex-1 items-center justify-center overflow-hidden bg-surface p-4 sm:p-8 dark:bg-black">
      <div
        aria-hidden
        className="pointer-events-none absolute -top-32 -left-32 h-96 w-96 rounded-full bg-secondary/30 blur-3xl dark:bg-secondary/20"
      />
      <div
        aria-hidden
        className="pointer-events-none absolute -right-32 -bottom-32 h-96 w-96 rounded-full bg-primary/30 blur-3xl dark:bg-primary/25"
      />

      <div className="relative grid w-full max-w-4xl overflow-hidden rounded-3xl border border-outline-variant/60 bg-surface-container-lowest shadow-elevated md:grid-cols-2 dark:border-zinc-800 dark:bg-zinc-950">
        {/* form panel (left) */}
        <section className="flex flex-col gap-8 p-8 sm:p-10">{children}</section>

        {/* visual panel (right) */}
        <aside className="relative hidden overflow-hidden bg-gradient-to-br from-primary to-secondary text-on-primary md:block">
          <Image
            src="/login%20bg.jpeg"
            alt=""
            fill
            priority
            className="object-cover object-center"
          />
        </aside>
      </div>
    </main>
  );
}
