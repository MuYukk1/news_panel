import type { Item } from "@/lib/api";

// Minimal shared list/detail presentation; richer panel controls belong to P1-04.
export function TriageMetadata({ article }: { article: Item }) {
  if (article.id < 0) return <p className="text-sm text-muted-foreground">Triage unavailable for saved snapshot</p>;
  return (
    <div className="space-y-2 text-sm text-foreground">
      <dl className="flex flex-wrap gap-x-4 gap-y-1 break-words">
        <div className="flex gap-1"><dt className="text-muted-foreground">Domain:</dt><dd>{article.domain}</dd></div>
        <div className="flex gap-1"><dt className="text-muted-foreground">Score:</dt><dd>{article.ai_score ?? "Unscored"}</dd></div>
        <div className="flex gap-1"><dt className="text-muted-foreground">Priority:</dt><dd>{article.priority}</dd></div>
        <div className="flex gap-1"><dt className="text-muted-foreground">Triage:</dt><dd>{article.triage_status}</dd></div>
        <div className="flex min-w-0 gap-1"><dt className="text-muted-foreground">Tags:</dt><dd className="break-words">{article.ai_tags?.join(", ") || "None"}</dd></div>
      </dl>
      {article.ai_summary && <p className="break-words"><span className="text-muted-foreground">Summary: </span>{article.ai_summary}</p>}
    </div>
  );
}
