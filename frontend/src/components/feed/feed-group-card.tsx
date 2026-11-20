import { AlertCircle, ChevronDown, ChevronRight, Folder, Pause, Pencil, Plus, Sparkles, Trash2 } from "lucide-react";
import type { Dispatch, SetStateAction } from "react";
import { FeedFavicon } from "@/components/feed/feed-favicon";
import { GlassSwitch } from "@/components/panel/glass-switch";
import { Input } from "@/components/ui/input";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { getFaviconUrl } from "@/lib/api/favicon";
import { getFeedErrorPreview } from "@/lib/feed-error";
import type { TranslationKey } from "@/lib/i18n";
import { cn, formatDate } from "@/lib/utils";
import type { Feed, Group } from "@/lib/api";

interface FeedGroupCardProps {
  group: Group;
  groupFeeds: Feed[];
  isCollapsed: boolean;
  isEditing: boolean;
  editingGroupName: string;
  isMobile: boolean;
  mobileErrorTooltipFeedId: number | null;
  onToggleGroup: (groupId: number) => void;
  onStartEditingGroup: (group: Group) => void;
  onChangeEditingGroupName: (value: string) => void;
  onSaveGroupName: (group: Group) => void;
  onCancelEditingGroup: () => void;
  onOpenAddFeed: () => void;
  onOpenDeleteGroup: (group: Group) => void;
  onOpenEditFeed: (feed: Feed) => void;
  pendingFeedIds: ReadonlySet<number>;
  onToggleFeedEnabled: (feed: Feed, enabled: boolean) => void;
  onToggleFeedAIEnabled: (feed: Feed, aiEnabled: boolean) => void;
  onChangeMobileErrorTooltipFeedId: Dispatch<SetStateAction<number | null>>;
  t: (key: TranslationKey, params?: Record<string, string | number>) => string;
}

function getDomain(url: string) {
  try {
    return new URL(url).hostname;
  } catch {
    return url;
  }
}

