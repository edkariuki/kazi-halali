"use client";

import { useState } from "react";
import { CheckForm } from "@/components/CheckForm";
import { Header } from "@/components/Header";
import { Result } from "@/components/Result";
import { useLang } from "@/components/lang";
import type { Check } from "@/lib/types";

export default function Home() {
  const { t } = useLang();
  const [check, setCheck] = useState<Check | null>(null);

  return (
    <main className="mx-auto max-w-[640px] px-4 pb-20">
      <Header />
      {!check ? (
        <>
          <h1 className="mt-6 text-[2.1rem] sm:text-[2.6rem] leading-[1.1] font-bold tracking-tight max-w-[18ch]">
            {t.headline}
          </h1>
          <p className="mt-4 text-lg text-ink-muted max-w-prose">{t.intro}</p>
          <CheckForm
            onResult={(c) => {
              setCheck(c);
              window.history.replaceState(null, "", `/c/${c.id}`);
              window.scrollTo({ top: 0 });
            }}
          />
        </>
      ) : (
        <Result
          check={check}
          onReset={() => {
            setCheck(null);
            window.history.replaceState(null, "", "/");
          }}
        />
      )}
    </main>
  );
}
