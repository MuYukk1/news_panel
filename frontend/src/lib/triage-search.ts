import type { Item } from "@/lib/api";

export type TriageStatusFilter = Item["triage_status"] | "all";

export interface TriageSearch {
  article?: number;
  triage_status?: TriageStatusFilter;
  domain?: string;
  priority?: Item["priority"];
  min_score?: number;
}

const VALID_TRIAGE_STATUSES: readonly string[] = [
  "pending",
  "kept",
  "filtered",
  "failed",
  "all",
];

const VALID_PRIORITIES: readonly string[] = [
  "critical",
  "high",
  "normal",
  "low",
];

export function parseTriageSearch(search: Record<string, unknown>): TriageSearch {
  const scoreInput = search.min_score;
  const score =
    typeof scoreInput === "number" ||
    (typeof scoreInput === "string" && scoreInput.trim() !== "")
      ? Number(scoreInput)
      : NaN;
  const article = Number(search.article);
  const rawDomain =
    typeof search.domain === "string" ? search.domain.trim() : "";

  return {
    article: Number.isSafeInteger(article) && article > 0 ? article : undefined,
    triage_status:
      typeof search.triage_status === "string" &&
      VALID_TRIAGE_STATUSES.includes(search.triage_status)
        ? (search.triage_status as TriageStatusFilter)
        : undefined,
    domain: rawDomain ? rawDomain : undefined,
    priority:
      typeof search.priority === "string" &&
      VALID_PRIORITIES.includes(search.priority)
        ? (search.priority as Item["priority"])
        : undefined,
    min_score:
      search.min_score !== undefined &&
      Number.isInteger(score) &&
      score >= 0 &&
      score <= 100
        ? score
        : undefined,
  };
}
