import { ArrowRight, Newspaper, Star } from "lucide-react";
import type { Feed, Item } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { cn, extractSummary } from "@/lib/utils";
import { ItemTimestamps } from "./item-timestamps";
import {
  DomainPill,
  DomainTile,
  PriorityPill,
  SignalBars,
} from "./signal-bars";

interface TopPicksCardProps {
  items: Item[];
  isLoading: boolean;
  isError: boolean;
  selectedArticleId: number | null;
  onSelectArticle: (id: number) => void;
  isItemStarred: (id: number) => boolean;
  onToggleStar: (item: Item) => Promise<void>;
  getFeedById: (feedId: number) => Feed | undefined;
  onViewAll: () => void;
}

export function TopPicksCard({
  items,
  isLoading,
  isError,
  selectedArticleId,
  onSelectArticle,
  isItemStarred,
  onToggleStar,
  getFeedById,
  onViewAll,
}: TopPicksCardProps) {
  const { t } = useI18n();

  return (
    <section className="np-glass flex min-w-0 flex-col p-5">
      <header className="mb-3.5 flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2.5">
          <span className="np-icon-chip">
            <Newspaper className="h-4 w-4" />
          </span>
          <div className="min-w-0">
            <h2 className="truncate text-[16px] font-semibold tracking-tight text-[var(--glass-ink)]">
              {t("overview.topPicks.title")}
            </h2>
            <p className="truncate text-xs text-[var(--glass-faint)]">
              {t("overview.topPicks.subtitle")}
            </p>
          </div>
        </div>
        <button
          type="button"
          onClick={onViewAll}
          className="inline-flex shrink-0 items-center gap-1 rounded-xl px-2.5 py-1.5 text-xs font-semibold text-[var(--glass-acc)] transition-colors hover:bg-[var(--glass-soft)]"
        >
          {t("overview.topPicks.viewAll")}
          <ArrowRight className="h-3.5 w-3.5" />
        </button>
      </header>

      {isLoading ? (
        <div className="space-y-3">
          {[1, 2, 3, 4].map((idx) => (
            <div
              key={idx}
              className="h-[88px] animate-pulse rounded-2xl bg-[var(--glass-soft)]"
            />
          ))}
        </div>
      ) : isError ? (
        <div className="flex flex-1 flex-col items-center justify-center rounded-2xl border border-dashed border-[var(--glass-inner-line)] px-6 py-12 text-center">
          <p className="text-sm font-medium text-[var(--glass-ink)]">
            {t("overview.topPicks.loadFailed")}
          </p>
          <p className="mt-1 text-xs text-[var(--glass-faint)]">
            {t("overview.topPicks.loadFailedHint")}
          </p>
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center rounded-2xl border border-dashed border-[var(--glass-inner-line)] px-6 py-12 text-center">
          <p className="text-sm font-medium text-[var(--glass-ink)]">
            {t("overview.topPicks.emptyTitle")}
          </p>
          <p className="mt-1 text-xs text-[var(--glass-faint)]">
            {t("overview.topPicks.emptyDescription")}
          </p>
        </div>
      ) : (
        <div className="divide-y divide-[var(--glass-inner-line)]">
          {items.map((item) => {
            const feed = getFeedById(item.feed_id);
            const starred = isItemStarred(item.id);
            const isSelected = selectedArticleId === item.id;
            const summaryText =
              item.ai_summary.trim() ||
              (item.source_id
                ? item.content.slice(0, 160)
                : extractSummary(item.content, 160));
            const sourceLabel =
              item.author ||
              item.source_id ||
              feed?.name ||
              t("common.unknown");

            return (
              <article
                key={item.id}
                role="button"
                tabIndex={0}
                onClick={() => onSelectArticle(item.id)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onSelectArticle(item.id);
                  }
                }}
                className={cn(
                  "group -mx-2.5 flex cursor-pointer items-start gap-3.5 rounded-2xl px-2.5 py-3 text-left transition-colors outline-none hover:bg-[var(--glass-soft)] focus-visible:ring-2 focus-visible:ring-[var(--glass-acc)]",
                  isSelected && "bg-[var(--glass-soft)]",
                )}
              >
                <DomainTile
                  domain={item.domain}
                  size="md"
                  className="mt-0.5 hidden sm:grid"
                />

                <div className="min-w-0 flex-1">
                  <div className="flex items-start gap-2">
                    <h3
                      className={cn(
                        "line-clamp-1 flex-1 text-[14.5px] leading-snug tracking-tight transition-colors group-hover:text-[var(--glass-acc)]",
                        item.unread
                          ? "font-semibold text-[var(--glass-ink)]"
                          : "font-medium text-[var(--glass-sub)]",
                      )}
                    >
                      {item.unread && (
                        <span
                          className="np-unread-dot mr-2 align-middle"
                          aria-label={t("article.filter.unread")}
                        />
                      )}
                      {item.title || extractSummary(item.content, 90)}
                    </h3>
                  </div>

                  {summaryText && (
                    <p className="mt-1 line-clamp-2 text-[12.5px] leading-relaxed text-[var(--glass-sub)]">
                      {summaryText}
                    </p>
                  )}

                  <div className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-[11.5px] text-[var(--glass-faint)]">
                    <DomainPill domain={item.domain} />
                    <span className="max-w-[140px] truncate font-medium text-[var(--glass-sub)]">
                      {sourceLabel}
                    </span>
                    {item.ai_tags.slice(0, 2).map((tag) => (
                      <span
                        key={tag}
                        className="hidden rounded-md bg-[var(--glass-soft)] px-1.5 py-0.5 text-[10.5px] text-[var(--glass-sub)] md:inline"
                      >
                        #{tag}
                      </span>
                    ))}
                    <ItemTimestamps
                      item={item}
                      className="ml-auto shrink-0 sm:ml-0"
                    />
                  </div>
                </div>

                <div className="flex shrink-0 flex-col items-end gap-1.5 pt-0.5">
                  <div className="flex items-center gap-1.5">
                    <PriorityPill
                      priority={item.priority}
                      score={item.ai_score}
                    />
                    <button
                      type="button"
                      aria-label={
                        starred
                          ? t("article.action.unstar")
                          : t("article.action.star")
                      }
                      title={
                        starred
                          ? t("article.action.unstar")
                          : t("article.action.star")
                      }
                      onClick={(e) => {
                        e.stopPropagation();
                        void onToggleStar(item);
                      }}
                      className={cn(
                        "grid h-7 w-7 place-items-center rounded-lg transition-colors hover:bg-[var(--glass-soft-hover)]",
                        starred
                          ? "text-amber-500"
                          : "text-[var(--glass-faint)] hover:text-[var(--glass-ink)]",
                      )}
                    >
                      <Star
                        className={cn("h-3.5 w-3.5", starred && "fill-current")}
                      />
                    </button>
                  </div>
                  <SignalBars
                    score={item.ai_score}
                    priority={item.priority}
                    className="hidden sm:inline-flex"
                  />
                </div>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}
