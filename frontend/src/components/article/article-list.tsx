import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { Loader2, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { TriageToolbar } from "@/components/panel/triage-toolbar";
import { sortLoadedArticles } from "@/components/panel/panel-utils";
import { ArticleItem } from "./article-item";
import { useArticleNavigation } from "@/hooks/use-keyboard";
import { useUrlState } from "@/hooks/use-url-state";
import { useArticleList } from "@/hooks/use-article-list";
import { useMarkItemsRead, useMarkItemsUnread } from "@/queries/items";
import { useFeedLookup } from "@/queries/feeds";
import { useGroups } from "@/queries/groups";
import { useCreateBookmark, useDeleteBookmark } from "@/queries/bookmarks";
import { getFaviconUrl } from "@/lib/api/favicon";
import { useI18n } from "@/lib/i18n";
import {
  articleListKey,
  useArticlePinsStore,
  useReadStatePins,
  useUIStore,
} from "@/store";
import type { Item } from "@/lib/api";

export function ArticleList() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const setSearchOpen = useUIStore((s) => s.setSearchOpen);

  const {
    articleFilter,
    setArticleFilter,
    triageSearch,
    setTriageSearch,
    selectedFeedId,
    selectedGroupId,
    selectedArticleId,
    setSelectedArticle,
  } = useUrlState();

  const [sortBy, setSortBy] = useState<"time" | "score">("time");
  const [isRefreshing, setIsRefreshing] = useState(false);

  const {
    articles,
    hasMore,
    isLoading,
    isLoadingMore,
    isStarredMode,
    fetchNextPage,
    isItemStarred,
    getBookmarkByItemId,
  } = useArticleList({
    feedId: selectedFeedId,
    groupId: selectedGroupId,
    articleFilter,
  });

  const { data: groups = [] } = useGroups();
  const { feeds, getFeedById, isLoading: isFeedsLoading } = useFeedLookup();
  const markItemsRead = useMarkItemsRead();
  const markItemsUnread = useMarkItemsUnread();
  const createBookmark = useCreateBookmark();
  const deleteBookmark = useDeleteBookmark();

  const { pinRead, unpinRead } = useReadStatePins({
    feedId: selectedFeedId,
    groupId: selectedGroupId,
    articleFilter,
  });

  const pinsKey = articleListKey(selectedFeedId, selectedGroupId, articleFilter);
  const clearPins = useArticlePinsStore((s) => s.clearPins);

  useEffect(() => {
    return () => {
      clearPins(pinsKey);
    };
  }, [pinsKey, clearPins]);

  const displayedArticles = useMemo(
    () => sortLoadedArticles(articles, sortBy),
    [articles, sortBy],
  );

  const articleIds = useMemo(
    () => displayedArticles.map((a) => a.id),
    [displayedArticles],
  );
  useArticleNavigation(articleIds, {
    enabled: selectedArticleId === null,
  });

  useEffect(() => {
    if (isLoading || isLoadingMore) return;
    if (articles.length === 0 && hasMore) {
      fetchNextPage();
    }
  }, [articles.length, hasMore, isLoading, isLoadingMore, fetchNextPage]);

  let title = t("article.list.all");
  if (selectedFeedId) {
    const feed = getFeedById(selectedFeedId);
    title = feed?.name ?? t("article.feedFallback");
  } else if (selectedGroupId) {
    const group = groups.find((g) => g.id === selectedGroupId);
    title = group?.name ?? t("article.groupFallback");
  } else if (triageSearch.triage_status === "filtered") {
    title = t("panel.nav.filtered");
  } else if (triageSearch.triage_status === "unscored") {
    title = t("panel.nav.unscored");
  } else if (articleFilter === "unread") {
    title = t("article.filter.unread");
  } else if (articleFilter === "starred") {
    title = t("article.filter.starred");
  }

  const unreadCount = displayedArticles.filter((a) => a.unread).length;
  const hasNoFeeds = !isFeedsLoading && feeds.length === 0;
  const hasActiveTriageFilters = Boolean(
    triageSearch.domain ||
      triageSearch.priority ||
      triageSearch.feed_id !== undefined ||
      (triageSearch.min_score !== undefined && triageSearch.min_score > 0) ||
      (triageSearch.triage_status && triageSearch.triage_status !== "kept"),
  );

  const handleRefresh = useCallback(async () => {
    setIsRefreshing(true);
    try {
      await queryClient.invalidateQueries();
    } finally {
      window.setTimeout(() => setIsRefreshing(false), 320);
    }
  }, [queryClient]);

  const handleToggleRead = useCallback(
    async (article: Item) => {
      if (article.id <= 0) return;

      try {
        if (article.unread) {
          pinRead([article.id]);
          await markItemsRead.mutateAsync([article.id]);
        } else {
          await markItemsUnread.mutateAsync([article.id]);
          unpinRead([article.id]);
        }
      } catch (error) {
        console.error("Failed to toggle read status:", error);
      }
    },
    [markItemsRead, markItemsUnread, pinRead, unpinRead],
  );

  const handleToggleStar = useCallback(
    async (article: Item) => {
      try {
        if (isItemStarred(article.id)) {
          const bookmark = getBookmarkByItemId(article.id);
          if (bookmark) {
            await deleteBookmark.mutateAsync(bookmark.id);
          }
          return;
        }

        await createBookmark.mutateAsync(article);
      } catch (error) {
        console.error("Failed to toggle star:", error);
      }
    },
    [createBookmark, deleteBookmark, getBookmarkByItemId, isItemStarred],
  );

  const handleMarkAllAsRead = async () => {
    const unreadIds = displayedArticles
      .filter((a) => a.unread && a.id > 0)
      .map((a) => a.id);

    if (unreadIds.length === 0) return;

    try {
      await markItemsRead.mutateAsync(unreadIds);
      pinRead(unreadIds);
    } catch (error) {
      console.error("Failed to mark all as read:", error);
    }
  };

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden">
      <TriageToolbar
        title={title}
        articleFilter={articleFilter}
        triageSearch={triageSearch}
        feeds={feeds}
        isFeedRoute={selectedFeedId !== null}
        sortBy={sortBy}
        unreadCount={unreadCount}
        isRefreshing={isRefreshing}
        onChangeArticleFilter={setArticleFilter}
        onUpdateTriage={setTriageSearch}
        onChangeSortBy={setSortBy}
        onOpenSearch={() => setSearchOpen(true)}
        onRefresh={() => void handleRefresh()}
        onMarkAllRead={() => void handleMarkAllAsRead()}
      />

      <ScrollArea className="min-h-0 flex-1">
        <div className="mx-auto w-full max-w-[1440px] px-4 pb-8 pt-1 sm:px-6">
          <div className="np-glass overflow-hidden">
            {isLoading || (displayedArticles.length === 0 && hasMore) ? (
              <div className="space-y-3 p-4 sm:p-5">
                {[1, 2, 3, 4, 5].map((i) => (
                  <div
                    key={i}
                    className="h-24 animate-pulse rounded-2xl bg-[var(--glass-soft)]"
                  />
                ))}
              </div>
            ) : displayedArticles.length === 0 ? (
              hasNoFeeds && !hasActiveTriageFilters ? (
                <div className="flex flex-col items-center justify-center gap-3 px-6 py-16 text-center">
                  <p className="text-sm font-medium text-[var(--glass-sub)]">
                    {t("article.list.noFeeds")}
                  </p>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => void navigate({ to: "/feeds" })}
                  >
                    {t("article.list.openFeedManagement")}
                  </Button>
                </div>
              ) : (
                <div className="flex flex-col items-center justify-center gap-3 px-6 py-16 text-center">
                  <p className="text-sm font-medium text-[var(--glass-sub)]">
                    {t("article.list.noArticles")}
                  </p>
                  {hasActiveTriageFilters && (
                    <Button
                      variant="outline"
                      size="sm"
                      className="gap-1.5"
                      onClick={() => {
                        setSortBy("time");
                        setTriageSearch({}, { reset: true });
                      }}
                    >
                      <RotateCcw className="h-3.5 w-3.5" />
                      {t("panel.toolbar.resetFilters")}
                    </Button>
                  )}
                </div>
              )
            ) : (
              <>
                <div>
                  {displayedArticles.map((article) => {
                    const feed = getFeedById(article.feed_id);
                    const bookmark = getBookmarkByItemId(article.id);

                    return (
                      <ArticleItem
                        key={article.id}
                        article={article}
                        selectedArticleId={selectedArticleId}
                        onSelectArticle={setSelectedArticle}
                        onToggleRead={handleToggleRead}
                        onToggleStar={handleToggleStar}
                        canToggleRead={article.id > 0}
                        isStarred={isItemStarred(article.id)}
                        showFetchedTime={!isStarredMode}
                        feedName={
                          feed?.name ??
                          bookmark?.feed_name ??
                          t("common.unknown")
                        }
                        feedFaviconUrl={
                          feed ? getFaviconUrl(feed.link, feed.site_url) : null
                        }
                      />
                    );
                  })}
                </div>
                {hasMore && (
                  <div className="flex justify-center border-t border-[var(--glass-inner-line)] py-4">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => fetchNextPage()}
                      disabled={isLoadingMore}
                      className="gap-2 rounded-xl border-[var(--glass-border)] bg-[var(--glass-panel)]"
                    >
                      {isLoadingMore && (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      )}
                      {isLoadingMore
                        ? t("article.list.loading")
                        : t("article.list.loadMore")}
                    </Button>
                  </div>
                )}
              </>
            )}
          </div>
        </div>
      </ScrollArea>
    </div>
  );
}
