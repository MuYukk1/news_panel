import type { Item } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import {
  DomainPill,
  PriorityPill,
  SignalBars,
  TriageStatusPill,
} from "@/components/panel/signal-bars";

export function TriageMetadata({ article }: { article: Item }) {
  const { t } = useI18n();

  if (article.id < 0) {
    return (
      <p className="rounded-xl bg-[var(--glass-soft)] px-3 py-2 text-xs text-[var(--glass-sub)]">
        {t("panel.drawer.snapshotNoTriage")}
      </p>
    );
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
      <DomainPill domain={article.domain} />
      <TriageStatusPill status={article.triage_status} />
      <PriorityPill priority={article.priority} score={article.ai_score} />
      <SignalBars score={article.ai_score} priority={article.priority} />
      {article.ai_tags?.map((tag) => (
        <span
          key={tag}
          className="rounded-full bg-[var(--glass-soft)] px-2 py-0.5 text-[11px] font-medium text-[var(--glass-sub)]"
        >
          #{tag}
        </span>
      ))}
    </div>
  );
}
