import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { RefreshCw, Settings } from "lucide-react";
import { AppLayout } from "@/components/layout/app-layout";
import { SidebarTrigger } from "@/components/layout/sidebar-trigger";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useArticleNavigation } from "@/hooks/use-keyboard";
import { useUrlState } from "@/hooks/use-url-state";
import type { Item } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import {
  useBookmarkLookup,
  useCreateBookmark,
  useDeleteBookmark,
} from "@/queries/bookmarks";
import { useFeedLookup } from "@/queries/feeds";
import { useOverviewCounts, useOverviewKeptItems } from "@/queries/items";
import { usePluginHeartbeats } from "@/queries/plugins";
import { useUIStore } from "@/store";
import { DomainPulseCard } from "./domain-pulse-card";
import { KpiStrip } from "./kpi-strip";
import {
  computeDomainPulse,
  getGreetingKey,
  selectTopPicks,
  type PanelDomain,
} from "./panel-utils";
import { SourceHealthCard } from "./source-health-card";
import { ThemeToggleButton } from "./theme-toggle-button";
import { TopPicksCard } from "./top-picks-card";

export function OverviewPage() {
  const { t, locale } = useI18n();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const setSettingsOpen = useUIStore((s) => s.setSettingsOpen);
  const { selectedArticleId, setSelectedArticle } = useUrlState();

  const { unreadKept, kept, filtered } = useOverviewCounts();
  const keptItemsQuery = useOverviewKeptItems(50);
  const heartbeatsQuery = usePluginHeartbeats();
  const { total: starredTotal, isItemStarred, getBookmarkByItemId } =
    useBookmarkLookup();
  const { getFeedById } = useFeedLookup();
  const createBookmark = useCreateBookmark();
  const deleteBookmark = useDeleteBookmark();

  const [isRefreshing, setIsRefreshing] = useState(false);
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const timer = window.setInterval(() => setNow(new Date()), 60_000);
    return () => window.clearInterval(timer);
  }, []);

  const keptItems = useMemo(
    () => keptItemsQuery.data ?? [],
    [keptItemsQuery.data],
  );
  const topPicks = useMemo(() => selectTopPicks(keptItems, 5), [keptItems]);
  const domainPulseRows = useMemo(
    () => computeDomainPulse(keptItems),
    [keptItems],
  );
  const topPickIds = useMemo(() => topPicks.map((item) => item.id), [topPicks]);

  useArticleNavigation(topPickIds, {
    enabled: selectedArticleId === null,
  });

  const handleRefresh = useCallback(async () => {
    setIsRefreshing(true);
    try {
      await queryClient.invalidateQueries();
    } finally {
      window.setTimeout(() => setIsRefreshing(false), 320);
    }
  }, [queryClient]);

  const handleToggleStar = useCallback(
    async (item: Item) => {
      try {
        if (isItemStarred(item.id)) {
          const bookmark = getBookmarkByItemId(item.id);
          if (bookmark) {
            await deleteBookmark.mutateAsync(bookmark.id);
          }
          return;
        }
        await createBookmark.mutateAsync(item);
      } catch (error) {
        console.error("Failed to toggle star:", error);
      }
    },
    [createBookmark, deleteBookmark, getBookmarkByItemId, isItemStarred],
  );

  const handleSelectDomain = useCallback(
    (domain: PanelDomain) => {
      void navigate({
        to: "/$filter",
        params: { filter: "all" },
        search: { domain },
      });
    },
    [navigate],
  );

  const formattedDateTime = useMemo(
    () =>
      new Intl.DateTimeFormat(locale, {
        month: "short",
        day: "numeric",
        weekday: "short",
        hour: "2-digit",
        minute: "2-digit",
        hour12: false,
      }).format(now),
    [locale, now],
  );

  const greetingText = t(getGreetingKey(now.getHours()));
  const kpiLoading =
    unreadKept.isLoading || kept.isLoading || filtered.isLoading;

  return (
    <AppLayout>
      <div className="flex h-full min-h-0 flex-col overflow-hidden">
        {/* Compact 60px Glass Header */}
        <header className="np-header-glass flex h-[60px] shrink-0 items-center justify-between gap-3 px-4 sm:px-6">
          <div className="flex min-w-0 items-center gap-2.5">
            <SidebarTrigger />
            <div className="min-w-0">
              <div className="flex items-baseline gap-2.5">
                <h1 className="truncate text-[18px] font-bold tracking-tight text-[var(--glass-ink)] sm:text-[20px]">
                  {greetingText}
                </h1>
                <span className="hidden truncate text-xs text-[var(--glass-sub)] md:inline">
                  {t("overview.statusLine", {
                    unread: unreadKept.data ?? 0,
                    kept: kept.data ?? 0,
                  })}
                </span>
              </div>
              <p className="truncate text-[11.5px] text-[var(--glass-sub)] md:hidden">
                {t("overview.statusLine", {
                  unread: unreadKept.data ?? 0,
                  kept: kept.data ?? 0,
                })}
              </p>
            </div>
          </div>

          <div className="flex shrink-0 items-center gap-2">
            <span className="hidden rounded-xl border border-[var(--glass-border)] bg-[var(--glass-panel)] px-3 py-1.5 text-xs font-medium tabular-nums text-[var(--glass-sub)] lg:inline-flex">
              {formattedDateTime}
            </span>
            <button
              type="button"
              onClick={() => void handleRefresh()}
              aria-label={t("common.refresh")}
              title={t("common.refresh")}
              className="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-[var(--glass-border)] bg-[var(--glass-panel)] text-[var(--glass-sub)] shadow-xs backdrop-blur-md transition-colors hover:bg-[var(--glass-soft-hover)] hover:text-[var(--glass-ink)]"
            >
              <RefreshCw
                className={cn("h-4 w-4", isRefreshing && "animate-spin")}
              />
            </button>
            <ThemeToggleButton />
            <button
              type="button"
              onClick={() => setSettingsOpen(true)}
              aria-label={t("common.settings")}
              title={t("common.settings")}
              className="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-[var(--glass-border)] bg-[var(--glass-panel)] text-[var(--glass-sub)] shadow-xs backdrop-blur-md transition-colors hover:bg-[var(--glass-soft-hover)] hover:text-[var(--glass-ink)]"
            >
              <Settings className="h-4 w-4" />
            </button>
          </div>
        </header>

        <ScrollArea className="min-h-0 flex-1">
          <div className="mx-auto flex w-full max-w-[1440px] flex-col gap-5 p-4 sm:p-5 lg:p-6">
            {/* Row 1: 4 Clickable Real-Data KPI Cards */}
            <KpiStrip
              unreadCount={unreadKept.data}
              keptCount={kept.data}
              filteredCount={filtered.data}
              starredCount={starredTotal}
              isLoading={kpiLoading}
              onClickUnread={() =>
                void navigate({
                  to: "/$filter",
                  params: { filter: "unread" },
                })
              }
              onClickKept={() =>
                void navigate({
                  to: "/$filter",
                  params: { filter: "all" },
                  search: { triage_status: "kept" },
                })
              }
              onClickFiltered={() =>
                void navigate({
                  to: "/$filter",
                  params: { filter: "all" },
                  search: { triage_status: "filtered" },
                })
              }
              onClickStarred={() =>
                void navigate({
                  to: "/$filter",
                  params: { filter: "starred" },
                })
              }
            />

            {/* Row 2: Horizontal 2-Column Bento Main Workspace */}
            <div className="grid grid-cols-1 items-start gap-5 xl:grid-cols-[minmax(0,1.65fr)_340px]">
              <TopPicksCard
                items={topPicks}
                isLoading={keptItemsQuery.isLoading}
                isError={keptItemsQuery.isError}
                selectedArticleId={selectedArticleId}
                onSelectArticle={(id) => setSelectedArticle(id)}
                isItemStarred={isItemStarred}
                onToggleStar={handleToggleStar}
                getFeedById={getFeedById}
                onViewAll={() =>
                  void navigate({
                    to: "/$filter",
                    params: { filter: "unread" },
                  })
                }
              />

              <div className="flex flex-col gap-5">
                <DomainPulseCard
                  rows={domainPulseRows}
                  isLoading={keptItemsQuery.isLoading}
                  onSelectDomain={handleSelectDomain}
                />
                <SourceHealthCard
                  heartbeats={heartbeatsQuery.data ?? []}
                  isLoading={heartbeatsQuery.isLoading}
                  isError={heartbeatsQuery.isError}
                  onViewSources={() => void navigate({ to: "/sources" })}
                />
              </div>
            </div>
          </div>
        </ScrollArea>
      </div>
    </AppLayout>
  );
}
