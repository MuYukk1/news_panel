import { BookmarkCheck, Inbox, ShieldAlert, Sparkles } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";

interface KpiStripProps {
  unreadCount: number | undefined;
  keptCount: number | undefined;
  filteredCount: number | undefined;
  starredCount: number;
  isLoading: boolean;
  onClickUnread: () => void;
  onClickKept: () => void;
  onClickFiltered: () => void;
  onClickStarred: () => void;
}

export function KpiStrip({
  unreadCount,
  keptCount,
  filteredCount,
  starredCount,
  isLoading,
  onClickUnread,
  onClickKept,
  onClickFiltered,
  onClickStarred,
}: KpiStripProps) {
  const { t } = useI18n();

  const cards = [
    {
      key: "unread",
      label: t("overview.kpi.unread"),
      hint: t("overview.kpi.unreadHint"),
      value: unreadCount,
      icon: Inbox,
      tone: "var(--glass-acc)",
      onClick: onClickUnread,
    },
    {
      key: "kept",
      label: t("overview.kpi.kept"),
      hint: t("overview.kpi.keptHint"),
      value: keptCount,
      icon: Sparkles,
      tone: "var(--status-ok)",
      onClick: onClickKept,
    },
    {
      key: "filtered",
      label: t("overview.kpi.filtered"),
      hint: t("overview.kpi.filteredHint"),
      value: filteredCount,
      icon: ShieldAlert,
      tone: "var(--prio-critical)",
      onClick: onClickFiltered,
    },
    {
      key: "starred",
      label: t("overview.kpi.starred"),
      hint: t("overview.kpi.starredHint"),
      value: starredCount,
      icon: BookmarkCheck,
      tone: "var(--prio-high)",
      onClick: onClickStarred,
    },
  ];

  return (
    <section
      aria-label={t("overview.kpi.sectionLabel")}
      className="grid grid-cols-2 gap-3.5 lg:grid-cols-4"
    >
      {cards.map((card) => {
        const Icon = card.icon;
        const showSkeleton = isLoading && card.value === undefined;
        return (
          <button
            key={card.key}
            type="button"
            onClick={card.onClick}
            className={cn(
              "np-glass np-card-interactive group flex flex-col justify-between p-4 text-left outline-none focus-visible:ring-2 focus-visible:ring-[var(--glass-acc)]",
            )}
          >
            <div className="flex items-center justify-between gap-2">
              <span className="truncate text-xs font-medium text-[var(--glass-sub)]">
                {card.label}
              </span>
              <span
                className="grid h-8 w-8 shrink-0 place-items-center rounded-xl transition-transform group-hover:scale-105"
                style={{
                  color: card.tone,
                  backgroundColor: `color-mix(in srgb, ${card.tone} 13%, transparent)`,
                }}
              >
                <Icon className="h-4 w-4" />
              </span>
            </div>
            <div className="mt-2.5 flex items-baseline justify-between gap-2">
              {showSkeleton ? (
                <span className="h-7 w-14 animate-pulse rounded-lg bg-[var(--glass-soft)]" />
              ) : (
                <span className="text-[26px] font-bold leading-none tracking-tight tabular-nums text-[var(--glass-ink)]">
                  {card.value ?? 0}
                </span>
              )}
              <span className="hidden truncate text-[11px] text-[var(--glass-faint)] sm:inline">
                {card.hint}
              </span>
            </div>
          </button>
        );
      })}
    </section>
  );
}
