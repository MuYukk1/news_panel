import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Circle,
  CircleCheck,
  ExternalLink,
  Eye,
  Heart,
  MessageCircle,
  Repeat2,
  Sparkles,
  Star,
  X,
} from "lucide-react";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { Button, buttonVariants } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { FeedFavicon } from "@/components/feed/feed-favicon";
import { ItemTimestamps } from "@/components/panel/item-timestamps";
import {
  extractRawMeta,
  formatCompactNumber,
  isPluginSourceFeed,
  selectTopPicks,
} from "@/components/panel/panel-utils";
import { useArticleList } from "@/hooks/use-article-list";
import { useAutoMarkRead } from "@/hooks/use-auto-mark-read";
import { useArticleNavigation } from "@/hooks/use-keyboard";
import { useUrlState } from "@/hooks/use-url-state";
import type { Item } from "@/lib/api";
import { getFaviconUrl } from "@/lib/api/favicon";
import { processArticleContent } from "@/lib/content";
import { useI18n } from "@/lib/i18n";
import { toSafeExternalUrl } from "@/lib/safe-url";
import { cn } from "@/lib/utils";
import {
  useCreateBookmark,
  useDeleteBookmark,
} from "@/queries/bookmarks";
import { useFeedLookup } from "@/queries/feeds";
import {
  useItem,
  useMarkItemsRead,
  useMarkItemsUnread,
  useOverviewKeptItems,
} from "@/queries/items";
import { usePreferencesStore, useReadStatePins } from "@/store";
import { autoMarkReadDelayMs } from "@/store/preferences";
import { TriageMetadata } from "./triage-metadata";

