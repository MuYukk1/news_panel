import { useMemo } from "react";
import { CheckSquare, Folder, Search, Square } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  areAllVisibleSelected,
  filterImportCandidates,
  toggleAllVisibleSelection,
  toggleCandidateSelection,
  type ImportCandidateFeed,
} from "@/lib/feed-import";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";

interface FeedMultiSelectListProps {
  candidates: readonly ImportCandidateFeed[];
  selectedLinks: ReadonlySet<string>;
  onChangeSelected: (next: Set<string>) => void;
  searchQuery: string;
  onChangeSearchQuery: (value: string) => void;
  existingLinks?: ReadonlySet<string>;
}

export function FeedMultiSelectList({
  candidates,
  selectedLinks,
  onChangeSelected,
  searchQuery,
  onChangeSearchQuery,
  existingLinks,
}: FeedMultiSelectListProps) {
  const { t } = useI18n();

  const visibleCandidates = useMemo(
    () => filterImportCandidates(candidates, searchQuery),
    [candidates, searchQuery],
  );

  const allVisibleSelected = useMemo(
    () => areAllVisibleSelected(selectedLinks, visibleCandidates),
    [selectedLinks, visibleCandidates],
  );

  return (
    <div className="space-y-3">
      {/* Search + Select All Controls */}
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-[180px] flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={searchQuery}
            onChange={(e) => onChangeSearchQuery(e.target.value)}
            placeholder={t("feed.batch.searchPlaceholder")}
            aria-label={t("feed.batch.searchPlaceholder")}
            className="h-9 pl-8 text-xs"
            autoComplete="off"
          />
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-9 shrink-0 gap-1.5 text-xs"
          disabled={visibleCandidates.length === 0}
          onClick={() =>
            onChangeSelected(
              toggleAllVisibleSelection(selectedLinks, visibleCandidates),
            )
          }
        >
          {allVisibleSelected ? (
            <CheckSquare className="h-3.5 w-3.5 text-[var(--glass-acc)]" />
          ) : (
            <Square className="h-3.5 w-3.5 text-muted-foreground" />
          )}
          {allVisibleSelected
            ? t("feed.batch.deselectAll")
            : t("feed.batch.selectAll")}
        </Button>
      </div>

      {/* Selected Counter */}
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span className="tabular-nums font-medium text-foreground">
          {t("feed.batch.selectedCount", {
            selected: selectedLinks.size,
            total: candidates.length,
          })}
        </span>
        {searchQuery.trim() !== "" && (
          <span className="tabular-nums">
            {t("feed.batch.showingCount", { count: visibleCandidates.length })}
          </span>
        )}
      </div>

      {/* Scrollable Checkbox List */}
      <div className="max-h-[300px] space-y-1.5 overflow-y-auto pr-1">
        {visibleCandidates.length === 0 ? (
          <div className="rounded-lg border border-dashed py-8 text-center text-xs text-muted-foreground">
            {t("feeds.noMatch")}
          </div>
        ) : (
          visibleCandidates.map((feed) => {
            const checked = selectedLinks.has(feed.link);
            const alreadySubscribed = existingLinks?.has(feed.link) ?? false;
            return (
              <label
                key={feed.link}
                className={cn(
                  "flex cursor-pointer items-start gap-2.5 rounded-xl border p-2.5 text-left transition-colors",
                  checked
                    ? "border-[var(--glass-acc)]/45 bg-[var(--glass-soft)]"
                    : "border-border/70 bg-background/60 hover:bg-accent/40",
                )}
              >
                <input
                  type="checkbox"
                  checked={checked}
                  onChange={() =>
                    onChangeSelected(
                      toggleCandidateSelection(selectedLinks, feed.link),
                    )
                  }
                  className="mt-0.5 h-4 w-4 shrink-0 rounded accent-[var(--glass-acc)]"
                />
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <span className="truncate text-[13px] font-medium text-foreground">
                      {feed.name}
                    </span>
                    {feed.groupName && (
                      <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-[var(--glass-soft)] px-2 py-0.5 text-[10.5px] font-medium text-[var(--glass-sub)]">
                        <Folder className="h-2.5 w-2.5" />
                        {feed.groupName}
                      </span>
                    )}
                    {alreadySubscribed && (
                      <span className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-[10.5px] font-medium text-muted-foreground">
                        {t("feed.batch.alreadySubscribed")}
                      </span>
                    )}
                  </div>
                  <p className="mt-0.5 truncate text-[11.5px] text-muted-foreground">
                    {feed.link}
                  </p>
                </div>
              </label>
            );
          })
        )}
      </div>
    </div>
  );
}
