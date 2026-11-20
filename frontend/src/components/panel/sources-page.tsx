import { useCallback, useMemo, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  Pause,
  Radio,
  RefreshCw,
  Rss,
} from "lucide-react";
import { toast } from "sonner";
import { AppLayout } from "@/components/layout/app-layout";
import { SidebarTrigger } from "@/components/layout/sidebar-trigger";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { PluginHeartbeat } from "@/lib/api";
import { getFeedErrorPreview } from "@/lib/feed-error";
import { useI18n, type TranslationKey } from "@/lib/i18n";
import { cn, formatDate } from "@/lib/utils";
import { useFeedLookup } from "@/queries/feeds";
import { usePluginHeartbeats, useUpdatePlugin } from "@/queries/plugins";
import { GlassSwitch } from "./glass-switch";
import {
  formatRelativeIsoTime,
  HEARTBEAT_COLORS,
  isPluginSourceFeed,
  summarizeHeartbeats,
} from "./panel-utils";
import { ThemeToggleButton } from "./theme-toggle-button";

const STATUS_LABELS: Record<PluginHeartbeat["status"], TranslationKey> = {
  ok: "sources.status.ok",
  degraded: "sources.status.degraded",
  error: "sources.status.error",
};

function formatExactIso(iso: string | null | undefined, locale: string): string {
  if (!iso) return "—";
  const parsed = Date.parse(iso);
  if (Number.isNaN(parsed)) return iso;
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(new Date(parsed));
}

