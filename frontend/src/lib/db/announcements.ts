import { db } from "./index";
import { pushNotification } from "./notifications";
import { ROUTES } from "@/lib/types/routes";
import type { Announcement } from "@/lib/types";

// Local-only table, no server sync yet (no backend endpoint exists).

export async function listAnnouncements(): Promise<Announcement[]> {
  return db.announcements.orderBy("created_at").reverse().toArray();
}

export async function getAnnouncement(id: string): Promise<Announcement | undefined> {
  return db.announcements.get(id);
}

export interface CreateAnnouncementInput {
  title: string;
  body: string;
  created_by?: string;
}

export async function createAnnouncement(
  input: CreateAnnouncementInput,
): Promise<Announcement> {
  const title = input.title.trim();
  if (!title) throw new Error("Title is required");
  const body = input.body.trim();
  if (!body) throw new Error("Body is required");

  const announcement: Announcement = {
    id: crypto.randomUUID(),
    title,
    body,
    status: "draft",
    created_at: Date.now(),
    created_by: input.created_by,
  };
  await db.announcements.add(announcement);
  return announcement;
}

export async function updateAnnouncement(
  id: string,
  changes: Partial<Pick<Announcement, "title" | "body">>,
): Promise<void> {
  const title = changes.title?.trim();
  if (title === "") throw new Error("Title is required");
  const body = changes.body?.trim();
  if (body === "") throw new Error("Body is required");

  await db.announcements.update(id, {
    ...(title ? { title } : {}),
    ...(body ? { body } : {}),
  });
}

/**
 * Publishing also raises the existing notification bell (`pushNotification`)
 * rather than building a second delivery mechanism — announcements get their
 * own draft/publish/archive lifecycle, but the alert itself rides the app's
 * one notification pipeline.
 */
export async function publishAnnouncement(id: string): Promise<void> {
  const announcement = await db.announcements.get(id);
  if (!announcement) throw new Error("That announcement no longer exists");

  const published_at = Date.now();
  await db.announcements.update(id, { status: "published", published_at });
  await pushNotification({
    kind: "announcement",
    title: announcement.title,
    href: ROUTES.hrm.announcements,
  });
}

export async function archiveAnnouncement(id: string): Promise<void> {
  await db.announcements.update(id, { status: "archived" });
}

export async function deleteAnnouncement(id: string): Promise<void> {
  await db.announcements.delete(id);
}
