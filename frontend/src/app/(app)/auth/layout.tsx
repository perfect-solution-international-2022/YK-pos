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
      <aside className="relative hidden w-1/2 overflow-hidden md:block">
        <Image
          src="/login%20bg.jpeg"
          alt=""
          fill
          priority
          className="object-cover object-center"
        />
      </aside>

      {/* form panel (right) */}
      <section className="flex w-full flex-1 items-center justify-center bg-surface p-4 sm:p-8 dark:bg-black">
        <div className="w-full max-w-md rounded-3xl border border-outline-variant/60 bg-surface-container-lowest p-10 shadow-elevated dark:border-zinc-800 dark:bg-zinc-950">
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
