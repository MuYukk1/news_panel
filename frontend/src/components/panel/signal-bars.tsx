import { CircleDashed, Cpu, Globe, Tag, TrendingUp } from "lucide-react";
import type { Item } from "@/lib/api";
import { useI18n, type TranslationKey } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import {
  getDomainVisual,
  normalizePanelDomain,
  PRIORITY_COLORS,
  type PanelDomain,
} from "./panel-utils";

const DOMAIN_ICONS: Record<PanelDomain, typeof Cpu> = {
  ai: Cpu,
  investing: TrendingUp,
  deals: Tag,
  other: Globe,
};

const DOMAIN_LABEL_KEYS: Record<PanelDomain, TranslationKey> = {
  ai: "panel.domain.ai",
  investing: "panel.domain.investing",
  deals: "panel.domain.deals",
  other: "panel.domain.other",
};

const PRIORITY_LABEL_KEYS: Record<Item["priority"], TranslationKey> = {
  critical: "panel.priority.critical",
  high: "panel.priority.high",
  normal: "panel.priority.normal",
  low: "panel.priority.low",
};

const TRIAGE_LABEL_KEYS: Record<
  Item["triage_status"] | "unscored",
  TranslationKey
> = {
  kept: "panel.triage.kept",
  unscored: "panel.triage.unscored",
  filtered: "panel.triage.filtered",
  pending: "panel.triage.pending",
  failed: "panel.triage.failed",
};

interface DomainTileProps {
  domain?: string | null;
  size?: "xs" | "sm" | "md" | "lg";
  className?: string;
}

export function DomainTile({
  domain,
  size = "md",
  className,
}: DomainTileProps) {
  const key = normalizePanelDomain(domain);
  const visual = getDomainVisual(key);
  const Icon = DOMAIN_ICONS[key];
  const iconSize =
    size === "xs" ? 13 : size === "sm" ? 16 : size === "lg" ? 21 : 19;

  return (
    <span
      aria-hidden="true"
      className={cn(
        "np-tile",
        size === "xs" && "np-tile-xs",
        size === "sm" && "np-tile-sm",
        size === "lg" && "np-tile-lg",
        className,
      )}
      style={{ background: visual.gradient }}
    >
      <Icon size={iconSize} strokeWidth={2} />
    </span>
  );
}

interface DomainPillProps {
  domain?: string | null;
  className?: string;
}

export function DomainPill({ domain, className }: DomainPillProps) {
  const { t } = useI18n();
  const key = normalizePanelDomain(domain);
  const visual = getDomainVisual(key);
  const Icon = DOMAIN_ICONS[key];

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-semibold leading-4",
        className,
      )}
      style={{ color: visual.tone, backgroundColor: visual.badgeBg }}
    >
      <Icon size={11} strokeWidth={2.2} />
      {t(DOMAIN_LABEL_KEYS[key])}
    </span>
  );
}

interface PriorityPillProps {
  priority: Item["priority"];
  score?: number | null;
  className?: string;
}

export function PriorityPill({
  priority,
  score,
  className,
}: PriorityPillProps) {
  const { t } = useI18n();
  const color = PRIORITY_COLORS[priority] ?? PRIORITY_COLORS.normal;
  const label = t(PRIORITY_LABEL_KEYS[priority] ?? "panel.priority.normal");

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-[11px] font-semibold leading-4 tabular-nums",
        className,
      )}
      style={{
        color,
        backgroundColor: `color-mix(in srgb, ${color} 14%, transparent)`,
      }}
    >
      <span>{label}</span>
      {typeof score === "number" && (
        <span className="font-bold">{score}</span>
      )}
    </span>
  );
}

interface TriageStatusPillProps {
  status: Item["triage_status"] | "unscored";
  className?: string;
}

export function TriageStatusPill({ status, className }: TriageStatusPillProps) {
  const { t } = useI18n();
  const color =
    status === "kept"
      ? "var(--status-ok)"
      : status === "filtered"
        ? "var(--prio-critical)"
        : status === "failed"
          ? "var(--prio-high)"
          : "var(--glass-faint)";

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-semibold leading-4",
        className,
      )}
      style={{
        color,
        backgroundColor: `color-mix(in srgb, ${color} 14%, transparent)`,
      }}
    >
      <span
        className="h-1.5 w-1.5 rounded-full"
        style={{ backgroundColor: color }}
      />
      {t(TRIAGE_LABEL_KEYS[status] ?? "panel.triage.pending")}
    </span>
  );
}

export function UnscoredPill({ className }: { className?: string }) {
  const { t } = useI18n();
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-full border border-[var(--glass-inner-line)] bg-[var(--glass-soft)] px-2.5 py-0.5 text-[11px] font-medium leading-4 text-[var(--glass-sub)]",
        className,
      )}
    >
      <CircleDashed className="h-3 w-3 text-[var(--glass-faint)]" />
      <span>{t("panel.score.unscored")}</span>
    </span>
  );
}

interface SignalBarsProps {
  score: number | null;
  priority: Item["priority"];
  showValue?: boolean;
  className?: string;
}

export function SignalBars({
  score,
  priority,
  showValue = false,
  className,
}: SignalBarsProps) {
  const { t } = useI18n();
  const clamped =
    typeof score === "number" && Number.isFinite(score)
      ? Math.max(0, Math.min(100, score))
      : 0;
  const lit = score === null ? 0 : Math.round(clamped / 10);
  const color = PRIORITY_COLORS[priority] ?? PRIORITY_COLORS.normal;

  return (
    <div
      className={cn("inline-flex items-center gap-2", className)}
      role="img"
      aria-label={
        score === null
          ? t("panel.score.unscored")
          : t("panel.score.aria", { score: clamped })
      }
    >
      <span className="flex h-[16px] items-end gap-[2px]">
        {Array.from({ length: 10 }, (_, idx) => (
          <span
            key={idx}
            className="w-[3px] rounded-[1px]"
            style={{
              height: `${4 + idx * 1.25}px`,
              backgroundColor: idx < lit ? color : "var(--glass-bar)",
            }}
          />
        ))}
      </span>
      {showValue && (
        <span
          className="text-xs font-bold tabular-nums"
          style={{ color: score === null ? "var(--glass-faint)" : color }}
        >
          {score === null ? "—" : clamped}
        </span>
      )}
    </div>
  );
}
