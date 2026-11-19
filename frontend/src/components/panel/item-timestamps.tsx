import { Clock, Download } from "lucide-react";
import type { Item } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { cn, formatDate } from "@/lib/utils";
import { formatExactDateTime, resolveItemTimestamps } from "./panel-utils";

interface ItemTimestampsProps {
  item: Pick<Item, "pub_date" | "published_at" | "created_at">;
  showFetched?: boolean;
  variant?: "compact" | "detailed";
  className?: string;
}

export function ItemTimestamps({
  item,
  showFetched = true,
  variant = "compact",
  className,
}: ItemTimestampsProps) {
  const { locale, t } = useI18n();
  const { publishedUnix, fetchedUnix } = resolveItemTimestamps(item);
  const canShowFetched = showFetched && fetchedUnix !== null;

  if (publishedUnix === null && !canShowFetched) {
    return null;
  }

  const publishedIso =
    publishedUnix !== null ? new Date(publishedUnix * 1000).toISOString() : undefined;
  const fetchedIso =
    canShowFetched && fetchedUnix !== null
      ? new Date(fetchedUnix * 1000).toISOString()
      : undefined;

  const exactPublished = formatExactDateTime(publishedUnix, locale);
  const exactFetched = formatExactDateTime(fetchedUnix, locale);

  if (variant === "detailed") {
    return (
      <span
        className={cn(
          "inline-flex flex-wrap items-center gap-1.5 tabular-nums",
          className,
        )}
      >
        {publishedUnix !== null && (
          <time
            dateTime={publishedIso}
            title={t("panel.time.publishedTitle", { time: exactPublished })}
            className="inline-flex items-center gap-1 rounded-lg bg-[var(--glass-soft)] px-2 py-0.5 text-[11.5px] text-[var(--glass-sub)]"
          >
            <Clock className="h-3 w-3 shrink-0 text-[var(--glass-faint)]" />
            <span>
              {t("panel.time.published")} {formatDate(publishedUnix)}
            </span>
            <span className="text-[var(--glass-faint)]">· {exactPublished}</span>
          </time>
        )}
        {canShowFetched && fetchedUnix !== null && (
          <time
            dateTime={fetchedIso}
            title={t("panel.time.fetchedTitle", { time: exactFetched })}
            className="inline-flex items-center gap-1 rounded-lg bg-[var(--glass-soft)] px-2 py-0.5 text-[11.5px] text-[var(--glass-sub)]"
          >
            <Download className="h-3 w-3 shrink-0 text-[var(--glass-faint)]" />
            <span>
              {t("panel.time.fetched")} {formatDate(fetchedUnix)}
            </span>
            <span className="text-[var(--glass-faint)]">· {exactFetched}</span>
          </time>
        )}
      </span>
    );
  }

  return (
    <span
      className={cn(
        "inline-flex flex-wrap items-center gap-x-1.5 tabular-nums",
        className,
      )}
    >
      {publishedUnix !== null && (
        <time
          dateTime={publishedIso}
          title={t("panel.time.publishedTitle", { time: exactPublished })}
        >
          {t("panel.time.published")} {formatDate(publishedUnix)}
        </time>
      )}
      {publishedUnix !== null && canShowFetched && (
        <span aria-hidden="true">·</span>
      )}
      {canShowFetched && fetchedUnix !== null && (
        <time
          dateTime={fetchedIso}
          title={t("panel.time.fetchedTitle", { time: exactFetched })}
        >
          {t("panel.time.fetched")} {formatDate(fetchedUnix)}
        </time>
      )}
    </span>
  );
}
