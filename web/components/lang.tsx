"use client";

import { createContext, useContext, useEffect, useSyncExternalStore, type ReactNode } from "react";

export type Lang = "en" | "sw";

const strings = {
  en: {
    brand: "Kazi Halali",
    headline: "Check a job abroad before you pay.",
    intro:
      "Paste the job ad or upload a screenshot. We check the agency against the National Employment Authority's licensed list and look for the signs found in reported scams.",
    adLabel: "The job ad, offer letter or message",
    adPlaceholder: "Paste the ad here — for example a WhatsApp forward or a Facebook post",
    attach: "Add a screenshot",
    replace: "Change screenshot",
    remove: "Remove",
    submit: "Check this job",
    checking: "Checking…",
    needInput: "Paste the job ad or add a screenshot first.",
    privacy: "Don't include your ID or passport number. What you share is stored so you can share the result.",
    stampDanger: "Do not pay",
    stampCaution: "Be careful",
    stampOk: "No red flags found",
    yourAd: "What you shared",
    fromImage: "Read from your screenshot",
    whatToDo: "What to do now",
    why: "Why",
    passed: "Checks that came back clean",
    reminders: "Before you travel",
    evidence: "Found in the ad",
    sources: "Source",
    how: "How this check was done",
    steps: {
      vision: "Read your screenshot",
      extract: "Picked out the agency, job, fees and visa",
      search: "Searched for public fraud reports",
      official: "Searched government websites for warnings",
      salary: "Compared the salary with typical pay",
      explain: "Wrote this explanation",
    } as Record<string, string>,
    registry: "Checked the agency against our copy of the NEA register",
    registerPublished: "register published",
    registerCopied: "our copy taken",
    notGenerated: "Explanation written from fixed templates.",
    share: "Share on WhatsApp",
    copy: "Copy link",
    copied: "Link copied",
    again: "Check another job",
    shareText: "I checked this overseas job offer on Kazi Halali:",
    disclaimer:
      "Kazi Halali points out warning signs. It cannot prove that a job is genuine. Always confirm with the National Employment Authority before you pay or travel.",
    notFound: "We couldn't find that check. The link may be wrong.",
    loading: "Loading the check…",
  },
  sw: {
    brand: "Kazi Halali",
    headline: "Kagua kazi ya nje kabla ya kulipa.",
    intro:
      "Bandika tangazo la kazi au pakia picha ya skrini. Tunakagua wakala kwenye orodha ya mawakala wenye leseni ya Mamlaka ya Kitaifa ya Ajira (NEA) na kutafuta dalili zilizoonekana kwenye utapeli ulioripotiwa.",
    adLabel: "Tangazo la kazi, barua ya ofa au ujumbe",
    adPlaceholder: "Bandika tangazo hapa — kwa mfano ujumbe wa WhatsApp au chapisho la Facebook",
    attach: "Ongeza picha ya skrini",
    replace: "Badilisha picha",
    remove: "Ondoa",
    submit: "Kagua kazi hii",
    checking: "Inakagua…",
    needInput: "Bandika tangazo au ongeza picha ya skrini kwanza.",
    privacy: "Usiweke nambari ya kitambulisho au pasipoti. Unachotuma kinahifadhiwa ili uweze kushiriki matokeo.",
    stampDanger: "Usilipe",
    stampCaution: "Tahadhari",
    stampOk: "Hakuna dalili za hatari",
    yourAd: "Ulichotuma",
    fromImage: "Kilichosomwa kwenye picha yako",
    whatToDo: "Fanya hivi sasa",
    why: "Kwa nini",
    passed: "Ukaguzi uliopita bila tatizo",
    reminders: "Kabla ya kusafiri",
    evidence: "Kilichoonekana kwenye tangazo",
    sources: "Chanzo",
    how: "Jinsi ukaguzi huu ulivyofanywa",
    steps: {
      vision: "Tumesoma picha yako",
      extract: "Tumetoa wakala, kazi, ada na viza",
      search: "Tumetafuta ripoti za utapeli mtandaoni",
      official: "Tumetafuta maonyo kwenye tovuti za serikali",
      salary: "Tumelinganisha mshahara na malipo ya kawaida",
      explain: "Tumeandika maelezo haya",
    } as Record<string, string>,
    registry: "Tumelinganisha wakala na nakala yetu ya rejista ya NEA",
    registerPublished: "rejista ilichapishwa",
    registerCopied: "nakala yetu ilichukuliwa",
    notGenerated: "Maelezo yameandikwa kwa violezo maalum.",
    share: "Shiriki kwenye WhatsApp",
    copy: "Nakili kiungo",
    copied: "Kiungo kimenakiliwa",
    again: "Kagua kazi nyingine",
    shareText: "Nimekagua ofa hii ya kazi ya nje kwenye Kazi Halali:",
    disclaimer:
      "Kazi Halali huonyesha dalili za hatari. Haiwezi kuthibitisha kuwa kazi ni ya kweli. Thibitisha kila mara na NEA kabla ya kulipa au kusafiri.",
    notFound: "Hatukupata ukaguzi huo. Huenda kiungo si sahihi.",
    loading: "Inapakia ukaguzi…",
  },
};

export type Strings = (typeof strings)["en"];

const Ctx = createContext<{ lang: Lang; t: Strings; setLang: (l: Lang) => void }>({
  lang: "en",
  t: strings.en,
  setLang: () => { },
});

// The chosen language lives in localStorage (a per-viewer convenience).
// useSyncExternalStore reads it without a setState-in-effect round trip and
// renders English on the server, then the saved choice on the client.
const LANG_KEY = "kh-lang";
const listeners = new Set<() => void>();

function readLang(): Lang {
  try {
    const saved = localStorage.getItem(LANG_KEY);
    if (saved === "en" || saved === "sw") return saved;
  } catch {
    /* storage unavailable: fall back to English */
  }
  return "en";
}

function subscribe(cb: () => void) {
  listeners.add(cb);
  return () => listeners.delete(cb);
}

function writeLang(l: Lang) {
  try {
    localStorage.setItem(LANG_KEY, l);
  } catch {
    /* ignore */
  }
  memoryLang = l;
  listeners.forEach((cb) => cb());
}

// Used when storage throws, so the toggle still works for this visit.
let memoryLang: Lang | null = null;

export function LangProvider({ children }: { children: ReactNode }) {
  const lang = useSyncExternalStore(
    subscribe,
    () => memoryLang ?? readLang(),
    () => "en" as Lang,
  );

  useEffect(() => {
    document.documentElement.lang = lang;
  }, [lang]);

  const setLang = writeLang;

  return <Ctx.Provider value={{ lang, t: strings[lang], setLang }}>{children}</Ctx.Provider>;
}

export const useLang = () => useContext(Ctx);

export function LangToggle() {
  const { lang, setLang } = useLang();
  const btn = (l: Lang, label: string) => (
    <button
      type="button"
      onClick={() => setLang(l)}
      aria-pressed={lang === l}
      className={`px-3 py-1.5 text-sm rounded-[3px] ${lang === l ? "bg-ink text-slip font-bold" : "text-ink-muted hover:text-ink"
        }`}
    >
      {label}
    </button>
  );
  return (
    <div role="group" aria-label="Language / Lugha" className="flex gap-1 border border-rule rounded-[5px] p-0.5 bg-slip">
      {btn("en", "English")}
      {btn("sw", "Kiswahili")}
    </div>
  );
}