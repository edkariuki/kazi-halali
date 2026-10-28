import type { Metadata, Viewport } from "next";
// Self-hosted fonts (no request to Google): faster on slow connections.
// Atkinson Hyperlegible was designed for readers with low vision — clear
// letterforms on cheap screens and for readers of every literacy level.
import "@fontsource/atkinson-hyperlegible/400.css";
import "@fontsource/atkinson-hyperlegible/700.css";
// Anton is used only for the verdict stamp.
import "@fontsource/anton/400.css";
import { LangProvider } from "@/components/lang";
import "./globals.css";

export const metadata: Metadata = {
  title: "Kazi Halali — check a job abroad before you pay",
  description:
    "Paste an overseas job ad or upload a screenshot. Kazi Halali checks the agency against the NEA licensed list and looks for known scam signs.",
};

export const viewport: Viewport = {
  themeColor: "#f3f5f7",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className="antialiased">
      <body className="min-h-dvh">
        <LangProvider>{children}</LangProvider>
      </body>
    </html>
  );
}
