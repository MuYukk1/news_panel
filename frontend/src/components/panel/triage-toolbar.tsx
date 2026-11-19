import { useState } from "react";
import {
  CheckCheck,
  RefreshCw,
  RotateCcw,
  Search,
  SlidersHorizontal,
} from "lucide-react";
import { SidebarTrigger } from "@/components/layout/sidebar-trigger";
import type { Item } from "@/lib/api";
import type { ArticleFilter } from "@/lib/article-filter";
import { useI18n, type TranslationKey } from "@/lib/i18n";
import type { TriageSearch, TriageStatusFilter } from "@/lib/triage-search";
import { cn } from "@/lib/utils";
import {
  countActiveAdvancedFilters,
  PANEL_DOMAINS,
  PANEL_PRIORITIES,
  PANEL_TRIAGE_TABS,
  PRIORITY_COLORS,
  type PanelDomain,
  type PanelTriageTab,
} from "./panel-utils";
import { ThemeToggleButton } from "./theme-toggle-button";

const DOMAIN_LABELS: Record<PanelDomain, TranslationKey> = {
  ai: "panel.domain.ai",
  investing: "panel.domain.investing",
  deals: "panel.domain.deals",
  other: "panel.domain.other",
};

const TRIAGE_TAB_LABELS: Record<PanelTriageTab, TranslationKey> = {
  kept: "panel.triage.kept",
  filtered: "panel.triage.filtered",
  pending: "panel.triage.pending",
  all: "panel.triage.all",
};

const PRIORITY_LABELS: Record<Item["priority"], TranslationKey> = {
  critical: "panel.priority.critical",
  high: "panel.priority.high",
  normal: "panel.priority.normal",
  low: "panel.priority.low",
};

const SCORE_PRESETS = [0, 45, 60, 80] as const;

interface TriageToolbarProps {
  title: string;
  articleFilter: ArticleFilter;
  triageSearch: TriageSearch;
  sortBy: "time" | "score";
  unreadCount: number;
  isRefreshing: boolean;
  onChangeArticleFilter: (filter: ArticleFilter) => void;
  onUpdateTriage: (
    patch: Partial<Omit<TriageSearch, "article">>,
    options?: { reset?: boolean; filter?: ArticleFilter },
  ) => void;
  onChangeSortBy: (sort: "time" | "score") => void;
  onOpenSearch: () => void;
  onRefresh: () => void;
  onMarkAllRead: () => void;
}

