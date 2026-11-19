import { ArrowRight, Radio } from "lucide-react";
import type { PluginHeartbeat } from "@/lib/api";
import { useI18n, type TranslationKey } from "@/lib/i18n";
import { formatRelativeIsoTime, HEARTBEAT_COLORS } from "./panel-utils";

const STATUS_LABELS: Record<PluginHeartbeat["status"], TranslationKey> = {
  ok: "sources.status.ok",
  degraded: "sources.status.degraded",
  error: "sources.status.error",
};

interface SourceHealthCardProps {
  heartbeats: PluginHeartbeat[];
  isLoading: boolean;
  isError: boolean;
  onViewSources: () => void;
}

export function SourceHealthCard({
  heartbeats,
  isLoading,
  isError,
  onViewSources,
}: SourceHealthCardProps) {
  const { t, locale } = useI18n();

  return (
    <section className="np-glass p-5">
      <header className="mb-3 flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2.5">
          <span className="np-icon-chip">
            <Radio className="h-4 w-4" />
          </span>
          <div className="min-w-0">
            <h2 className="truncate text-[15px] font-semibold text-[var(--glass-ink)]">
              {t("overview.sourceHealth.title")}
            </h2>
            <p className="truncate text-[11.5px] text-[var(--glass-faint)]">
              {t("overview.sourceHealth.subtitle")}
            </p>
          </div>
        </div>
        <button
          type="button"
          onClick={onViewSources}
          className="inline-flex shrink-0 items-center gap-1 rounded-xl px-2 py-1 text-xs font-semibold text-[var(--glass-acc)] transition-colors hover:bg-[var(--glass-soft)]"
        >
          {t("overview.sourceHealth.details")}
          <ArrowRight className="h-3.5 w-3.5" />
        </button>
      </header>

      {isLoading ? (
        <div className="space-y-2.5">
          {[1, 2, 3].map((idx) => (
            <div
              key={idx}
              className="h-10 animate-pulse rounded-xl bg-[var(--glass-soft)]"
            />
          ))}
        </div>
      ) : isError ? (
        <p className="rounded-xl bg-[var(--glass-soft)] px-3 py-4 text-center text-xs text-[var(--glass-sub)]">
          {t("sources.loadFailed")}
        </p>
      ) : heartbeats.length === 0 ? (
        <div className="rounded-xl border border-dashed border-[var(--glass-inner-line)] px-3 py-5 text-center">
          <p className="text-xs font-medium text-[var(--glass-sub)]">
            {t("sources.emptyTitle")}
          </p>
          <p className="mt-1 text-[11px] text-[var(--glass-faint)]">
            {t("sources.emptyHint")}
          </p>
        </div>
      ) : (
        <ul className="divide-y divide-[var(--glass-inner-line)]">
          {heartbeats.slice(0, 5).map((hb) => {
            const tone = HEARTBEAT_COLORS[hb.status];
            return (
              <li key={hb.source_id} className="py-2.5 first:pt-0.5 last:pb-0.5">
                <div className="flex items-center gap-2.5">
                  <span
                    className="h-2 w-2 shrink-0 rounded-full"
                    style={{ backgroundColor: tone }}
                    aria-label={t(STATUS_LABELS[hb.status])}
                  />
                  <span className="min-w-0 flex-1 truncate text-[13px] font-medium text-[var(--glass-ink)]">
                    {hb.source_id}
                  </span>
                  <span className="shrink-0 rounded-full bg-[var(--glass-soft)] px-2 py-0.5 text-[11px] font-medium tabular-nums text-[var(--glass-sub)]">
                    {t("sources.fetchedBadge", { count: hb.fetched_count })}
                  </span>
                  <span
                    className="shrink-0 text-[11px] tabular-nums"
                    style={{
                      color: hb.status === "ok" ? "var(--glass-faint)" : tone,
                    }}
                  >
                    {formatRelativeIsoTime(hb.timestamp || hb.updated_at, locale)}
                  </span>
                </div>
                {hb.error_message && (
                  <p className="mt-1 line-clamp-1 pl-4 text-[11px] text-[var(--prio-critical)]">
                    {hb.error_message}
                  </p>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
