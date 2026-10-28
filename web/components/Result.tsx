"use client";

import { useState } from "react";
import type { Check, Finding, Severity, Verdict } from "@/lib/types";
import { useLang, type Lang } from "./lang";

const stampColor: Record<Verdict, string> = {
  danger: "text-danger",
  caution: "text-caution",
  likely_ok: "text-ok",
};

const sevWord: Record<Lang, Partial<Record<Severity, string>>> = {
  en: { critical: "Serious", high: "Warning", medium: "Watch out" },
  sw: { critical: "Hatari kubwa", high: "Onyo", medium: "Angalia" },
};

export function Result({ check, onReset }: { check: Check; onReset?: () => void }) {
  const { t, lang } = useLang();
  const findings = check.findings ?? [];
  const rank: Record<string, number> = { critical: 0, high: 1, medium: 2 };
  const red = findings
    .filter((f) => f.severity in rank)
    .sort((a, b) => rank[a.severity] - rank[b.severity]);
  const clean = findings.filter((f) => f.severity === "pass");
  const info = findings.filter((f) => f.severity === "info");
  const steps = (lang === "sw" ? check.explanation.next_steps_sw : check.explanation.next_steps_en) ?? [];
  const summary = lang === "sw" ? check.explanation.summary_sw : check.explanation.summary_en;
  const stampText = { danger: t.stampDanger, caution: t.stampCaution, likely_ok: t.stampOk }[check.verdict];

  const excerpt = (check.input_text || check.transcript || "").trim();

  return (
    <article className="mt-8" aria-live="polite">
      {/* The ad, with the verdict stamped across it */}
      <div className="relative">
        <div className="slip p-4 pr-6 pb-14 min-h-36">
          <p className="text-sm text-ink-muted mb-2">{check.input_text ? t.yourAd : t.fromImage}</p>
          <p className="whitespace-pre-line line-clamp-6 text-[0.95rem] leading-relaxed text-ink/80">{excerpt || "—"}</p>
        </div>
        <div
          className={`stamp stamp-land absolute right-3 -bottom-6 sm:right-6 text-[1.9rem] sm:text-[2.4rem] uppercase bg-slip/0 ${stampColor[check.verdict]}`}
          role="img"
          aria-label={stampText}
        >
          {stampText}
        </div>
      </div>

      <p className="mt-12 text-xl leading-snug font-bold max-w-[34ch] sm:max-w-[40ch]">{summary}</p>

      {steps.length > 0 && (
        <section className="mt-8">
          <h2 className="font-bold text-lg mb-3">{t.whatToDo}</h2>
          <ol className="list-decimal pl-6 space-y-2 marker:font-bold max-w-prose">
            {steps.map((s, i) => (
              <li key={i} className="pl-1">
                {s}
              </li>
            ))}
          </ol>
        </section>
      )}

      {red.length > 0 && (
        <section className="mt-10">
          <h2 className="font-bold text-lg mb-3">{t.why}</h2>
          <ul className="space-y-3">
            {red.map((f, i) => (
              <FindingRow key={f.rule_id + i} f={f} lang={lang} tag={sevWord[lang][f.severity]} />
            ))}
          </ul>
        </section>
      )}

      {clean.length > 0 && (
        <section className="mt-10">
          <h2 className="font-bold mb-3">{t.passed}</h2>
          <ul className="space-y-3">
            {clean.map((f, i) => (
              <FindingRow key={f.rule_id + i} f={f} lang={lang} />
            ))}
          </ul>
        </section>
      )}

      {info.length > 0 && (
        <section className="mt-10">
          <h2 className="font-bold mb-3">{t.reminders}</h2>
          <ul className="space-y-3">
            {info.map((f, i) => (
              <FindingRow key={f.rule_id + i} f={f} lang={lang} />
            ))}
          </ul>
        </section>
      )}

      <ShareBar check={check} />

      <HowItWasChecked check={check} />

      <p className="mt-8 text-sm text-ink-muted max-w-prose">{t.disclaimer}</p>

      {onReset && (
        <button type="button" onClick={onReset} className="mt-6 underline font-bold">
          {t.again}
        </button>
      )}
    </article>
  );
}

