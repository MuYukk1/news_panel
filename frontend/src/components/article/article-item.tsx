import {
  Circle,
  CircleCheck,
  ExternalLink,
  ShieldAlert,
  Star,
} from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";
import { FeedFavicon } from "@/components/feed/feed-favicon";
import { ItemTimestamps } from "@/components/panel/item-timestamps";
import {
  DomainPill,
  DomainTile,
  PriorityPill,
  SignalBars,
  TriageStatusPill,
  UnscoredPill,
} from "@/components/panel/signal-bars";
import type { Item } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { toSafeExternalUrl } from "@/lib/safe-url";
import { cn, extractSummary } from "@/lib/utils";

interface ArticleItemProps {
  article: Item;
  selectedArticleId: number | null;
  onSelectArticle: (articleId: number | null) => void;
  onToggleRead: (article: Item) => Promise<void>;
  onToggleStar: (article: Item) => Promise<void>;
  canToggleRead: boolean;
  isStarred: boolean;
  showFetchedTime?: boolean;
  feedName: string;
  feedFaviconUrl: string | null;
}

export function ArticleItem({
  article,
  selectedArticleId,
  onSelectArticle,
  onToggleRead,
  onToggleStar,
  canToggleRead,
  isStarred,
  showFetchedTime = true,
  feedName,
  feedFaviconUrl,
}: ArticleItemProps) {
  const { t } = useI18n();

  const isSelected = selectedArticleId === article.id;
  const safeArticleLink = toSafeExternalUrl(article.link);
  const isFiltered = article.triage_status === "filtered";
  const isUnscored =
    article.ai_score === null && article.triage_status !== "filtered";
  const summaryText =
    article.ai_summary.trim() ||
    (article.source_id
      ? article.content.slice(0, 180)
      : extractSummary(article.content, 180));

  const handleToggleRead = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!canToggleRead) return;

    try {
      await onToggleRead(article);
    } catch (error) {
      console.error("Failed to toggle read status:", error);
    }
  };

  const handleToggleStar = async (e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await onToggleStar(article);
    } catch (error) {
      console.error("Failed to toggle star:", error);
    }
  };

  return (
    <article
      role="button"
      tabIndex={0}
      onClick={() => onSelectArticle(article.id)}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onSelectArticle(article.id);
        }
      }}
      className={cn(
        "group relative flex w-full cursor-pointer items-start gap-3.5 border-b border-[var(--glass-inner-line)] px-4 py-3.5 text-left transition-colors outline-none last:border-b-0 hover:bg-[var(--glass-soft)] focus-visible:ring-2 focus-visible:ring-[var(--glass-acc)] sm:px-5",
        isSelected && "bg-[var(--glass-soft)]",
      )}
    >
      {/* Domain Gradient Tile */}
      <DomainTile
        domain={article.domain}
        size="md"
        className="mt-0.5 hidden sm:grid"
      />

      {/* Main Content */}
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="flex items-start justify-between gap-3">
          <h3
            className={cn(
              "line-clamp-2 text-[15px] leading-snug tracking-tight transition-colors group-hover:text-[var(--glass-acc)]",
              article.unread
                ? "font-semibold text-[var(--glass-ink)]"
                : "font-medium text-[var(--glass-sub)]",
            )}
          >
            {article.unread && (
              <span
                className="np-unread-dot mr-2 align-middle"
                aria-label={t("article.filter.unread")}
              />
            )}
            {article.title || extractSummary(article.content, 100)}
          </h3>

          {/* Right Score & Priority Column */}
          <div className="hidden shrink-0 flex-col items-end gap-1.5 pt-0.5 sm:flex">
            {isUnscored ? (
              <UnscoredPill />
            ) : (
              <>
                <PriorityPill
                  priority={article.priority}
                  score={article.ai_score}
                />
                <SignalBars
                  score={article.ai_score}
                  priority={article.priority}
                />
              </>
            )}
          </div>
        </div>

        {/* AI Summary / Content Excerpt */}
        {summaryText && (
          <p className="line-clamp-2 max-w-[74ch] text-[13px] leading-relaxed text-[var(--glass-sub)]">
            {summaryText}
          </p>
        )}

        {/* Filtered Audit Banner (helps inspect L1/L2 false positives) */}
        {isFiltered && (
          <div className="mt-0.5 inline-flex flex-wrap items-center gap-1.5 rounded-xl border border-[var(--prio-critical)]/25 bg-[var(--prio-critical)]/8 px-2.5 py-1 text-[11.5px] text-[var(--prio-critical)]">
            <ShieldAlert className="h-3.5 w-3.5 shrink-0" />
            <span className="font-semibold">
              {t("panel.item.filteredBanner")}
            </span>
            {article.ai_tags.length > 0 && (
              <span className="text-[var(--glass-sub)]">
                · {article.ai_tags.join(", ")}
              </span>
            )}
          </div>
        )}

        {/* Meta Footer */}
        <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-[11.5px] text-[var(--glass-faint)]">
          <DomainPill domain={article.domain} />
          {(article.triage_status === "filtered" ||
            article.triage_status === "failed") && (
            <TriageStatusPill status={article.triage_status} />
          )}
          <span className="sm:hidden">
            {isUnscored ? (
              <UnscoredPill />
            ) : (
              <PriorityPill
                priority={article.priority}
                score={article.ai_score}
              />
            )}
          </span>

          <span className="inline-flex max-w-[180px] items-center gap-1.5 truncate font-medium text-[var(--glass-sub)]">
            {!article.source_id && (
              <FeedFavicon
                src={feedFaviconUrl}
                className="h-3.5 w-3.5 rounded-sm"
              />
            )}
            <span className="truncate">
              {article.author
                ? `${article.author}${article.source_id ? ` · ${article.source_id}` : ""}`
                : article.source_id || feedName}
            </span>
          </span>

          {article.ai_tags.slice(0, 3).map((tag) => (
            <span
              key={tag}
              className="hidden rounded-md bg-[var(--glass-soft)] px-1.5 py-0.5 text-[10.5px] text-[var(--glass-sub)] md:inline"
            >
              #{tag}
            </span>
          ))}

          <ItemTimestamps
            item={article}
            showFetched={showFetchedTime}
            className="shrink-0"
          />
        </div>
      </div>

      {/* Hover / Focus Quick Actions */}
      <div className="absolute right-3 bottom-3 hidden items-center gap-1 rounded-xl border border-[var(--glass-border)] bg-[var(--glass-panel-strong)] p-1 shadow-sm backdrop-blur-md group-focus-within:flex group-hover:flex">
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={handleToggleRead}
          disabled={!canToggleRead}
          className={cn(
            "h-7 w-7 rounded-lg",
            article.unread
              ? "hover:bg-[var(--glass-soft)]"
              : "bg-[var(--glass-soft)] text-[var(--glass-acc)]",
          )}
          aria-label={
            article.unread
              ? t("article.action.markRead")
              : t("article.action.markUnread")
          }
          title={
            article.unread
              ? t("article.action.markRead")
              : t("article.action.markUnread")
          }
        >
          {article.unread ? (
            <Circle className="h-3.5 w-3.5 text-[var(--glass-sub)]" />
          ) : (
            <CircleCheck className="h-3.5 w-3.5 text-[var(--glass-acc)]" />
          )}
        </Button>
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={handleToggleStar}
          className={cn(
            "h-7 w-7 rounded-lg",
            isStarred
              ? "bg-amber-500/15 text-amber-500"
              : "hover:bg-[var(--glass-soft)]",
          )}
          aria-label={
            isStarred ? t("article.action.unstar") : t("article.action.star")
          }
          title={
            isStarred ? t("article.action.unstar") : t("article.action.star")
          }
        >
          <Star
            className={cn(
              "h-3.5 w-3.5",
              isStarred
                ? "fill-amber-500 text-amber-500"
                : "text-[var(--glass-sub)]",
            )}
          />
        </Button>
        {safeArticleLink ? (
          <a
            href={safeArticleLink}
            target="_blank"
            rel="noopener noreferrer"
            onClick={(e) => e.stopPropagation()}
            className={cn(
              buttonVariants({ variant: "ghost", size: "icon-sm" }),
              "h-7 w-7 rounded-lg hover:bg-[var(--glass-soft)]",
            )}
            aria-label={t("article.action.openInBrowser")}
            title={t("article.action.openInBrowser")}
          >
            <ExternalLink className="h-3.5 w-3.5 text-[var(--glass-sub)]" />
          </a>
        ) : (
          <Button
            variant="ghost"
            size="icon-sm"
            disabled
            className="h-7 w-7 rounded-lg"
            aria-label={t("article.action.openInBrowser")}
            title={t("article.action.openInBrowser")}
          >
            <ExternalLink className="h-3.5 w-3.5 text-[var(--glass-faint)]" />
          </Button>
        )}
      </div>
    </article>
  );
}
