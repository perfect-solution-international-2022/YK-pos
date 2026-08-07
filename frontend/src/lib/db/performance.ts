import { db } from "./index";
import type { PerformanceReview } from "@/lib/types";

// Local-only table, no server sync yet (no backend endpoint exists). No
// uniqueness constraint on (employee_id, period) — unlike payroll, a manager
// may legitimately log more than one note in the same month.

export async function listReviews(): Promise<PerformanceReview[]> {
  return db.performanceReviews.orderBy("reviewed_at").reverse().toArray();
}

export async function getReview(id: string): Promise<PerformanceReview | undefined> {
  return db.performanceReviews.get(id);
}

export async function listReviewsForEmployee(
  employeeId: string,
): Promise<PerformanceReview[]> {
  return db.performanceReviews.where("employee_id").equals(employeeId).toArray();
}

export interface CreateReviewInput {
  employee_id: string;
  period: string;
  rating: number;
  notes?: string;
  reviewed_by?: string;
}

export async function createReview(input: CreateReviewInput): Promise<PerformanceReview> {
  if (input.rating < 1 || input.rating > 5) throw new Error("Rating must be 1 to 5");

  const review: PerformanceReview = {
    id: crypto.randomUUID(),
    employee_id: input.employee_id,
    period: input.period,
    rating: input.rating,
    notes: input.notes,
    reviewed_by: input.reviewed_by,
    reviewed_at: Date.now(),
  };
  await db.performanceReviews.add(review);
  return review;
}

export async function updateReview(
  id: string,
  changes: Partial<Pick<PerformanceReview, "period" | "rating" | "notes">>,
): Promise<void> {
  if (changes.rating !== undefined && (changes.rating < 1 || changes.rating > 5)) {
    throw new Error("Rating must be 1 to 5");
  }
  await db.performanceReviews.update(id, changes);
}

export async function deleteReview(id: string): Promise<void> {
  await db.performanceReviews.delete(id);
}

export function averageRating(reviews: PerformanceReview[]): number | null {
  if (reviews.length === 0) return null;
  return reviews.reduce((sum, review) => sum + review.rating, 0) / reviews.length;
}
