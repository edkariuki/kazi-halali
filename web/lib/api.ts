import type { Check } from "./types";

export const API_URL = (process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080").replace(/\/$/, "");

async function parse(res: Response): Promise<Check> {
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error((body as { error?: string }).error ?? `Request failed (${res.status})`);
  }
  return body as Check;
}

export async function createCheck(text: string, image: File | null): Promise<Check> {
  if (image) {
    const form = new FormData();
    form.append("text", text);
    form.append("image", image);
    return parse(await fetch(`${API_URL}/api/checks`, { method: "POST", body: form }));
  }
  return parse(
    await fetch(`${API_URL}/api/checks`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text }),
    }),
  );
}

export async function getCheck(id: string): Promise<Check> {
  return parse(await fetch(`${API_URL}/api/checks/${encodeURIComponent(id)}`));
}
