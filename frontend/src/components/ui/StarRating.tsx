"use client";

import { Star } from "lucide-react";

interface StarRatingProps {
  value: number;
  /** Supplying this makes the stars clickable; omit for a read-only display. */
  onChange?: (value: number) => void;
  max?: number;
  size?: number;
  label?: string;
}

// Native <select>-style controls don't suit a 1-5 star pick, but the whole
// point of a star row is a large, obvious click target — same touch-target
// reasoning as NumberField's +/- buttons.
export function StarRating({
  value,
  onChange,
  max = 5,
  size = 20,
  label = "Rating",
}: Readonly<StarRatingProps>) {
  const interactive = Boolean(onChange);

  return (
    <div
      role={interactive ? "radiogroup" : undefined}
      aria-label={label}
      className="flex items-center gap-1"
    >
      {Array.from({ length: max }, (_, index) => {
        const starValue = index + 1;
        const filled = starValue <= Math.round(value);
        const star = (
          <Star
            size={size}
            aria-hidden
            className={
              filled
                ? "fill-amber-500 text-amber-500"
                : "fill-transparent text-outline-variant dark:text-zinc-700"
            }
          />
        );

        if (!interactive) {
          return <span key={starValue}>{star}</span>;
        }

        return (
          <button
            key={starValue}
            type="button"
            role="radio"
            aria-checked={starValue === Math.round(value)}
            aria-label={`${starValue} star${starValue === 1 ? "" : "s"}`}
            onClick={() => onChange?.(starValue)}
            className="rounded transition-transform duration-[var(--duration-fast)] ease-[var(--ease-standard)] hover:scale-110 active:scale-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
          >
            {star}
          </button>
        );
      })}
    </div>
  );
}