function FindingRow({ f, lang, tag }: { f: Finding; lang: Lang; tag?: string }) {
  const { t } = useLang();
  const title = lang === "sw" ? f.title_sw : f.title_en;
  const detail = lang === "sw" ? f.detail_sw : f.detail_en;
  return (
    <li className={`sev sev-${f.severity} bg-slip border-y border-r border-rule rounded-r-[3px] pl-4 pr-4 py-3`}>
      <p className="font-bold leading-snug">
        {tag && <span className="font-normal text-ink-muted">{tag}: </span>}
        {title}
      </p>
      <p className="mt-1 max-w-prose">{detail}</p>
      {f.evidence && f.severity !== "pass" && (
        <p className="mt-2 text-sm text-ink-muted">
          {t.evidence}: <q>{f.evidence}</q>
        </p>
      )}
      {(f.sources ?? []).length > 0 && (
        <p className="mt-2 text-sm">
          <span className="text-ink-muted">{t.sources}: </span>
          {(f.sources ?? []).map((s, i) => (
            <span key={s.url + i}>
              {i > 0 && ", "}
              <a href={s.url} target="_blank" rel="noopener noreferrer" className="underline decoration-rule underline-offset-2 hover:decoration-ink">
                {s.title}
              </a>
            </span>
          ))}
        </p>
      )}
    </li>
  );
}

function ShareBar({ check }: { check: Check }) {
  const { t } = useLang();
  const [copied, setCopied] = useState(false);
  const url = typeof window === "undefined" ? "" : `${window.location.origin}/c/${check.id}`;
  const wa = `https://wa.me/?text=${encodeURIComponent(`${t.shareText} ${url}`)}`;

  return (
    <div className="mt-10 flex flex-wrap gap-3">
      <a href={wa} target="_blank" rel="noopener noreferrer" className="bg-ok text-slip font-bold px-5 py-3 rounded-[5px]">
        {t.share}
      </a>
      <button
        type="button"
        className="border border-ink/30 bg-slip px-5 py-3 rounded-[5px] hover:border-ink"
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(url);
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
          } catch {
            window.prompt(t.copy, url);
          }
        }}
      >
        {copied ? t.copied : t.copy}
      </button>
    </div>
  );
}

function HowItWasChecked({ check }: { check: Check }) {
  const { t } = useLang();
  const trace = check.trace ?? [];
  return (
    <details className="mt-10 border-t border-rule pt-4 group">
      <summary className="cursor-pointer font-bold">{t.how}</summary>
      <ul className="mt-3 space-y-2 text-sm">
        {trace.map((c, i) => (
          <li key={i} className="flex flex-wrap gap-x-2">
            <span>{t.steps[c.step] ?? c.step}</span>
            <span className="text-ink-muted">
              {c.model}
              {c.latency_ms > 0 && ` (${(c.latency_ms / 1000).toFixed(1)}s)`}
            </span>
            {c.error && <span className="text-caution w-full">{c.error}</span>}
          </li>
        ))}
        <li className="flex flex-wrap gap-x-2">
          <span>{t.registry}</span>
          <RegisterDates check={check} />
        </li>
        {!check.explanation.generated && <li className="text-ink-muted">{t.notGenerated}</li>}
      </ul>
    </details>
  );
}

// Go sends zero dates as "0001-01-01T00:00:00Z"; treat those as unknown.
function validDate(s?: string): Date | null {
  if (!s) return null;
  const d = new Date(s);
  return isNaN(d.getTime()) || d.getUTCFullYear() < 2000 ? null : d;
}

function RegisterDates({ check }: { check: Check }) {
  const { t, lang } = useLang();
  const r = check.registry.register;
  const fmt = (d: Date) =>
    d.toLocaleDateString(lang === "sw" ? "sw-KE" : "en-GB", { day: "numeric", month: "short", year: "numeric", timeZone: "UTC" });
  const pub = validDate(r?.published_at);
  const snap = validDate(r?.snapshot_at);
  const parts = [pub && `${t.registerPublished} ${fmt(pub)}`, snap && `${t.registerCopied} ${fmt(snap)}`].filter(Boolean);
  if (parts.length === 0) return null;
  const text = `(${parts.join("; ")})`;
  return r?.source_url ? (
    <a href={r.source_url} target="_blank" rel="noopener noreferrer" className="text-ink-muted underline decoration-rule underline-offset-2">
      {text}
    </a>
  ) : (
    <span className="text-ink-muted">{text}</span>
  );
}