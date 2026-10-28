"use client";

import Link from "next/link";
import { LangToggle, useLang } from "./lang";

export function Header() {
  const { t } = useLang();
  return (
    <header className="flex items-center justify-between gap-4 py-5">
      <Link href="/" className="font-bold text-lg tracking-tight no-underline">
        {t.brand}
      </Link>
      <LangToggle />
    </header>
  );
}
