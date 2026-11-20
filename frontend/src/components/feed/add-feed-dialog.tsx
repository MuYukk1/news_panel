import { useMemo, useState } from "react";
import { ChevronDown, Plus, Radar, X } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { FeedMultiSelectList } from "@/components/feed/feed-multi-select-list";
import { useUIStore } from "@/store";
import { useGroups } from "@/queries/groups";
import { useCreateFeed, useFeedLookup } from "@/queries/feeds";
import {
  feedAPI,
  type CreateFeedRequest,
  type DiscoveredFeed,
} from "@/lib/api";
import {
  batchImportCandidates,
  discoveredToCandidates,
  type ImportCandidateFeed,
  type ImportGroupMode,
} from "@/lib/feed-import";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";

export function AddFeedDialog() {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const { isAddFeedOpen, setAddFeedOpen } = useUIStore();
  const { data: groups = [] } = useGroups();
  const { feeds: existingFeeds } = useFeedLookup();
  const createFeed = useCreateFeed();

  const [url, setUrl] = useState("");
  const [name, setName] = useState("");
  const [groupId, setGroupId] = useState<string>("");
  const [proxy, setProxy] = useState("");
  const [isAdvancedOpen, setIsAdvancedOpen] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isValidating, setIsValidating] = useState(false);

  // Multi-select batch import dialog state
  const [detectedCandidates, setDetectedCandidates] = useState<
    ImportCandidateFeed[]
  >([]);
  const [selectedLinks, setSelectedLinks] = useState<Set<string>>(new Set());
  const [batchSearchQuery, setBatchSearchQuery] = useState("");
  const [batchGroupMode, setBatchGroupMode] = useState<ImportGroupMode>("auto");
  const [batchGroupId, setBatchGroupId] = useState<string>("");
  const [isFeedSelectOpen, setIsFeedSelectOpen] = useState(false);
  const [isBatchSubmitting, setIsBatchSubmitting] = useState(false);

  const groupItems = groups.map((group) => ({
    value: group.id.toString(),
    label: group.name,
  }));

  const existingLinks = useMemo(
    () => new Set(existingFeeds.map((f) => f.link)),
    [existingFeeds],
  );

  const hasDetectedGroups = useMemo(
    () => detectedCandidates.some((c) => Boolean(c.groupName)),
    [detectedCandidates],
  );

  const resetBatchState = () => {
    setDetectedCandidates([]);
    setSelectedLinks(new Set());
    setBatchSearchQuery("");
    setBatchGroupMode("auto");
    setBatchGroupId("");
    setIsFeedSelectOpen(false);
  };

  const resetForm = () => {
    setUrl("");
    setName("");
    setGroupId("");
    setProxy("");
    setIsAdvancedOpen(false);
    resetBatchState();
  };

  const handleClose = () => {
    setAddFeedOpen(false);
    resetForm();
  };

  const openMultiSelectDialog = (feeds: DiscoveredFeed[]) => {
    const candidates = discoveredToCandidates(feeds);
    const hasGroups = candidates.some((c) => Boolean(c.groupName));
    setDetectedCandidates(candidates);
    setSelectedLinks(new Set(candidates.map((c) => c.link)));
    setBatchSearchQuery("");
    setBatchGroupMode(hasGroups ? "auto" : "single");
    setBatchGroupId(groupId || groups[0]?.id.toString() || "");
    setIsFeedSelectOpen(true);
  };

  const handleSelectSingleDetectedFeed = (feed: DiscoveredFeed) => {
    setUrl(feed.link);
    setName((prev) => {
      if (prev.trim() || !feed.title.trim()) {
        return prev;
      }
      return feed.title.trim();
    });
    resetBatchState();
    toast.success(t("feed.toast.detected"));
  };

  const validateRequest = (target: string) => ({
    url: target,
    ...(proxy.trim() ? { proxy: proxy.trim() } : {}),
  });

  const handleValidate = async () => {
    if (!url.trim()) return;

    setIsValidating(true);
    try {
      const response = await feedAPI.validate(validateRequest(url.trim()));
      const feeds = response.data?.feeds ?? [];

      if (feeds.length === 0) {
        toast.info(t("feed.toast.noFeedsForUrl"));
        return;
      }

      if (feeds.length === 1) {
        handleSelectSingleDetectedFeed(feeds[0]);
        return;
      }

      openMultiSelectDialog(feeds);
    } catch {
      toast.error(t("feed.toast.detectFailed"));
    } finally {
      setIsValidating(false);
    }
  };

  const handleBatchSubmit = async () => {
    const chosen = detectedCandidates.filter((c) => selectedLinks.has(c.link));
    if (chosen.length === 0) return;

    setIsBatchSubmitting(true);
    try {
      const effectiveMode: ImportGroupMode = hasDetectedGroups
        ? batchGroupMode
        : "single";
      const parsedTargetGroupId = batchGroupId
        ? parseInt(batchGroupId, 10)
        : groupId
          ? parseInt(groupId, 10)
          : (groups[0]?.id ?? 1);

      const { created, failed, errors } = await batchImportCandidates({
        candidates: chosen,
        groupMode: effectiveMode,
        targetGroupId: parsedTargetGroupId,
        defaultGroupName: t("opml.group.default"),
        queryClient,
      });

      if (created > 0) {
        toast.success(t("opml.toast.imported", { count: created }));
      }
      if (failed > 0) {
        const errorMsg = errors?.join(", ") || t("feed.toast.addFailed");
        toast.warning(
          t("opml.toast.failedItems", { count: failed, errors: errorMsg }),
        );
      }
      if (created === 0 && failed === 0) {
        toast.info(t("opml.toast.noNewFeeds"));
      }
      handleClose();
    } catch {
      toast.error(t("feed.toast.addFailed"));
    } finally {
      setIsBatchSubmitting(false);
    }
  };

  const handleSubmit = async () => {
    if (!url.trim()) {
      toast.error(t("feed.toast.enterUrl"));
      return;
    }

    setIsSubmitting(true);
    try {
      const check = await feedAPI.validate(validateRequest(url.trim()));
      const feeds = check.data?.feeds ?? [];
      if (feeds.length === 0) {
        toast.error(t("feed.toast.invalidFeed"));
        return;
      }

      const entered = url.trim();
      const normalizeUrl = (value: string) =>
        value.trim().replace(/\/+$/, "").toLowerCase();
      const match = feeds.find(
        (f) => normalizeUrl(f.link) === normalizeUrl(entered),
      );

      let link = entered;
      let title = "";
      let siteUrl = "";
      if (match) {
        link = match.link;
        title = match.title.trim();
        siteUrl = match.site_url?.trim() ?? "";
        setUrl(link);
      } else if (feeds.length === 1) {
        link = feeds[0].link;
        title = feeds[0].title.trim();
        siteUrl = feeds[0].site_url?.trim() ?? "";
        setUrl(link);
      } else {
        openMultiSelectDialog(feeds);
        return;
      }
      if (!name.trim() && title) {
        setName(title);
      }

      const selectedGroupId = groupId
        ? parseInt(groupId, 10)
        : (groups[0]?.id ?? 1);

      const request: CreateFeedRequest = {
        link,
        name: name.trim() || title || link,
        group_id: selectedGroupId,
        ...(siteUrl ? { site_url: siteUrl } : {}),
      };

      if (proxy.trim()) {
        request.proxy = proxy.trim();
      }

      await createFeed.mutateAsync(request);
      toast.success(t("feed.toast.added"));
      handleClose();
    } catch {
      toast.error(t("feed.toast.addFailed"));
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <>
      <Dialog open={isAddFeedOpen} onOpenChange={setAddFeedOpen}>
        <DialogContent
          className="flex w-full max-w-[480px] flex-col gap-0 overflow-hidden p-0"
          showCloseButton={false}
        >
          {/* Header */}
          <DialogHeader className="flex flex-row items-center justify-between border-b px-5 py-4">
            <DialogTitle className="text-base font-semibold">
              {t("feed.add.title")}
            </DialogTitle>
            <Button variant="ghost" size="icon-sm" onClick={handleClose}>
              <span className="sr-only">{t("common.cancel")}</span>
              <X className="h-[18px] w-[18px] text-muted-foreground" />
            </Button>
          </DialogHeader>

          {/* Form Content */}
          <div className="space-y-4 p-5">
            {/* URL Section */}
            <div className="space-y-1.5">
              <label htmlFor="add-feed-url" className="text-[13px] font-medium">
                {t("feed.add.urlLabel")}
              </label>
              <div className="flex gap-2">
                <Input
                  id="add-feed-url"
                  name="feed-url"
                  type="url"
                  inputMode="url"
                  placeholder={t("feed.add.urlPlaceholder")}
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  className="h-10"
                  autoComplete="off"
                  spellCheck={false}
                />
                <Button
                  variant="outline"
                  size="icon"
                  className="h-10 w-10 shrink-0"
                  onClick={handleValidate}
                  disabled={isValidating || !url.trim()}
                  aria-label={t("feed.add.validateTitle")}
                  title={t("feed.add.validateTitle")}
                >
                  <Radar
                    className={cn(
                      "h-[18px] w-[18px]",
                      isValidating && "animate-pulse",
                    )}
                  />
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                {t("feed.add.detectHint")}
              </p>
            </div>

            {/* Name Section */}
            <div className="space-y-1.5">
              <label htmlFor="add-feed-name" className="text-[13px] font-medium">
                {t("feed.add.nameLabel")}
              </label>
              <Input
                id="add-feed-name"
                name="feed-name"
                placeholder={t("feed.add.namePlaceholder")}
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="h-10"
                autoComplete="off"
              />
            </div>

            {/* Group Section */}
            <div className="space-y-1.5">
              <label className="text-[13px] font-medium" id="add-feed-group-label">
                {t("feed.add.groupLabel")}
              </label>
              <Select
                items={groupItems}
                value={groupId || null}
                onValueChange={(v) => {
                  if (v) setGroupId(v);
                }}
              >
                <SelectTrigger className="h-10" aria-labelledby="add-feed-group-label">
                  <SelectValue placeholder={t("feed.add.groupPlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  {groupItems.map((group) => (
                    <SelectItem key={group.value} value={group.value}>
                      {group.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* Advanced Section */}
            <Collapsible open={isAdvancedOpen} onOpenChange={setIsAdvancedOpen}>
              <CollapsibleTrigger className="flex w-full items-center gap-1.5 text-[13px] font-medium text-muted-foreground">
                <ChevronDown
                  className={cn(
                    "h-3.5 w-3.5 transition-transform",
                    isAdvancedOpen && "rotate-180",
                  )}
                />
                {t("feed.add.advanced")}
              </CollapsibleTrigger>
              <CollapsibleContent className="space-y-1.5 pl-5 pt-3">
                <label htmlFor="add-feed-proxy" className="text-[13px] font-medium">
                  {t("feed.add.proxyLabel")}
                </label>
                <Input
                  id="add-feed-proxy"
                  name="feed-proxy"
                  type="url"
                  inputMode="url"
                  placeholder={t("feed.add.proxyPlaceholder")}
                  value={proxy}
                  onChange={(e) => setProxy(e.target.value)}
                  className="h-10"
                  autoComplete="off"
                  spellCheck={false}
                />
                <p className="text-xs text-muted-foreground">
                  {t("feed.add.proxyHint")}
                </p>
              </CollapsibleContent>
            </Collapsible>
          </div>

          {/* Footer */}
          <div className="flex items-center justify-end gap-3 border-t px-5 py-4">
            <Button variant="outline" onClick={handleClose}>
              {t("common.cancel")}
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={isSubmitting || !url.trim()}
            >
              <Plus className="mr-1.5 h-4 w-4" />
              {t("feed.add.button")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Multi-Select Batch Discovery Dialog */}
      <Dialog
        open={isFeedSelectOpen}
        onOpenChange={(open) => {
          setIsFeedSelectOpen(open);
          if (!open) {
            resetBatchState();
          }
        }}
      >
        <DialogContent
          className="flex w-full max-w-[600px] flex-col gap-0 overflow-hidden p-0"
          showCloseButton={false}
        >
          <DialogHeader className="flex flex-row items-center justify-between border-b px-5 py-4">
            <div>
              <DialogTitle className="text-base font-semibold">
                {t("feed.select.title")}
              </DialogTitle>
              <DialogDescription>
                {t("feed.select.batchDescription")}
              </DialogDescription>
            </div>
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={resetBatchState}
            >
              <span className="sr-only">{t("common.cancel")}</span>
              <X className="h-[18px] w-[18px] text-muted-foreground" />
            </Button>
          </DialogHeader>

          <div className="space-y-4 p-5">
            <FeedMultiSelectList
              candidates={detectedCandidates}
              selectedLinks={selectedLinks}
              onChangeSelected={setSelectedLinks}
              searchQuery={batchSearchQuery}
              onChangeSearchQuery={setBatchSearchQuery}
              existingLinks={existingLinks}
            />

            {/* Target Group Strategy */}
            <div className="space-y-2 rounded-xl border border-border/70 bg-muted/30 p-3">
              {hasDetectedGroups && (
                <div className="flex flex-wrap gap-2">
                  <button
                    type="button"
                    className="np-pill-btn !h-7 !px-3 !text-xs"
                    aria-pressed={batchGroupMode === "auto"}
                    onClick={() => setBatchGroupMode("auto")}
                  >
                    {t("feed.batch.groupModeAuto")}
                  </button>
                  <button
                    type="button"
                    className="np-pill-btn !h-7 !px-3 !text-xs"
                    aria-pressed={batchGroupMode === "single"}
                    onClick={() => setBatchGroupMode("single")}
                  >
                    {t("feed.batch.groupModeSingle")}
                  </button>
                </div>
              )}

              {(!hasDetectedGroups || batchGroupMode === "single") && (
                <div className="flex items-center gap-2">
                  <label
                    className="shrink-0 text-xs font-medium text-muted-foreground"
                    id="batch-feed-group-label"
                  >
                    {t("feed.add.groupLabel")}
                  </label>
                  <Select
                    items={groupItems}
                    value={batchGroupId || null}
                    onValueChange={(v) => {
                      if (v) setBatchGroupId(v);
                    }}
                  >
                    <SelectTrigger
                      className="h-8 flex-1 text-xs"
                      aria-labelledby="batch-feed-group-label"
                    >
                      <SelectValue
                        placeholder={t("feed.add.groupPlaceholder")}
                      />
                    </SelectTrigger>
                    <SelectContent>
                      {groupItems.map((group) => (
                        <SelectItem key={group.value} value={group.value}>
                          {group.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}
            </div>
          </div>

          <div className="flex items-center justify-end gap-3 border-t px-5 py-4">
            <Button
              variant="outline"
              onClick={resetBatchState}
              disabled={isBatchSubmitting}
            >
              {t("common.cancel")}
            </Button>
            <Button
              onClick={() => void handleBatchSubmit()}
              disabled={isBatchSubmitting || selectedLinks.size === 0}
            >
              <Plus className="mr-1.5 h-4 w-4" />
              {isBatchSubmitting
                ? t("common.importing")
                : t("feed.batch.addButton", { count: selectedLinks.size })}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