export function FeedGroupCard({
  group,
  groupFeeds,
  isCollapsed,
  isEditing,
  editingGroupName,
  isMobile,
  mobileErrorTooltipFeedId,
  onToggleGroup,
  onStartEditingGroup,
  onChangeEditingGroupName,
  onSaveGroupName,
  onCancelEditingGroup,
  onOpenAddFeed,
  onOpenDeleteGroup,
  onOpenEditFeed,
  pendingFeedIds,
  onToggleFeedEnabled,
  onToggleFeedAIEnabled,
  onChangeMobileErrorTooltipFeedId,
  t,
}: FeedGroupCardProps) {
  return (
    <div className="np-glass-sm overflow-hidden">
      <div
        className={cn(
          "group/header flex items-center justify-between bg-[var(--glass-soft)] px-3.5 py-2.5",
          isCollapsed ? "rounded-2xl" : "rounded-t-2xl",
        )}
      >
        {isEditing ? (
          <div className="flex min-w-0 flex-1 items-center gap-2 text-left">
            {isCollapsed ? (
              <ChevronRight className="h-4 w-4 text-muted-foreground" />
            ) : (
              <ChevronDown className="h-4 w-4" />
            )}
            <Folder
              className={cn(
                "h-4 w-4",
                isCollapsed && "text-muted-foreground",
              )}
            />
            <Input
              value={editingGroupName}
              onChange={(e) => onChangeEditingGroupName(e.target.value)}
              onBlur={() => onSaveGroupName(group)}
              onKeyDown={(e) => {
                if (e.key === "Enter") onSaveGroupName(group);
                if (e.key === "Escape") onCancelEditingGroup();
              }}
              className="h-7 w-40 px-2 text-sm"
              aria-label={t("group.add.placeholder")}
              autoFocus={!isMobile}
            />
            <span
              className={cn(
                "rounded-full bg-muted px-2 py-0.5 text-[11px]",
                isCollapsed
                  ? "font-medium text-muted-foreground"
                  : "font-semibold text-muted-foreground",
              )}
            >
              {groupFeeds.length}
            </span>
          </div>
        ) : (
          <button
            type="button"
            onClick={() => onToggleGroup(group.id)}
            className="flex min-w-0 flex-1 items-center gap-2 text-left"
            aria-expanded={!isCollapsed}
            aria-label={`Toggle group ${group.name}`}
          >
            {isCollapsed ? (
              <ChevronRight className="h-4 w-4 text-muted-foreground" />
            ) : (
              <ChevronDown className="h-4 w-4" />
            )}
            <Folder
              className={cn(
                "h-4 w-4",
                isCollapsed && "text-muted-foreground",
              )}
            />
            <span
              className={cn(
                "text-sm",
                isCollapsed ? "font-medium" : "font-semibold",
              )}
            >
              {group.name}
            </span>
            <span
              className={cn(
                "rounded-full bg-muted px-2 py-0.5 text-[11px]",
                isCollapsed
                  ? "font-medium text-muted-foreground"
                  : "font-semibold text-muted-foreground",
              )}
            >
              {groupFeeds.length}
            </span>
          </button>
        )}
        <div className="flex items-center gap-1 sm:opacity-0 sm:transition-opacity sm:group-hover/header:opacity-100">
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onOpenAddFeed();
            }}
            className="rounded p-1 hover:bg-accent"
            aria-label={t("feed.add.button")}
          >
            <Plus className="h-3.5 w-3.5 text-muted-foreground" />
          </button>
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onStartEditingGroup(group);
            }}
            className="rounded p-1 hover:bg-accent"
            aria-label={t("feeds.renameGroup")}
          >
            <Pencil className="h-3.5 w-3.5 text-muted-foreground" />
          </button>
          {group.id !== 1 && (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                onOpenDeleteGroup(group);
              }}
              className="rounded p-1 hover:bg-accent"
              aria-label={t("feeds.deleteGroup.title")}
            >
              <Trash2 className="h-3.5 w-3.5 text-muted-foreground" />
            </button>
          )}
        </div>
      </div>

      {!isCollapsed && groupFeeds.length > 0 && (
        <div>
          {groupFeeds.map((feed, index) => {
            const isPending = pendingFeedIds.has(feed.id);
            return (
              <div
                key={feed.id}
                className={cn(
                  "flex items-center justify-between gap-2 py-2.5 pl-8 pr-3.5 transition-all hover:bg-accent/30 sm:pl-11",
                  index < groupFeeds.length - 1 && "border-b border-border/50",
                  feed.suspended && "opacity-65",
                )}
              >
                <div className="flex min-w-0 flex-1 items-center gap-2.5">
                  <FeedFavicon
                    src={getFaviconUrl(feed.link, feed.site_url)}
                    className="h-5 w-5"
                  />
                  <div className="min-w-0">
                    <div className="flex items-center gap-1.5">
                      <p className="truncate text-[13px] font-medium">
                        {feed.name}
                      </p>
                      {feed.suspended && (
                        <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-[var(--glass-soft)] px-2 py-0.5 text-[10.5px] font-semibold text-[var(--glass-sub)]">
                          <Pause className="h-2.5 w-2.5" />
                          {t("feeds.status.paused")}
                        </span>
                      )}
                    </div>
                    <p className="truncate text-[11px] text-muted-foreground">
                      {getDomain(feed.link)}
                    </p>
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-2 sm:gap-2.5">
                  {feed.fetch_state.last_error && (
                    <Tooltip
                      open={
                        isMobile
                          ? mobileErrorTooltipFeedId === feed.id
                          : undefined
                      }
                      onOpenChange={(open) => {
                        if (!isMobile || open) return;
                        onChangeMobileErrorTooltipFeedId((current) =>
                          current === feed.id ? null : current,
                        );
                      }}
                    >
                      <TooltipTrigger
                        render={
                          <button
                            type="button"
                            aria-label={t("feeds.status.error")}
                            onClick={() => {
                              if (!isMobile) return;
                              onChangeMobileErrorTooltipFeedId((current) =>
                                current === feed.id ? null : feed.id,
                              );
                            }}
                            className="flex items-center gap-1 rounded-sm text-xs text-destructive"
                          />
                        }
                      >
                        <AlertCircle className="h-3.5 w-3.5 shrink-0" />
                        <span className="hidden max-w-44 truncate font-medium md:inline">
                          {getFeedErrorPreview(feed.fetch_state.last_error)}
                        </span>
                      </TooltipTrigger>
                      <TooltipContent
                        side="top"
                        className="max-w-sm whitespace-normal break-words"
                      >
                        {feed.fetch_state.last_error.trim()}
                      </TooltipContent>
                    </Tooltip>
                  )}
                  <span className="hidden text-xs text-muted-foreground lg:inline">
                    {t("feeds.itemCount", {
                      count: feed.item_count,
                    })}{" "}
                    ·{" "}
                    {feed.fetch_state.last_checked_at > 0
                      ? formatDate(feed.fetch_state.last_checked_at)
                      : t("common.unknown")}
                  </span>

                  {/* Per-Feed AI Scoring Opt-in Chip (DEC-007 Option A, default off) */}
                  <Tooltip>
                    <TooltipTrigger
                      render={
                        <button
                          type="button"
                          role="switch"
                          aria-checked={feed.ai_enabled}
                          aria-label={t("feeds.aiToggle.aria", {
                            name: feed.name,
                          })}
                          disabled={isPending}
                          onClick={(e) => {
                            e.stopPropagation();
                            onToggleFeedAIEnabled(feed, !feed.ai_enabled);
                          }}
                          className={cn(
                            "inline-flex h-6 items-center gap-1 rounded-full border px-2 text-[11px] font-semibold transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--glass-acc)] disabled:cursor-not-allowed disabled:opacity-50",
                            feed.ai_enabled
                              ? "border-[var(--glass-acc)]/50 bg-[var(--glass-acc)]/14 text-[var(--glass-acc)]"
                              : "border-[var(--glass-inner-line)] bg-[var(--glass-panel)] text-[var(--glass-faint)] hover:text-[var(--glass-ink)]",
                          )}
                        />
                      }
                    >
                      <Sparkles className="h-3 w-3 shrink-0" />
                      <span>{t("feeds.aiToggle.label")}</span>
                    </TooltipTrigger>
                    <TooltipContent
                      side="top"
                      className="max-w-xs whitespace-normal break-words"
                    >
                      {t("feeds.aiToggle.tooltip")}
                    </TooltipContent>
                  </Tooltip>

                  {/* Liquid Glass Enable / Pause Switch */}
                  <GlassSwitch
                    size="sm"
                    checked={!feed.suspended}
                    loading={isPending}
                    aria-label={t("feeds.switch.aria", { name: feed.name })}
                    title={
                      feed.suspended
                        ? t("feeds.switch.resume")
                        : t("feeds.switch.pause")
                    }
                    onCheckedChange={(checked) =>
                      onToggleFeedEnabled(feed, checked)
                    }
                  />

                  <button
                    type="button"
                    onClick={() => onOpenEditFeed(feed)}
                    className="rounded p-1 hover:bg-accent"
                    aria-label={t("feed.edit.title")}
                  >
                    <Pencil className="h-3.5 w-3.5 text-muted-foreground" />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
