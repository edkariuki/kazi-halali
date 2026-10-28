// Mirrors api/internal/domain. Arrays may arrive as null from Go.

export type Severity = "critical" | "high" | "medium" | "info" | "pass";
export type Verdict = "danger" | "caution" | "likely_ok";

export interface Source {
  title: string;
  url: string;
}

export interface Finding {
  rule_id: string;
  severity: Severity;
  title_en: string;
  title_sw: string;
  detail_en: string;
  detail_sw: string;
  evidence?: string;
  sources: Source[] | null;
}

export interface Agency {
  id: number;
  name: string;
  nea_reg_no: string;
  status: "valid" | "under_verification" | "expired" | "unknown";
  source_url: string;
}

export interface ModelCall {
  step: "vision" | "extract" | "search" | "official" | "salary" | "explain";
  model: string;
  latency_ms: number;
  input_tokens: number;
  output_tokens: number;
  error?: string;
}

export interface Check {
  id: string;
  created_at: string;
  input_text: string;
  had_image: boolean;
  transcript?: string;
  facts: {
    agency_name: string;
    agency_licence_no: string;
    destination_country: string;
    job_title: string;
    job_category: string;
  };
  registry: {
    kind: "none" | "exact" | "licence" | "lookalike";
    agency?: Agency;
    similarity?: number;
    licence_owner?: Agency;
    register?: {
      source_url?: string;
      agencies: number;
      published_at?: string;
      snapshot_at?: string;
    };
  };
  official_searched?: boolean;
  salary_benchmark?: {
    job_title: string;
    country: string;
    found: boolean;
    monthly_low: number;
    monthly_high: number;
    currency: string;
    ad_monthly: number;
    ad_currency: string;
    ratio: number;
    sources: Source[] | null;
    note?: string;
  };
  findings: Finding[] | null;
  verdict: Verdict;
  explanation: {
    summary_en: string;
    summary_sw: string;
    next_steps_en: string[] | null;
    next_steps_sw: string[] | null;
    generated: boolean;
  };
  trace: ModelCall[] | null;
}