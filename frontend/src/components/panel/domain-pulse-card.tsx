import { TrendingUp } from "lucide-react";
import { useI18n, type TranslationKey } from "@/lib/i18n";
import {
  getDomainVisual,
  type DomainPulseRow,
  type PanelDomain,
} from "./panel-utils";
import { DomainTile } from "./signal-bars";

const DOMAIN_LABELS: Record<PanelDomain, TranslationKey> = {
  ai: "panel.domain.ai",
  investing: "panel.domain.investing",
  deals: "panel.domain.deals",
  other: "panel.domain.other",
};

interface DomainPulseCardProps {
  rows: DomainPulseRow[];
  isLoading: boolean;
  onSelectDomain: (domain: PanelDomain) => void;
}

export function DomainPulseCard({
  rows,
  isLoading,
  onSelectDomain,
}: DomainPulseCardProps) {
  const { t } = useI18n();
  const maxCount = Math.max(1, ...rows.map((r) => r.count));

  return (
    <section className="np-glass p-5">
      <header className="mb-3 flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2.5">
          <span className="np-icon-chip">
            <TrendingUp className="h-4 w-4" />
          </span>
          <div className="min-w-0">
            <h2 className="truncate text-[15px] font-semibold text-[var(--glass-ink)]">
              {t("overview.domainPulse.title")}
            </h2>
            <p className="truncate text-[11.5px] text-[var(--glass-faint)]">
              {t("overview.domainPulse.subtitle")}
            </p>
          </div>
        </div>
      </header>

      {isLoading ? (
        <div className="space-y-2.5">
          {[1, 2, 3, 4].map((idx) => (
            <div
              key={idx}
              className="h-12 animate-pulse rounded-xl bg-[var(--glass-soft)]"
            />
          ))}
        </div>
      ) : (
        <ul className="space-y-1">
          {rows.map((row, idx) => {
            const visual = getDomainVisual(row.domain);
            const widthPct =
              row.avgScore !== null
                ? Math.max(8, Math.min(100, row.avgScore))
                : row.count > 0
                  ? Math.max(12, Math.round((row.count / maxCount) * 100))
                  : 0;

            return (
              <li key={row.domain}>
                <button
                  type="button"
                  onClick={() => onSelectDomain(row.domain)}
                  className="-mx-2 flex w-[calc(100%+1rem)] items-center gap-3 rounded-xl px-2 py-2 text-left transition-colors hover:bg-[var(--glass-soft)]"
                >
                  <DomainTile domain={row.domain} size="sm" />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-baseline justify-between gap-2">
                      <span className="truncate text-[13px] font-semibold text-[var(--glass-ink)]">
                        {t(DOMAIN_LABELS[row.domain])}
                      </span>
                      <span className="shrink-0 text-[11px] tabular-nums text-[var(--glass-faint)]">
                        {t("overview.domainPulse.counts", {
                          unread: row.unread,
                          total: row.count,
                        })}
                      </span>
                    </div>
                    <span className="np-track mt-1.5">
                      <span
                        className="np-fill"
                        style={{
                          width: `${widthPct}%`,
                          background: visual.gradient,
                          animationDelay: `${idx * 70}ms`,
                        }}
                      />
                    </span>
                  </div>
                  <span className="w-8 shrink-0 text-right text-[14px] font-bold tabular-nums text-[var(--glass-ink)]">
                    {row.avgScore !== null ? row.avgScore : "—"}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
