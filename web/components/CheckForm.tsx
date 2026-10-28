"use client";

import { useRef, useState } from "react";
import { createCheck } from "@/lib/api";
import type { Check } from "@/lib/types";
import { useLang } from "./lang";

export function CheckForm({ onResult }: { onResult: (c: Check) => void }) {
  const { t } = useLang();
  const [text, setText] = useState("");
  const [image, setImage] = useState<File | null>(null);
  const [preview, setPreview] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  function pick(file: File | null) {
    if (preview) URL.revokeObjectURL(preview);
    setImage(file);
    setPreview(file ? URL.createObjectURL(file) : null);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!text.trim() && !image) {
      setError(t.needInput);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      onResult(await createCheck(text, image));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="mt-8 space-y-4" aria-busy={busy}>
      <label htmlFor="ad" className="block font-bold">
        {t.adLabel}
      </label>
      <textarea
        id="ad"
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={t.adPlaceholder}
        rows={7}
        maxLength={8000}
        className="slip w-full p-4 text-base leading-relaxed placeholder:text-ink-muted/80 resize-y"
      />

      <div className="flex flex-wrap items-center gap-3">
        <input
          ref={fileRef}
          type="file"
          accept="image/png,image/jpeg,image/webp"
          className="sr-only"
          id="shot"
          onChange={(e) => pick(e.target.files?.[0] ?? null)}
        />
        <label
          htmlFor="shot"
          className="cursor-pointer border border-ink/30 rounded-[5px] px-4 py-2.5 bg-slip hover:border-ink"
          tabIndex={0}
          onKeyDown={(e) => (e.key === "Enter" || e.key === " ") && fileRef.current?.click()}
        >
          {image ? t.replace : t.attach}
        </label>
        {preview && (
          <span className="flex items-center gap-3">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={preview} alt="" className="h-12 w-12 object-cover border border-rule rounded-[3px]" />
            <button
              type="button"
              className="underline text-ink-muted hover:text-ink"
              onClick={() => {
                pick(null);
                if (fileRef.current) fileRef.current.value = "";
              }}
            >
              {t.remove}
            </button>
          </span>
        )}
      </div>

      <p className="text-sm text-ink-muted max-w-prose">{t.privacy}</p>

      {error && (
        <p role="alert" className="text-danger font-bold">
          {error}
        </p>
      )}

      <button
        type="submit"
        disabled={busy}
        className="w-full sm:w-auto bg-ink text-slip font-bold text-lg px-7 py-3.5 rounded-[5px] disabled:opacity-60"
      >
        {busy ? t.checking : t.submit}
      </button>
    </form>
  );
}
