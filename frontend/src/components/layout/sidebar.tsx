import { useState } from "react";
import {
  ChevronDown,
  Cpu,
  Globe,
  Inbox,
  Layers,
  LayoutDashboard,
  LogOut,
  Newspaper,
  Radio,
  Rss,
  Search,
  Settings,
  ShieldAlert,
  Star,
  Tag,
  TrendingUp,
} from "lucide-react";
import { useLocation, useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { FeedList } from "@/components/feed/feed-list";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  getDomainVisual,
  PANEL_DOMAINS,
  summarizeHeartbeats,
  type PanelDomain,
} from "@/components/panel/panel-utils";
import { useUrlState } from "@/hooks/use-url-state";
import { sessionAPI } from "@/lib/api";
import { isArticleFilter } from "@/lib/article-filter";
import { useI18n, type TranslationKey } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { useBookmarkLookup } from "@/queries/bookmarks";
import { useUnreadCounts } from "@/queries/feeds";
import { useOverviewCounts } from "@/queries/items";
import { usePluginHeartbeats } from "@/queries/plugins";
import { resetSessionGuard } from "@/routes/__root";
import { useUIStore } from "@/store";

const DOMAIN_META: Record<
  PanelDomain,
  { labelKey: TranslationKey; Icon: typeof Cpu }
> = {
  ai: { labelKey: "panel.domain.ai", Icon: Cpu },
  investing: { labelKey: "panel.domain.investing", Icon: TrendingUp },
  deals: { labelKey: "panel.domain.deals", Icon: Tag },
  other: { labelKey: "panel.domain.other", Icon: Globe },
};