export function TriageToolbar({
  title,
  articleFilter,
  triageSearch,
  sortBy,
  unreadCount,
  isRefreshing,
  onChangeArticleFilter,
  onUpdateTriage,
  onChangeSortBy,
  onOpenSearch,
  onRefresh,
  onMarkAllRead,
}: TriageToolbarProps) {
  const { t } = useI18n();
  const [advancedOpen, setAdvancedOpen] = useState(
    () => countActiveAdvancedFilters(triageSearch) > 0,
  );

  const isStarredMode = articleFilter === "starred";
  const activeTriageStatus: TriageStatusFilter =
    triageSearch.triage_status ?? (isStarredMode ? "all" : "kept");
  const activeDomain = triageSearch.domain ?? "all";
  const activePriority = triageSearch.priority ?? "all";
  const activeMinScore = triageSearch.min_score ?? 0;
  const activeAdvancedCount =
    countActiveAdvancedFilters(triageSearch) + (sortBy === "score" ? 1 : 0);

  const hasAnyFilter =
    activeDomain !== "all" ||
    activePriority !== "all" ||
    activeMinScore > 0 ||
    activeTriageStatus !== (isStarredMode ? "all" : "kept") ||
    sortBy !== "time";

  const handleReset = () => {
    onChangeSortBy("time");
    onUpdateTriage({}, { reset: true });
  };

  return (
    <div className="shrink-0 space-y-2.5 px-4 pt-3 pb-2 sm:px-6">
      {/* Row 1: Compact Glass Top Control Bar */}
      <div className="np-glass-sm flex flex-wrap items-center justify-between gap-2.5 px-3.5 py-2.5">
        <div className="flex min-w-0 flex-wrap items-center gap-2.5">
          <div className="flex min-w-0 items-center gap-1.5">
            <SidebarTrigger />
            <h2 className="max-w-[200px] truncate text-[16px] font-bold tracking-tight text-[var(--glass-ink)] sm:max-w-[260px]">
              {title}
            </h2>
          </div>

          {/* All / Unread / Starred route switch */}
          <div
            className="np-seg"
            role="group"
            aria-label={t("panel.toolbar.viewScope")}
          >
            {(["all", "unread", "starred"] as const).map((filterKey) => (
              <button
                key={filterKey}
                type="button"
                className="np-seg-btn"
                aria-pressed={articleFilter === filterKey}
                onClick={() => onChangeArticleFilter(filterKey)}
              >
                {t(`article.filter.${filterKey}`)}
              </button>
            ))}
          </div>

          {/* Triage status switch (kept / filtered / pending / all) */}
          <div
            className="np-seg"
            role="group"
            aria-label={t("panel.toolbar.triageStatus")}
          >
            {PANEL_TRIAGE_TABS.map((status) => (
              <button
                key={status}
                type="button"
                className="np-seg-btn"
                aria-pressed={activeTriageStatus === status}
                onClick={() =>
                  onUpdateTriage({
                    triage_status:
                      !isStarredMode && status === "kept"
                        ? undefined
                        : isStarredMode && status === "all"
                          ? undefined
                          : status,
                  })
                }
              >
                {t(TRIAGE_TAB_LABELS[status])}
              </button>
            ))}
          </div>
        </div>

        {/* Right Actions */}
        <div className="flex items-center gap-1.5">
          <button
            type="button"
            onClick={onOpenSearch}
            aria-label={t("common.search")}
            title={t("common.search")}
            className="inline-flex h-8 w-8 items-center justify-center rounded-xl border border-[var(--glass-inner-line)] bg-[var(--glass-panel)] text-[var(--glass-sub)] transition-colors hover:bg-[var(--glass-soft)] hover:text-[var(--glass-ink)]"
          >
            <Search className="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            onClick={onRefresh}
            aria-label={t("common.refresh")}
            title={t("common.refresh")}
            className="inline-flex h-8 w-8 items-center justify-center rounded-xl border border-[var(--glass-inner-line)] bg-[var(--glass-panel)] text-[var(--glass-sub)] transition-colors hover:bg-[var(--glass-soft)] hover:text-[var(--glass-ink)]"
          >
            <RefreshCw
              className={cn("h-3.5 w-3.5", isRefreshing && "animate-spin")}
            />
          </button>
          <button
            type="button"
            aria-expanded={advancedOpen}
            onClick={() => setAdvancedOpen((prev) => !prev)}
            className={cn(
              "inline-flex h-8 items-center gap-1.5 rounded-xl border px-2.5 text-xs font-semibold transition-colors",
              advancedOpen || activeAdvancedCount > 0
                ? "border-[var(--glass-acc)] bg-[var(--glass-soft)] text-[var(--glass-acc)]"
                : "border-[var(--glass-inner-line)] bg-[var(--glass-panel)] text-[var(--glass-sub)] hover:text-[var(--glass-ink)]",
            )}
          >
            <SlidersHorizontal className="h-3.5 w-3.5" />
            <span className="hidden sm:inline">
              {t("panel.toolbar.advancedFilters")}
            </span>
            {activeAdvancedCount > 0 && (
              <span className="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-[var(--glass-acc)] px-1 text-[10px] font-bold text-white dark:text-[#0b0d1f]">
                {activeAdvancedCount}
              </span>
            )}
          </button>
          {!isStarredMode && (
            <button
              type="button"
              onClick={onMarkAllRead}
              disabled={unreadCount === 0}
              aria-label={t("article.list.markAllRead")}
              title={t("article.list.markAllRead")}
              className="inline-flex h-8 items-center gap-1.5 rounded-xl border border-[var(--glass-inner-line)] bg-[var(--glass-panel)] px-2.5 text-xs font-medium text-[var(--glass-sub)] transition-colors hover:bg-[var(--glass-soft)] hover:text-[var(--glass-ink)] disabled:cursor-not-allowed disabled:opacity-45"
            >
              <CheckCheck className="h-3.5 w-3.5" />
              <span className="hidden lg:inline">
                {t("article.list.markAllRead")}
              </span>
            </button>
          )}
          <ThemeToggleButton className="h-8 w-8 rounded-xl" />
        </div>
      </div>

      {/* Row 2: Domain Pill Strip */}
      <div className="flex items-center gap-2 overflow-x-auto pb-0.5 [scrollbar-width:none]">
        <button
          type="button"
          className="np-pill-btn shrink-0"
          aria-pressed={activeDomain === "all"}
          onClick={() => onUpdateTriage({ domain: undefined })}
        >
          {t("panel.domain.all")}
        </button>
        {PANEL_DOMAINS.map((domain) => (
          <button
            key={domain}
            type="button"
            className="np-pill-btn shrink-0"
            aria-pressed={activeDomain === domain}
            onClick={() =>
              onUpdateTriage({
                domain: activeDomain === domain ? undefined : domain,
              })
            }
          >
            {t(DOMAIN_LABELS[domain])}
          </button>
        ))}

        {hasAnyFilter && (
          <button
            type="button"
            onClick={handleReset}
            className="ml-auto inline-flex h-8 shrink-0 items-center gap-1 rounded-full px-2.5 text-xs font-semibold text-[var(--glass-acc)] transition-colors hover:bg-[var(--glass-soft)]"
          >
            <RotateCcw className="h-3 w-3" />
            {t("panel.toolbar.resetFilters")}
          </button>
        )}
      </div>

      {/* Collapsible Advanced Filters Panel */}
      <div className="np-orig-collapse" data-open={advancedOpen}>
        <div>
          <div className="np-glass-sm mt-0.5 flex flex-wrap items-center gap-x-6 gap-y-3 px-4 py-3">
            {/* Priority Filter */}
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="mr-1 text-xs font-semibold text-[var(--glass-sub)]">
                {t("panel.toolbar.priority")}
              </span>
              <button
                type="button"
                className="np-pill-btn !h-7 !px-2.5 !text-xs"
                aria-pressed={activePriority === "all"}
                onClick={() => onUpdateTriage({ priority: undefined })}
              >
                {t("panel.priority.all")}
              </button>
              {PANEL_PRIORITIES.map((priority) => (
                <button
                  key={priority}
                  type="button"
                  className="np-pill-btn !h-7 !px-2.5 !text-xs"
                  aria-pressed={activePriority === priority}
                  onClick={() =>
                    onUpdateTriage({
                      priority:
                        activePriority === priority ? undefined : priority,
                    })
                  }
                >
                  <span
                    className="h-2 w-2 rounded-full"
                    style={{ backgroundColor: PRIORITY_COLORS[priority] }}
                  />
                  {t(PRIORITY_LABELS[priority])}
                </button>
              ))}
            </div>

            {/* Min Score Filter */}
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-xs font-semibold text-[var(--glass-sub)]">
                {t("panel.toolbar.minScore")}
              </span>
              <div className="flex items-center gap-1">
                {SCORE_PRESETS.map((preset) => (
                  <button
                    key={preset}
                    type="button"
                    className="np-pill-btn !h-7 !px-2.5 !text-xs tabular-nums"
                    aria-pressed={activeMinScore === preset}
                    onClick={() =>
                      onUpdateTriage({
                        min_score: preset === 0 ? undefined : preset,
                      })
                    }
                  >
                    {preset === 0 ? t("panel.toolbar.scoreAny") : `≥ ${preset}`}
                  </button>
                ))}
              </div>
              <label className="flex items-center gap-2">
                <input
                  type="range"
                  min={0}
                  max={100}
                  step={5}
                  value={activeMinScore}
                  aria-label={t("panel.toolbar.minScoreSlider")}
                  className="np-range w-[110px]"
                  onChange={(e) => {
                    const next = Number(e.target.value);
                    onUpdateTriage({
                      min_score: next === 0 ? undefined : next,
                    });
                  }}
                />
                <span className="w-9 text-xs font-semibold tabular-nums text-[var(--glass-ink)]">
                  {activeMinScore === 0 ? "—" : `≥${activeMinScore}`}
                </span>
              </label>
            </div>

            {/* Client-Side Sort of Loaded Pages */}
            <div className="flex flex-wrap items-center gap-2 lg:ml-auto">
              <span className="text-xs font-semibold text-[var(--glass-sub)]">
                {t("panel.toolbar.sortLoaded")}
              </span>
              <div className="np-seg" role="group">
                <button
                  type="button"
                  className="np-seg-btn !py-1 !text-xs"
                  aria-pressed={sortBy === "time"}
                  onClick={() => onChangeSortBy("time")}
                >
                  {t("panel.toolbar.sortTime")}
                </button>
                <button
                  type="button"
                  className="np-seg-btn !py-1 !text-xs"
                  aria-pressed={sortBy === "score"}
                  onClick={() => onChangeSortBy("score")}
                >
                  {t("panel.toolbar.sortScore")}
                </button>
              </div>
              <span className="text-[11px] text-[var(--glass-faint)]">
                {t("panel.toolbar.sortLoadedHint")}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