export function ArticleDrawer() {
  const { t } = useI18n();
  const {
    selectedArticleId,
    setSelectedArticle,
    setSelectedFeed,
    selectedFeedId,
    selectedGroupId,
    articleFilter,
    isOverviewRoute,
  } = useUrlState();
  const { getFeedById } = useFeedLookup();

  const { articles, isStarredMode, isItemStarred, getBookmarkByItemId } =
    useArticleList({
      feedId: selectedFeedId,
      groupId: selectedGroupId,
      articleFilter,
      enabled: !isOverviewRoute && selectedArticleId !== null,
    });

  const overviewKeptQuery = useOverviewKeptItems(50, isOverviewRoute);
  const overviewTopPicks = useMemo(
    () => selectTopPicks(overviewKeptQuery.data ?? [], 5),
    [overviewKeptQuery.data],
  );

  const activeArticles = isOverviewRoute ? overviewTopPicks : articles;
  const articleIds = useMemo(
    () => activeArticles.map((a) => a.id),
    [activeArticles],
  );

  const markRead = useMarkItemsRead();
  const markUnread = useMarkItemsUnread();
  const createBookmark = useCreateBookmark();
  const deleteBookmark = useDeleteBookmark();

  const { pinRead, unpinRead } = useReadStatePins({
    feedId: selectedFeedId,
    groupId: selectedGroupId,
    articleFilter,
  });

  const storeArticle = selectedArticleId
    ? (activeArticles.find((i) => i.id === selectedArticleId) ?? null)
    : null;

  const shouldFetchArticle =
    selectedArticleId !== null &&
    selectedArticleId > 0 &&
    ((!isOverviewRoute && isStarredMode) || storeArticle === null);
  const { data: fetchedArticle } = useItem(
    selectedArticleId,
    shouldFetchArticle,
  );

  const isOpen = selectedArticleId !== null;
  const article: Item | null =
    (!isOverviewRoute && isStarredMode
      ? (fetchedArticle ?? storeArticle)
      : (storeArticle ?? fetchedArticle)) ?? null;

  // Read state comes from the in-hand row (list/bookmark cache), not the
  // detail refetch: in starred mode the fetched detail can disagree with the
  // row, and the header toggle, the auto-mark timer, and the veto must all
  // watch the same unread bit or a manual mark-unread gets overwritten.
  const readArticle = storeArticle ?? fetchedArticle ?? null;
  const canToggleRead = readArticle !== null && readArticle.id > 0;
  const hasItemFetchedTime =
    article !== null &&
    article.id > 0 &&
    (!isStarredMode || fetchedArticle !== undefined);

  // Retain the last displayed article while the drawer slides closed so the
  // body does not disappear on frame 0 before the exit transition finishes.
  const [retainedSnapshot, setRetainedSnapshot] = useState<{
    article: Item;
    unread: boolean;
    canToggleRead: boolean;
    hasItemFetchedTime: boolean;
  } | null>(null);
  const clearTimerRef = useRef<number | null>(null);

  const clearRetainedSnapshot = useCallback(() => {
    if (clearTimerRef.current !== null) {
      window.clearTimeout(clearTimerRef.current);
      clearTimerRef.current = null;
    }
    setRetainedSnapshot(null);
  }, []);

  useEffect(() => {
    if (isOpen && article) {
      if (clearTimerRef.current !== null) {
        window.clearTimeout(clearTimerRef.current);
        clearTimerRef.current = null;
      }
      setRetainedSnapshot({
        article,
        unread: readArticle?.unread ?? article.unread,
        canToggleRead,
        hasItemFetchedTime,
      });
    }
  }, [isOpen, article, readArticle?.unread, canToggleRead, hasItemFetchedTime]);

  // Fallback cleanup when reduced-motion disables CSS transitions and
  // transitionend / onOpenChangeComplete may complete synchronously or be skipped.
  useEffect(() => {
    if (!isOpen && retainedSnapshot !== null) {
      clearTimerRef.current = window.setTimeout(() => {
        clearTimerRef.current = null;
        setRetainedSnapshot(null);
      }, 260);
      return () => {
        if (clearTimerRef.current !== null) {
          window.clearTimeout(clearTimerRef.current);
          clearTimerRef.current = null;
        }
      };
    }
    return undefined;
  }, [isOpen, retainedSnapshot]);

  const displayArticle = isOpen ? article : (retainedSnapshot?.article ?? null);
  const displayUnread = isOpen
    ? (readArticle?.unread ?? false)
    : (retainedSnapshot?.unread ?? false);
  const displayCanToggleRead = isOpen
    ? canToggleRead
    : (retainedSnapshot?.canToggleRead ?? false);
  const displayHasItemFetchedTime = isOpen
    ? hasItemFetchedTime
    : (retainedSnapshot?.hasItemFetchedTime ?? false);

  const feed = displayArticle ? getFeedById(displayArticle.feed_id) : null;
  const bookmark = displayArticle ? getBookmarkByItemId(displayArticle.id) : null;
  const starred = displayArticle ? isItemStarred(displayArticle.id) : false;
  const safeArticleLink = displayArticle
    ? toSafeExternalUrl(displayArticle.link)
    : null;

  const [originalToggleState, setOriginalToggleState] = useState<{
    articleId: number;
    open: boolean;
  } | null>(null);

  const defaultOriginalOpen = Boolean(
    displayArticle &&
      (!displayArticle.ai_summary.trim() || !displayArticle.source_id),
  );
  const isOriginalOpen =
    displayArticle && originalToggleState?.articleId === displayArticle.id
      ? originalToggleState.open
      : defaultOriginalOpen;

  const autoMarkRead = usePreferencesStore((s) => s.autoMarkRead);
  const autoMarkDelayMs = autoMarkReadDelayMs(autoMarkRead);

  useAutoMarkRead({
    target:
      readArticle && readArticle.id > 0
        ? { id: readArticle.id, unread: readArticle.unread }
        : null,
    delayMs: autoMarkDelayMs,
    onMarkRead: (id) => {
      markRead.mutate([id]);
      if (!isOverviewRoute) {
        pinRead([id]);
      }
    },
    hasPendingMutation: () => markRead.isPending || markUnread.isPending,
  });

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      setSelectedArticle(null);
    }
  };

  const handleToggleRead = async () => {
    if (!readArticle || !canToggleRead) return;
    try {
      if (readArticle.unread) {
        if (!isOverviewRoute) {
          pinRead([readArticle.id]);
        }
        await markRead.mutateAsync([readArticle.id]);
      } else {
        await markUnread.mutateAsync([readArticle.id]);
        if (!isOverviewRoute) {
          unpinRead([readArticle.id]);
        }
      }
    } catch (error) {
      console.error("Failed to toggle read status:", error);
    }
  };

  const handleToggleStar = async () => {
    if (!article) return;
    try {
      if (starred) {
        const existingBookmark = getBookmarkByItemId(article.id);
        if (existingBookmark) {
          await deleteBookmark.mutateAsync(existingBookmark.id);
        }
      } else {
        await createBookmark.mutateAsync(article);
      }
    } catch (error) {
      console.error("Failed to toggle star:", error);
    }
  };

  const handleOpenOriginal = () => {
    if (!safeArticleLink) return;
    window.open(safeArticleLink, "_blank", "noopener,noreferrer");
  };

  const handleOpenFeed = () => {
    if (!displayArticle || displayArticle.feed_id <= 0) return;
    const isUnscoredRss =
      feed && !feed.ai_enabled && !isPluginSourceFeed(feed.link);
    setSelectedFeed(
      displayArticle.feed_id,
      isUnscoredRss ? { triage_status: "all" } : undefined,
    );
  };

  const getLinkDomain = (url: string) => {
    try {
      return new URL(url).hostname;
    } catch {
      return url;
    }
  };

  const { goToNext, goToPrevious, hasNext, hasPrevious } =
    useArticleNavigation(articleIds, {
      enabled: selectedArticleId !== null,
      onToggleRead: () => {
        void handleToggleRead();
      },
      onToggleStar: () => {
        void handleToggleStar();
      },
      onOpenOriginal: handleOpenOriginal,
    });

  const rawMeta = useMemo(
    () => (displayArticle ? extractRawMeta(displayArticle.raw_meta) : null),
    [displayArticle],
  );

  const metricCells = useMemo(() => {
    if (!rawMeta || !rawMeta.hasMetrics) return [];
    const cells: Array<{
      key: string;
      label: string;
      value: number;
      Icon: typeof Heart;
    }> = [];
    if (rawMeta.metrics.likes !== undefined) {
      cells.push({
        key: "likes",
        label: t("panel.drawer.metricLikes"),
        value: rawMeta.metrics.likes,
        Icon: Heart,
      });
    }
    if (rawMeta.metrics.retweets !== undefined) {
      cells.push({
        key: "retweets",
        label: t("panel.drawer.metricRetweets"),
        value: rawMeta.metrics.retweets,
        Icon: Repeat2,
      });
    }
    if (rawMeta.metrics.replies !== undefined) {
      cells.push({
        key: "replies",
        label: t("panel.drawer.metricReplies"),
        value: rawMeta.metrics.replies,
        Icon: MessageCircle,
      });
    }
    if (rawMeta.metrics.views !== undefined) {
      cells.push({
        key: "views",
        label: t("panel.drawer.metricViews"),
        value: rawMeta.metrics.views,
        Icon: Eye,
      });
    }
    return cells;
  }, [rawMeta, t]);

  return (
    <Sheet
      open={isOpen}
      onOpenChange={handleOpenChange}
      onOpenChangeComplete={(open) => {
        if (!open) {
          clearRetainedSnapshot();
        }
      }}
    >
      <SheetContent
        side="right"
        className="np-drawer-glass border-0 p-0 data-[side=right]:w-full data-[side=right]:sm:max-w-[max(740px,54vw)]"
        showCloseButton={false}
      >
        {displayArticle && (
          <div className="flex h-full flex-col">
            {/* Sticky Glass Header Action Bar */}
            <div className="np-header-glass flex shrink-0 items-center justify-between gap-2 px-4 py-3 sm:px-6">
              <div className="flex flex-wrap items-center gap-1.5">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleToggleRead}
                  disabled={!displayCanToggleRead}
                  className="h-8 gap-1.5 rounded-xl border-[var(--glass-border)] bg-[var(--glass-panel)] px-2.5 text-xs font-medium text-[var(--glass-ink)]"
                >
                  {displayUnread ? (
                    <Circle className="h-3.5 w-3.5 text-[var(--glass-sub)]" />
                  ) : (
                    <CircleCheck className="h-3.5 w-3.5 text-[var(--glass-acc)]" />
                  )}
                  {displayUnread
                    ? t("article.action.markRead")
                    : t("article.action.markUnread")}
                </Button>

                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleToggleStar}
                  className={cn(
                    "h-8 gap-1.5 rounded-xl border-[var(--glass-border)] bg-[var(--glass-panel)] px-2.5 text-xs font-medium",
                    starred
                      ? "text-amber-600 dark:text-amber-400"
                      : "text-[var(--glass-ink)]",
                  )}
                >
                  <Star
                    className={cn("h-3.5 w-3.5", starred && "fill-current")}
                  />
                  {starred
                    ? t("article.action.unstar")
                    : t("article.action.star")}
                </Button>

                {safeArticleLink ? (
                  <a
                    href={safeArticleLink}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={cn(
                      buttonVariants({ variant: "outline", size: "sm" }),
                      "h-8 gap-1.5 rounded-xl border-[var(--glass-border)] bg-[var(--glass-panel)] px-2.5 text-xs font-medium text-[var(--glass-ink)]",
                    )}
                  >
                    <ExternalLink className="h-3.5 w-3.5" />
                    {t("article.action.original")}
                  </a>
                ) : (
                  <Button
                    variant="outline"
                    size="sm"
                    disabled
                    className="h-8 gap-1.5 rounded-xl border-[var(--glass-border)] bg-[var(--glass-panel)] px-2.5 text-xs font-medium text-[var(--glass-faint)]"
                  >
                    <ExternalLink className="h-3.5 w-3.5" />
                    {t("article.action.original")}
                  </Button>
                )}
              </div>

              <SheetTitle className="sr-only">
                {displayArticle.title || displayArticle.content.slice(0, 100)}
              </SheetTitle>

              <div className="flex shrink-0 items-center gap-1">
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={goToPrevious}
                  disabled={!hasPrevious()}
                  aria-label={t("common.previous")}
                  title={t("common.previous")}
                  className="h-8 w-8 rounded-xl"
                >
                  <ChevronLeft className="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={goToNext}
                  disabled={!hasNext()}
                  aria-label={t("common.next")}
                  title={t("common.next")}
                  className="h-8 w-8 rounded-xl"
                >
                  <ChevronRight className="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={() => setSelectedArticle(null)}
                  aria-label={t("common.cancel")}
                  className="h-8 w-8 rounded-xl"
                >
                  <X className="h-4 w-4 text-[var(--glass-sub)]" />
                </Button>
              </div>
            </div>

            {/* Scrollable Body */}
            <ScrollArea className="min-h-0 flex-1">
              <article className="min-w-0 space-y-5 px-5 py-6 sm:px-10 sm:py-7">
                {/* 1. Metadata Strip */}
                <TriageMetadata article={displayArticle} />

                {/* 2. Title & Source / Author */}
                <div className="space-y-3">
                  <h1 className="text-[22px] font-bold leading-[1.35] tracking-tight text-[var(--glass-ink)] sm:text-[26px]">
                    {displayArticle.title || displayArticle.content.slice(0, 100)}
                  </h1>

                  <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1.5 text-xs text-[var(--glass-sub)]">
                    {displayArticle.author && (
                      <span className="font-semibold text-[var(--glass-ink)]">
                        {displayArticle.author}
                      </span>
                    )}

                    {displayArticle.feed_id > 0 ? (
                      <button
                        type="button"
                        onClick={handleOpenFeed}
                        className="inline-flex max-w-52 items-center gap-1.5 rounded-lg bg-[var(--glass-soft)] px-2 py-1 font-medium text-[var(--glass-sub)] transition-colors hover:bg-[var(--glass-soft-hover)] hover:text-[var(--glass-ink)]"
                      >
                        {feed && !displayArticle.source_id && (
                          <FeedFavicon
                            src={getFaviconUrl(feed.link, feed.site_url)}
                            className="h-3.5 w-3.5 rounded-sm"
                          />
                        )}
                        <span className="truncate">
                          {displayArticle.source_id ||
                            feed?.name ||
                            bookmark?.feed_name ||
                            t("common.unknown")}
                        </span>
                      </button>
                    ) : (
                      <span className="inline-flex max-w-52 items-center gap-1.5 rounded-lg bg-[var(--glass-soft)] px-2 py-1 font-medium text-[var(--glass-sub)]">
                        <span className="truncate">
                          {displayArticle.source_id ||
                            bookmark?.feed_name ||
                            t("common.unknown")}
                        </span>
                      </span>
                    )}

                    <ItemTimestamps
                      item={displayArticle}
                      showFetched={displayHasItemFetchedTime}
                      variant="detailed"
                    />

                    {safeArticleLink && (
                      <a
                        href={safeArticleLink}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="truncate font-medium text-[var(--glass-acc)] hover:underline"
                      >
                        {getLinkDomain(safeArticleLink)}
                      </a>
                    )}
                  </div>
                </div>

                {/* 3. Highlighted AI Summary Glass Card */}
                {displayArticle.ai_summary.trim() && (
                  <section className="rounded-2xl border border-[var(--glass-acc)]/25 bg-[var(--glass-soft)] p-4 sm:p-5">
                    <div className="mb-2 flex items-center gap-2 text-xs font-bold tracking-wide text-[var(--glass-acc)]">
                      <Sparkles className="h-3.5 w-3.5" />
                      <span>{t("panel.drawer.aiSummaryTitle")}</span>
                    </div>
                    <p className="text-[14.5px] leading-[1.75] text-[var(--glass-ink)]">
                      {displayArticle.ai_summary}
                    </p>
                  </section>
                )}

                {/* 4. Smooth Collapsible Original Content Section */}
                <section className="overflow-hidden rounded-2xl border border-[var(--glass-inner-line)] bg-[var(--glass-panel)]">
                  <button
                    type="button"
                    aria-expanded={isOriginalOpen}
                    onClick={() =>
                      setOriginalToggleState({
                        articleId: displayArticle.id,
                        open: !isOriginalOpen,
                      })
                    }
                    className="flex w-full items-center justify-between gap-2 px-4 py-3 text-left text-xs font-semibold text-[var(--glass-sub)] transition-colors hover:bg-[var(--glass-soft)] hover:text-[var(--glass-ink)]"
                  >
                    <span>{t("panel.drawer.originalContentTitle")}</span>
                    <span className="inline-flex items-center gap-1 text-[11px] font-medium text-[var(--glass-faint)]">
                      {isOriginalOpen
                        ? t("panel.drawer.collapse")
                        : t("panel.drawer.expand")}
                      <ChevronDown
                        className={cn(
                          "h-4 w-4 transition-transform duration-200",
                          isOriginalOpen && "rotate-180",
                        )}
                      />
                    </span>
                  </button>

                  <div className="np-orig-collapse" data-open={isOriginalOpen}>
                    <div>
                      <div className="border-t border-[var(--glass-inner-line)] px-4 py-4 sm:px-5">
                        {displayArticle.source_id ? (
                          <div className="whitespace-pre-wrap break-words text-[14px] leading-[1.75] text-[var(--glass-ink)]">
                            {displayArticle.content}
                          </div>
                        ) : (
                          <div
                            className="typeset typeset-article min-w-0 max-w-none"
                            dangerouslySetInnerHTML={{
                              __html: processArticleContent(
                                displayArticle.content,
                                safeArticleLink ?? undefined,
                              ),
                            }}
                          />
                        )}
                      </div>
                    </div>
                  </div>
                </section>

                {/* 5. Conditional Engagement Metrics & Origin Label */}
                {rawMeta && (rawMeta.hasMetrics || rawMeta.originLabel) && (
                  <section className="rounded-2xl border border-[var(--glass-inner-line)] bg-[var(--glass-panel)] p-4">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <h2 className="text-xs font-semibold text-[var(--glass-sub)]">
                        {t("panel.drawer.engagementTitle")}
                      </h2>
                      {rawMeta.originLabel && (
                        <span className="rounded-full bg-[var(--glass-soft)] px-2.5 py-0.5 text-[11px] font-medium text-[var(--glass-acc)]">
                          {t("panel.drawer.originLabel", {
                            label: rawMeta.originLabel,
                          })}
                        </span>
                      )}
                    </div>

                    {metricCells.length > 0 && (
                      <div className="mt-3 grid grid-cols-2 gap-2.5 sm:grid-cols-4">
                        {metricCells.map(({ key, label, value, Icon }) => (
                          <div
                            key={key}
                            className="rounded-xl bg-[var(--glass-soft)] px-3 py-2.5 text-center"
                          >
                            <Icon className="mx-auto h-3.5 w-3.5 text-[var(--glass-acc)]" />
                            <p className="mt-1 text-base font-bold tabular-nums text-[var(--glass-ink)]">
                              {formatCompactNumber(value)}
                            </p>
                            <p className="text-[11px] text-[var(--glass-faint)]">
                              {label}
                            </p>
                          </div>
                        ))}
                      </div>
                    )}
                  </section>
                )}
              </article>
            </ScrollArea>

            {/* Footer Navigation */}
            <div className="flex shrink-0 items-center justify-between border-t border-[var(--glass-inner-line)] px-4 py-3 sm:px-6">
              <Button
                variant="outline"
                size="sm"
                onClick={goToPrevious}
                disabled={!hasPrevious()}
                className="h-8 gap-1.5 rounded-xl border-[var(--glass-border)] bg-[var(--glass-panel)] px-3 text-xs font-medium text-[var(--glass-sub)]"
              >
                <ChevronLeft className="h-3.5 w-3.5" />
                {t("common.previous")}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={goToNext}
                disabled={!hasNext()}
                className="h-8 gap-1.5 rounded-xl border-[var(--glass-border)] bg-[var(--glass-panel)] px-3 text-xs font-medium text-[var(--glass-sub)]"
              >
                {t("common.next")}
                <ChevronRight className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