export function Sidebar() {
  const { t } = useI18n();
  const { setSearchOpen, setSettingsOpen } = useUIStore();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { pathname } = useLocation();
  const {
    selectedFeedId,
    selectedGroupId,
    articleFilter,
    triageSearch,
    selectTopLevelFilter,
  } = useUrlState();

  const { getTotalUnreadCount } = useUnreadCounts();
  const { total: starredTotal } = useBookmarkLookup();
  const { unreadKept, filtered } = useOverviewCounts();
  const { data: heartbeats = [] } = usePluginHeartbeats();

  const [feedsSectionOpen, setFeedsSectionOpen] = useState(false);

  const firstSegment = pathname.split("/").filter(Boolean)[0];
  const isOnArticleList =
    typeof firstSegment === "string" && isArticleFilter(firstSegment);
  const isTopLevelArticleRoute =
    isOnArticleList && selectedFeedId === null && selectedGroupId === null;

  const isOverviewPage = pathname === "/overview";
  const isSourcesPage = pathname === "/sources";
  const isFeedsPage = pathname === "/feeds";
  const isFilteredView =
    isTopLevelArticleRoute && triageSearch.triage_status === "filtered";
  const activeDomain =
    isTopLevelArticleRoute && !isFilteredView ? triageSearch.domain : undefined;

  const unreadBadge = unreadKept.data ?? getTotalUnreadCount();
  const filteredBadge = filtered.data;
  const hbSummary = summarizeHeartbeats(heartbeats);
  const healthRatio =
    hbSummary.total > 0 ? Math.round((hbSummary.ok / hbSummary.total) * 100) : 0;

  const handleSignOut = async () => {
    if (!window.confirm(t("sidebar.signOutConfirm"))) {
      return;
    }
    try {
      await sessionAPI.logout();
    } catch {
      toast.error(t("sidebar.signOutFailed"));
      return;
    }

    queryClient.clear();
    resetSessionGuard();
    window.location.assign("/login");
  };

  return (
    <aside className="sidebar-typography np-sidebar-glass flex h-full w-[252px] flex-none flex-col overflow-hidden text-[var(--glass-sub)]">
      {/* Brand Header */}
      <div className="flex h-[60px] shrink-0 items-center gap-3 px-4">
        <button
          type="button"
          onClick={() => void navigate({ to: "/overview" })}
          className="flex min-w-0 items-center gap-3 text-left"
        >
          <span
            className="np-tile np-tile-sm"
            style={{
              background: "linear-gradient(135deg, #5B5BEA 0%, #9A7BFF 100%)",
            }}
          >
            <Layers className="h-[17px] w-[17px]" strokeWidth={2.2} />
          </span>
          <div className="min-w-0">
            <p className="truncate text-[15px] font-bold leading-tight tracking-tight text-[var(--glass-ink)]">
              news_panel
            </p>
            <p className="truncate text-[11px] font-medium text-[var(--glass-faint)]">
              {t("panel.brand.subtitle")}
            </p>
          </div>
        </button>
      </div>

      {/* Global Search Trigger Pill */}
      <div className="px-3 pb-2">
        <button
          type="button"
          className="flex h-9 w-full items-center justify-between rounded-full border border-[var(--glass-border)] bg-[var(--glass-panel)] px-3.5 text-xs text-[var(--glass-sub)] shadow-xs transition-colors hover:border-[var(--glass-acc)]/40 hover:text-[var(--glass-ink)]"
          onClick={() => setSearchOpen(true)}
        >
          <span className="flex items-center gap-2">
            <Search className="h-3.5 w-3.5 text-[var(--glass-faint)]" />
            <span>{t("sidebar.search")}</span>
          </span>
          <kbd className="rounded-md bg-[var(--glass-soft)] px-1.5 py-0.5 font-mono text-[10px] font-semibold text-[var(--glass-faint)]">
            ⌘K / ?
          </kbd>
        </button>
      </div>

      {/* Scrollable Navigation Body */}
      <ScrollArea className="min-h-0 flex-1 w-full min-w-0 overflow-hidden [&_[data-slot=scroll-area-viewport]>div]:!block">
        <div className="space-y-4 px-3 py-2">
          {/* Primary Workspace Nav */}
          <nav aria-label={t("panel.nav.workspace")} className="space-y-0.5">
            <p className="px-2.5 pb-1 text-[10.5px] font-semibold uppercase tracking-wider text-[var(--glass-faint)]">
              {t("panel.nav.workspace")}
            </p>
            <button
              type="button"
              aria-current={isOverviewPage ? "page" : undefined}
              onClick={() => void navigate({ to: "/overview" })}
              className={cn(
                "flex w-full items-center gap-2.5 rounded-xl px-2.5 py-2 text-[13.5px] transition-colors",
                isOverviewPage
                  ? "bg-[var(--glass-soft)] font-semibold text-[var(--glass-acc)]"
                  : "hover:bg-[var(--glass-soft)] hover:text-[var(--glass-ink)]",
              )}
            >
              <LayoutDashboard className="h-4 w-4 shrink-0" />
              <span className="flex-1 truncate text-left">
                {t("panel.nav.overview")}
              </span>
            </button>

            <button
              type="button"
              aria-current={
                isTopLevelArticleRoute &&
                articleFilter === "all" &&
                !isFilteredView &&
                !activeDomain
                  ? "page"
                  : undefined
              }
              onClick={() => selectTopLevelFilter("all")}
              className={cn(
                "flex w-full items-center gap-2.5 rounded-xl px-2.5 py-2 text-[13.5px] transition-colors",
                isTopLevelArticleRoute &&
                  articleFilter === "all" &&
                  !isFilteredView &&
                  !activeDomain
                  ? "bg-[var(--glass-soft)] font-semibold text-[var(--glass-acc)]"
                  : "hover:bg-[var(--glass-soft)] hover:text-[var(--glass-ink)]",
              )}
            >
              <Newspaper className="h-4 w-4 shrink-0" />
              <span className="flex-1 truncate text-left">
                {t("panel.nav.all")}
              </span>
            </button>

            <button
              type="button"
              aria-current={
                isTopLevelArticleRoute && articleFilter === "unread"
                  ? "page"
                  : undefined
              }
              onClick={() => selectTopLevelFilter("unread")}
              className={cn(
                "flex w-full items-center gap-2.5 rounded-xl px-2.5 py-2 text-[13.5px] transition-colors",
                isTopLevelArticleRoute && articleFilter === "unread"
                  ? "bg-[var(--glass-soft)] font-semibold text-[var(--glass-acc)]"
                  : "hover:bg-[var(--glass-soft)] hover:text-[var(--glass-ink)]",
              )}
            >
              <Inbox className="h-4 w-4 shrink-0" />
              <span className="flex-1 truncate text-left">
                {t("article.filter.unread")}
              </span>
              {unreadBadge > 0 && (
                <span className="rounded-full bg-[var(--glass-acc)] px-2 py-0.5 text-[11px] font-semibold leading-none tabular-nums text-white dark:text-[#0b0d1f]">
                  {unreadBadge}
                </span>
              )}
            </button>

            <button
              type="button"
              aria-current={
                isTopLevelArticleRoute && articleFilter === "starred"
                  ? "page"
                  : undefined
              }
              onClick={() => selectTopLevelFilter("starred")}
              className={cn(
                "flex w-full items-center gap-2.5 rounded-xl px-2.5 py-2 text-[13.5px] transition-colors",
                isTopLevelArticleRoute && articleFilter === "starred"
                  ? "bg-[var(--glass-soft)] font-semibold text-[var(--glass-acc)]"
                  : "hover:bg-[var(--glass-soft)] hover:text-[var(--glass-ink)]",
              )}
            >
              <Star className="h-4 w-4 shrink-0" />
              <span className="flex-1 truncate text-left">
                {t("article.filter.starred")}
              </span>
              {starredTotal > 0 && (
                <span className="text-[11px] font-semibold tabular-nums text-[var(--glass-faint)]">
                  {starredTotal}
                </span>
              )}
            </button>
          </nav>

          {/* AI Triage & Domain Quick Access */}
          <nav aria-label={t("panel.nav.triage")} className="space-y-1">
            <p className="px-2.5 pb-1 text-[10.5px] font-semibold uppercase tracking-wider text-[var(--glass-faint)]">
              {t("panel.nav.triage")}
            </p>

            {/* Highlighted Filtered Noise Audit Shortcut */}
            <button
              type="button"
              aria-current={isFilteredView ? "page" : undefined}
              onClick={() =>
                selectTopLevelFilter("all", { triage_status: "filtered" })
              }
              className={cn(
                "flex w-full items-center gap-2.5 rounded-xl border px-2.5 py-2 text-[13px] transition-colors",
                isFilteredView
                  ? "border-[var(--prio-critical)]/40 bg-[var(--prio-critical)]/12 font-semibold text-[var(--prio-critical)]"
                  : "border-[var(--prio-critical)]/20 bg-[var(--prio-critical)]/6 text-[var(--glass-ink)] hover:bg-[var(--prio-critical)]/12",
              )}
            >
              <ShieldAlert className="h-4 w-4 shrink-0 text-[var(--prio-critical)]" />
              <span className="flex-1 truncate text-left">
                {t("panel.nav.filtered")}
              </span>
              {typeof filteredBadge === "number" && (
                <span
                  className="rounded-full px-1.5 py-0.5 text-[10.5px] font-bold leading-none tabular-nums"
                  style={{
                    color: "var(--prio-critical)",
                    backgroundColor:
                      "color-mix(in srgb, var(--prio-critical) 16%, transparent)",
                  }}
                >
                  {filteredBadge}
                </span>
              )}
            </button>

            {/* Domain Quick Filters */}
            <div className="space-y-0.5 pt-1">
              {PANEL_DOMAINS.map((domain) => {
                const { labelKey, Icon } = DOMAIN_META[domain];
                const visual = getDomainVisual(domain);
                const isActive = activeDomain === domain;
                return (
                  <button
                    key={domain}
                    type="button"
                    aria-current={isActive ? "page" : undefined}
                    onClick={() => selectTopLevelFilter("all", { domain })}
                    className={cn(
                      "flex w-full items-center gap-2.5 rounded-xl px-2.5 py-1.5 text-[13px] transition-colors",
                      isActive
                        ? "bg-[var(--glass-soft)] font-semibold text-[var(--glass-acc)]"
                        : "hover:bg-[var(--glass-soft)] hover:text-[var(--glass-ink)]",
                    )}
                  >
                    <span
                      className="grid h-5 w-5 shrink-0 place-items-center rounded-md text-white"
                      style={{ background: visual.gradient }}
                    >
                      <Icon className="h-3 w-3" />
                    </span>
                    <span className="flex-1 truncate text-left">
                      {t(labelKey)}
                    </span>
                  </button>
                );
              })}
            </div>
          </nav>

          {/* Collapsible Feed Groups Tree */}
          <div className="border-t border-[var(--glass-inner-line)] pt-2">
            <button
              type="button"
              aria-expanded={feedsSectionOpen}
              onClick={() => setFeedsSectionOpen((prev) => !prev)}
              className="flex w-full items-center justify-between rounded-xl px-2.5 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-[var(--glass-faint)] transition-colors hover:bg-[var(--glass-soft)] hover:text-[var(--glass-ink)]"
            >
              <span>{t("search.group.feeds")}</span>
              <ChevronDown
                className={cn(
                  "h-3.5 w-3.5 transition-transform duration-200",
                  feedsSectionOpen && "rotate-180",
                )}
              />
            </button>
            <div className="np-orig-collapse" data-open={feedsSectionOpen}>
              <div>
                <div className="pt-1">
                  <FeedList />
                </div>
              </div>
            </div>
          </div>
        </div>
      </ScrollArea>

      {/* Footer: Operations + Compact Health Card */}
      <div className="shrink-0 space-y-1 border-t border-[var(--glass-inner-line)] px-3 py-2.5">
        <button
          type="button"
          aria-current={isSourcesPage ? "page" : undefined}
          onClick={() => void navigate({ to: "/sources" })}
          className={cn(
            "flex w-full items-center gap-2.5 rounded-xl px-2.5 py-1.5 text-[13px] transition-colors",
            isSourcesPage
              ? "bg-[var(--glass-soft)] font-semibold text-[var(--glass-acc)]"
              : "hover:bg-[var(--glass-soft)] hover:text-[var(--glass-ink)]",
          )}
        >
          <Radio className="h-4 w-4 shrink-0" />
          <span className="flex-1 truncate text-left">
            {t("panel.nav.sources")}
          </span>
        </button>
        <button
          type="button"
          aria-current={isFeedsPage ? "page" : undefined}
          onClick={() => void navigate({ to: "/feeds" })}
          className={cn(
            "flex w-full items-center gap-2.5 rounded-xl px-2.5 py-1.5 text-[13px] transition-colors",
            isFeedsPage
              ? "bg-[var(--glass-soft)] font-semibold text-[var(--glass-acc)]"
              : "hover:bg-[var(--glass-soft)] hover:text-[var(--glass-ink)]",
          )}
        >
          <Rss className="h-4 w-4 shrink-0" />
          <span className="flex-1 truncate text-left">
            {t("sidebar.manageFeeds")}
          </span>
        </button>

        <div className="flex items-center gap-1 pt-0.5">
          <button
            type="button"
            className="flex flex-1 items-center gap-2 rounded-xl px-2.5 py-1.5 text-[12.5px] transition-colors hover:bg-[var(--glass-soft)] hover:text-[var(--glass-ink)]"
            onClick={() => setSettingsOpen(true)}
          >
            <Settings className="h-3.5 w-3.5 shrink-0" />
            <span className="truncate">{t("sidebar.settings")}</span>
          </button>
          <button
            type="button"
            className="flex items-center gap-1.5 rounded-xl px-2.5 py-1.5 text-[12.5px] transition-colors hover:bg-[var(--glass-soft)] hover:text-[var(--prio-critical)]"
            onClick={() => void handleSignOut()}
            title={t("sidebar.signOut")}
            aria-label={t("sidebar.signOut")}
          >
            <LogOut className="h-3.5 w-3.5 shrink-0" />
          </button>
        </div>

        {/* Compact Plugin Health Glass Card */}
        <button
          type="button"
          onClick={() => void navigate({ to: "/sources" })}
          className="np-glass-sm mt-1.5 block w-full p-3 text-left transition-colors hover:border-[var(--glass-acc)]/40"
        >
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-semibold text-[var(--glass-ink)]">
              {t("panel.healthCard.title")}
            </span>
            <span
              className="h-2 w-2 rounded-full"
              style={{
                backgroundColor:
                  hbSummary.error > 0
                    ? "var(--prio-critical)"
                    : hbSummary.degraded > 0
                      ? "var(--prio-high)"
                      : hbSummary.total > 0
                        ? "var(--status-ok)"
                        : "var(--glass-faint)",
              }}
            />
          </div>
          <p className="mt-1 text-[11px] tabular-nums text-[var(--glass-sub)]">
            {hbSummary.total > 0
              ? t("panel.healthCard.summary", {
                  ok: hbSummary.ok,
                  total: hbSummary.total,
                })
              : t("panel.healthCard.noPlugins")}
          </p>
          <span className="np-track mt-2">
            <span
              className="np-fill"
              style={{
                width: `${healthRatio}%`,
                backgroundColor:
                  hbSummary.error > 0
                    ? "var(--prio-critical)"
                    : "var(--status-ok)",
              }}
            />
          </span>
        </button>
      </div>
    </aside>
  );
}
