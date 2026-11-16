import type { Item } from "@/lib/api";

export interface TriageSearch {
  article?: number;
  triage_status?: Item["triage_status"];
  domain?: string;
  priority?: Item["priority"];
  min_score?: number;
}

// URL filters are shareable without adding P1-04's filter-bar UI here.
export function parseTriageSearch(search: Record<string, unknown>): TriageSearch {
  const statuses: string[] = ["pending", "kept", "filtered", "failed"];
  const priorities: string[] = ["critical", "high", "normal", "low"];
  const scoreInput = search.min_score;
  const score = (typeof scoreInput === "number" || (typeof scoreInput === "string" && scoreInput.trim() !== "")) ? Number(scoreInput) : NaN;
  const article = Number(search.article);
  return {
    article: Number.isSafeInteger(article) && article > 0 ? article : undefined,
    triage_status: typeof search.triage_status === "string" && statuses.includes(search.triage_status)
      ? search.triage_status as Item["triage_status"] : undefined,
    domain: typeof search.domain === "string" ? search.domain : undefined,
    priority: typeof search.priority === "string" && priorities.includes(search.priority)
      ? search.priority as Item["priority"] : undefined,
    min_score: search.min_score !== undefined && Number.isInteger(score) && score >= 0 && score <= 100 ? score : undefined,
  };
}