export function SourcesPage() {
  const { t, locale } = useI18n();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const heartbeatsQuery = usePluginHeartbeats();
  const updatePluginMutation = useUpdatePlugin();
  const { feeds, isLoading: feedsLoading } = useFeedLookup();
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [pendingPluginIds, setPendingPluginIds] = useState<Set<string>>(
    new Set(),
  );

  const heartbeats = useMemo(
    () => heartbeatsQuery.data ?? [],
    [heartbeatsQuery.data],
  );
  const summary = useMemo(() => summarizeHeartbeats(heartbeats), [heartbeats]);

  const rssFeeds = useMemo(
    () => feeds.filter((f) => !isPluginSourceFeed(f.link)),
    [feeds],
  );
  const rssStats = useMemo(() => {
    const errorFeeds = rssFeeds.filter((f) => Boolean(f.fetch_state.last_error));
    const pausedFeeds = rssFeeds.filter((f) => f.suspended);
    const activeFeeds = rssFeeds.filter(
      (f) => !f.suspended && !f.fetch_state.last_error,
    );
    return {
      total: rssFeeds.length,
      active: activeFeeds.length,
      paused: pausedFeeds.length,
      error: errorFeeds.length,
      errorFeeds,
      pausedFeeds,
    };
  }, [rssFeeds]);

  const highlightedFeeds = useMemo(() => {
    return [...rssFeeds]
      .sort((a, b) => {
        const aErr = a.fetch_state.last_error ? 2 : a.suspended ? 1 : 0;
        const bErr = b.fetch_state.last_error ? 2 : b.suspended ? 1 : 0;
        if (bErr !== aErr) return bErr - aErr;
        return b.fetch_state.last_checked_at - a.fetch_state.last_checked_at;
      })
      .slice(0, 12);
  }, [rssFeeds]);

  const handleRefresh = useCallback(async () => {
    setIsRefreshing(true);
    try {
      await queryClient.invalidateQueries();
    } finally {
      window.setTimeout(() => setIsRefreshing(false), 320);
    }
  }, [queryClient]);

  const handleTogglePlugin = useCallback(
    async (sourceId: string, enabled: boolean) => {
      setPendingPluginIds((prev) => new Set(prev).add(sourceId));
      try {
        await updatePluginMutation.mutateAsync({ sourceId, enabled });
        toast.success(
          enabled
            ? t("sources.toast.enabled", { source: sourceId })
            : t("sources.toast.paused", { source: sourceId }),
        );
      } catch {
        toast.error(t("sources.toast.toggleFailed"));
      } finally {
        setPendingPluginIds((prev) => {
          const next = new Set(prev);
          next.delete(sourceId);
          return next;
        });
      }
    },
    [t, updatePluginMutation],
  );

  return (
    <AppLayout>
      <div className="flex h-full min-h-0 flex-col overflow-hidden">
        <header className="np-header-glass flex h-[60px] shrink-0 items-center justify-between gap-3 px-4 sm:px-6">
          <div className="flex min-w-0 items-center gap-2.5">
            <SidebarTrigger />
            <div className="min-w-0">
              <h1 className="truncate text-[18px] font-bold tracking-tight text-[var(--glass-ink)]">
                {t("sources.header")}
              </h1>
              <p className="hidden truncate text-xs text-[var(--glass-sub)] sm:block">
                {t("sources.subtitle")}
              </p>
            </div>
          </div>

          <div className="flex shrink-0 items-center gap-2">
            <button
              type="button"
              onClick={() => void navigate({ to: "/feeds" })}
              className="inline-flex h-9 items-center gap-1.5 rounded-xl border border-[var(--glass-border)] bg-[var(--glass-panel)] px-3 text-xs font-semibold text-[var(--glass-ink)] transition-colors hover:bg-[var(--glass-soft-hover)]"
            >
              <Rss className="h-3.5 w-3.5 text-[var(--glass-acc)]" />
              <span>{t("sidebar.manageFeeds")}</span>
            </button>
            <button
              type="button"
              onClick={() => void handleRefresh()}
              aria-label={t("common.refresh")}
              title={t("common.refresh")}
              className="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-[var(--glass-border)] bg-[var(--glass-panel)] text-[var(--glass-sub)] transition-colors hover:bg-[var(--glass-soft-hover)] hover:text-[var(--glass-ink)]"
            >
              <RefreshCw
                className={cn("h-4 w-4", isRefreshing && "animate-spin")}
              />
            </button>
            <ThemeToggleButton />
          </div>
        </header>

        <ScrollArea className="min-h-0 flex-1">
          <div className="mx-auto flex w-full max-w-[1440px] flex-col gap-6 p-4 sm:p-6">
            {/* Top Section: External Plugin Heartbeats */}
            <section className="space-y-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-center gap-2.5">
                  <span className="np-icon-chip">
                    <Radio className="h-4 w-4" />
                  </span>
                  <div>
                    <h2 className="text-[16px] font-semibold text-[var(--glass-ink)]">
                      {t("sources.plugins.title")}
                    </h2>
                    <p className="text-xs text-[var(--glass-faint)]">
                      {t("sources.plugins.summary", {
                        total: summary.total,
                        ok: summary.ok,
                        degraded: summary.degraded,
                        error: summary.error,
                      })}
                    </p>
                  </div>
                </div>

                <div className="flex flex-wrap items-center gap-2 text-xs font-semibold">
                  <span
                    className="rounded-full px-2.5 py-1 tabular-nums"
                    style={{
                      color: "var(--status-ok)",
                      backgroundColor:
                        "color-mix(in srgb, var(--status-ok) 14%, transparent)",
                    }}
                  >
                    {t("sources.status.ok")} {summary.ok}
                  </span>
                  <span
                    className="rounded-full px-2.5 py-1 tabular-nums"
                    style={{
                      color: "var(--prio-high)",
                      backgroundColor:
                        "color-mix(in srgb, var(--prio-high) 14%, transparent)",
                    }}
                  >
                    {t("sources.status.degraded")} {summary.degraded}
                  </span>
                  <span
                    className="rounded-full px-2.5 py-1 tabular-nums"
                    style={{
                      color: "var(--prio-critical)",
                      backgroundColor:
                        "color-mix(in srgb, var(--prio-critical) 14%, transparent)",
                    }}
                  >
                    {t("sources.status.error")} {summary.error}
                  </span>
                </div>
              </div>

              {heartbeatsQuery.isLoading ? (
                <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                  {[1, 2, 3].map((idx) => (
                    <div
                      key={idx}
                      className="np-glass h-40 animate-pulse p-5"
                    />
                  ))}
                </div>
              ) : heartbeatsQuery.isError ? (
                <div className="np-glass p-6 text-center text-sm text-[var(--glass-sub)]">
                  {t("sources.loadFailed")}
                </div>
              ) : heartbeats.length === 0 ? (
                <div className="np-glass flex flex-col items-center justify-center p-10 text-center">
                  <Radio className="mb-3 h-7 w-7 text-[var(--glass-faint)]" />
                  <p className="text-sm font-semibold text-[var(--glass-ink)]">
                    {t("sources.emptyTitle")}
                  </p>
                  <p className="mt-1 max-w-md text-xs text-[var(--glass-faint)]">
                    {t("sources.emptyHint")}
                  </p>
                </div>
              ) : (
                <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                  {heartbeats.map((hb) => {
                    const tone = HEARTBEAT_COLORS[hb.status];
                    const isPending = pendingPluginIds.has(hb.source_id);
                    return (
                      <article
                        key={hb.source_id}
                        className={cn(
                          "np-glass np-card-interactive flex flex-col justify-between p-5 transition-opacity",
                          !hb.enabled && "opacity-75",
                        )}
                      >
                        <div>
                          <div className="flex items-start justify-between gap-3">
                            <div className="min-w-0">
                              <div className="flex items-center gap-2">
                                <span
                                  className="h-2.5 w-2.5 shrink-0 rounded-full"
                                  style={{
                                    backgroundColor: hb.enabled
                                      ? tone
                                      : "var(--glass-faint)",
                                  }}
                                />
                                <h3 className="truncate text-[15px] font-semibold text-[var(--glass-ink)]">
                                  {hb.source_id}
                                </h3>
                              </div>
                              <p className="mt-1 text-xs text-[var(--glass-faint)]">
                                {t("sources.pluginVersion", {
                                  version: hb.plugin_version,
                                })}
                              </p>
                            </div>
                            <div className="flex shrink-0 items-center gap-2">
                              {hb.enabled ? (
                                <span
                                  className="shrink-0 rounded-full px-2.5 py-0.5 text-xs font-semibold"
                                  style={{
                                    color: tone,
                                    backgroundColor: `color-mix(in srgb, ${tone} 14%, transparent)`,
                                  }}
                                >
                                  {t(STATUS_LABELS[hb.status])}
                                </span>
                              ) : (
                                <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-[var(--glass-soft)] px-2.5 py-0.5 text-xs font-semibold text-[var(--glass-sub)]">
                                  <Pause className="h-3 w-3" />
                                  {t("sources.status.paused")}
                                </span>
                              )}
                              <GlassSwitch
                                size="sm"
                                checked={hb.enabled}
                                loading={isPending}
                                aria-label={t("sources.switch.aria", {
                                  source: hb.source_id,
                                })}
                                title={
                                  hb.enabled
                                    ? t("sources.switch.pause")
                                    : t("sources.switch.resume")
                                }
                                onCheckedChange={(checked) => {
                                  void handleTogglePlugin(
                                    hb.source_id,
                                    checked,
                                  );
                                }}
                              />
                            </div>
                          </div>

                          <dl className="mt-4 grid grid-cols-2 gap-3 rounded-2xl bg-[var(--glass-soft)] p-3 text-xs">
                            <div>
                              <dt className="text-[var(--glass-faint)]">
                                {t("sources.fetchedCount")}
                              </dt>
                              <dd className="mt-0.5 text-sm font-bold tabular-nums text-[var(--glass-ink)]">
                                {hb.fetched_count}
                              </dd>
                            </div>
                            <div>
                              <dt className="text-[var(--glass-faint)]">
                                {t("sources.lastHeartbeat")}
                              </dt>
                              <dd className="mt-0.5 text-sm font-semibold tabular-nums text-[var(--glass-ink)]">
                                {formatRelativeIsoTime(hb.timestamp, locale)}
                              </dd>
                            </div>
                            <div className="col-span-2 flex items-center justify-between border-t border-[var(--glass-inner-line)] pt-2 text-[11px] text-[var(--glass-sub)]">
                              <span>{t("sources.reportedAt")}</span>
                              <span className="tabular-nums">
                                {formatExactIso(hb.timestamp, locale)}
                              </span>
                            </div>
                            <div className="col-span-2 flex items-center justify-between text-[11px] text-[var(--glass-sub)]">
                              <span>{t("sources.updatedAt")}</span>
                              <span className="tabular-nums">
                                {formatExactIso(hb.updated_at, locale)}
                              </span>
                            </div>
                          </dl>
                        </div>

                        {hb.error_message && (
                          <div
                            role="alert"
                            className="mt-3 flex items-start gap-2 rounded-xl border border-[var(--prio-critical)]/30 bg-[var(--prio-critical)]/10 p-3 text-xs text-[var(--prio-critical)]"
                          >
                            <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                            <span className="break-words">
                              {hb.error_message}
                            </span>
                          </div>
                        )}
                      </article>
                    );
                  })}
                </div>
              )}
            </section>

            {/* Bottom Section: Built-in RSS Feeds Overview */}
            <section className="np-glass p-5">
              <header className="mb-4 flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-center gap-2.5">
                  <span className="np-icon-chip">
                    <Rss className="h-4 w-4" />
                  </span>
                  <div>
                    <h2 className="text-[16px] font-semibold text-[var(--glass-ink)]">
                      {t("sources.rss.title")}
                    </h2>
                    <p className="text-xs text-[var(--glass-faint)]">
                      {t("sources.rss.subtitle")}
                    </p>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => void navigate({ to: "/feeds" })}
                  className="inline-flex items-center gap-1 rounded-xl px-2.5 py-1.5 text-xs font-semibold text-[var(--glass-acc)] transition-colors hover:bg-[var(--glass-soft)]"
                >
                  {t("sources.rss.manageLink")}
                  <ArrowRight className="h-3.5 w-3.5" />
                </button>
              </header>

              {/* RSS KPI Strip */}
              <div className="mb-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
                <div className="rounded-2xl bg-[var(--glass-soft)] p-3">
                  <p className="text-[11px] text-[var(--glass-faint)]">
                    {t("sources.rss.total")}
                  </p>
                  <p className="mt-1 text-xl font-bold tabular-nums text-[var(--glass-ink)]">
                    {rssStats.total}
                  </p>
                </div>
                <div className="rounded-2xl bg-[var(--glass-soft)] p-3">
                  <p className="text-[11px] text-[var(--glass-faint)]">
                    {t("sources.rss.active")}
                  </p>
                  <p
                    className="mt-1 text-xl font-bold tabular-nums"
                    style={{ color: "var(--status-ok)" }}
                  >
                    {rssStats.active}
                  </p>
                </div>
                <div className="rounded-2xl bg-[var(--glass-soft)] p-3">
                  <p className="text-[11px] text-[var(--glass-faint)]">
                    {t("sources.rss.paused")}
                  </p>
                  <p
                    className="mt-1 text-xl font-bold tabular-nums"
                    style={{ color: "var(--prio-high)" }}
                  >
                    {rssStats.paused}
                  </p>
                </div>
                <div className="rounded-2xl bg-[var(--glass-soft)] p-3">
                  <p className="text-[11px] text-[var(--glass-faint)]">
                    {t("sources.rss.error")}
                  </p>
                  <p
                    className="mt-1 text-xl font-bold tabular-nums"
                    style={{ color: "var(--prio-critical)" }}
                  >
                    {rssStats.error}
                  </p>
                </div>
              </div>

              {feedsLoading ? (
                <div className="space-y-2">
                  {[1, 2, 3].map((idx) => (
                    <div
                      key={idx}
                      className="h-11 animate-pulse rounded-xl bg-[var(--glass-soft)]"
                    />
                  ))}
                </div>
              ) : highlightedFeeds.length === 0 ? (
                <p className="py-6 text-center text-xs text-[var(--glass-faint)]">
                  {t("sources.rss.empty")}
                </p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full min-w-[540px] border-collapse text-left text-xs">
                    <thead>
                      <tr className="border-b border-[var(--glass-inner-line)] text-[11px] font-semibold text-[var(--glass-faint)]">
                        <th className="py-2 pr-3">{t("sources.rss.colFeed")}</th>
                        <th className="py-2 pr-3">{t("sources.rss.colStatus")}</th>
                        <th className="py-2 pr-3">{t("sources.rss.colItems")}</th>
                        <th className="py-2 pr-3">{t("sources.rss.colChecked")}</th>
                        <th className="py-2">{t("sources.rss.colError")}</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[var(--glass-inner-line)]">
                      {highlightedFeeds.map((feed) => {
                        const hasError = Boolean(feed.fetch_state.last_error);
                        return (
                          <tr
                            key={feed.id}
                            className="transition-colors hover:bg-[var(--glass-soft)]"
                          >
                            <td className="max-w-[220px] truncate py-2.5 pr-3 font-medium text-[var(--glass-ink)]">
                              {feed.name}
                            </td>
                            <td className="py-2.5 pr-3">
                              {hasError ? (
                                <span className="inline-flex items-center gap-1 font-semibold text-[var(--prio-critical)]">
                                  <AlertTriangle className="h-3.5 w-3.5" />
                                  {t("feeds.status.error")}
                                </span>
                              ) : feed.suspended ? (
                                <span className="inline-flex items-center gap-1 font-semibold text-[var(--prio-high)]">
                                  <Pause className="h-3.5 w-3.5" />
                                  {t("feeds.status.paused")}
                                </span>
                              ) : (
                                <span className="inline-flex items-center gap-1 font-semibold text-[var(--status-ok)]">
                                  <CheckCircle2 className="h-3.5 w-3.5" />
                                  {t("sources.status.ok")}
                                </span>
                              )}
                            </td>
                            <td className="py-2.5 pr-3 tabular-nums text-[var(--glass-sub)]">
                              {feed.item_count}
                            </td>
                            <td className="py-2.5 pr-3 tabular-nums text-[var(--glass-sub)]">
                              {feed.fetch_state.last_checked_at > 0
                                ? formatDate(feed.fetch_state.last_checked_at)
                                : t("common.unknown")}
                            </td>
                            <td className="max-w-[260px] truncate py-2.5 text-[var(--prio-critical)]">
                              {feed.fetch_state.last_error
                                ? getFeedErrorPreview(
                                    feed.fetch_state.last_error,
                                  )
                                : "—"}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </section>
          </div>
        </ScrollArea>
      </div>
    </AppLayout>
  );
}
