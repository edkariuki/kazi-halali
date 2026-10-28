"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { Header } from "@/components/Header";
import { Result } from "@/components/Result";
import { useLang } from "@/components/lang";
import { getCheck } from "@/lib/api";
import type { Check } from "@/lib/types";

// Shareable result page: /c/<id>
// The id is only known at request time, so useParams must sit inside Suspense.
export default function SharedCheckPage() {
  return (
    <main className="mx-auto max-w-[640px] px-4 pb-20">
      <Header />
      <Suspense fallback={<Loading />}>
        <SharedCheck />
      </Suspense>
    </main>
  );
}

function Loading() {
  const { t } = useLang();
  return <p className="mt-10 text-ink-muted">{t.loading}</p>;
}

function SharedCheck() {
  const { id } = useParams<{ id: string }>();
  const { t } = useLang();
  const [check, setCheck] = useState<Check | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    getCheck(id).then(setCheck).catch(() => setFailed(true));
  }, [id]);

  if (failed) {
    return (
      <div className="mt-10">
        <p className="font-bold">{t.notFound}</p>
        <Link href="/" className="mt-4 inline-block underline font-bold">
          {t.submit}
        </Link>
      </div>
    );
  }
  if (!check) return <Loading />;
  return (
    <>
      <Result check={check} />
      <Link href="/" className="mt-6 inline-block underline font-bold">
        {t.again}
      </Link>
    </>
  );
}
