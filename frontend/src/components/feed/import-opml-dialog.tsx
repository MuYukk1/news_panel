import { useMemo, useRef, useState } from "react";
import { ArrowLeft, FileUp, Link2, Loader2, Upload, X } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
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
import { FeedMultiSelectList } from "@/components/feed/feed-multi-select-list";
import { useUIStore } from "@/store";
import { useGroups } from "@/queries/groups";
import { useFeedLookup } from "@/queries/feeds";
import { feedAPI } from "@/lib/api";
import {
  batchImportCandidates,
  discoveredToCandidates,
  parsedOpmlToCandidates,
  type ImportCandidateFeed,
  type ImportGroupMode,
} from "@/lib/feed-import";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { parseOPML } from "@/lib/opml";

type InputMode = "file" | "url";

export function ImportOpmlDialog() {
  const { t } = useI18n();
  const { isImportOpmlOpen, setImportOpmlOpen } = useUIStore();
  const queryClient = useQueryClient();
  const { data: groups = [] } = useGroups();
  const { feeds: existingFeeds } = useFeedLookup();
  const fileInputId = "opml-file-input";

  const [inputMode, setInputMode] = useState<InputMode>("file");
  const [file, setFile] = useState<File | null>(null);
  const [remoteUrl, setRemoteUrl] = useState("");
  const [isDragging, setIsDragging] = useState(false);
  const [isFetchingUrl, setIsFetchingUrl] = useState(false);
  const [isImporting, setIsImporting] = useState(false);

  // Parsed preview + multi-select state
  const [candidates, setCandidates] = useState<ImportCandidateFeed[]>([]);
  const [selectedLinks, setSelectedLinks] = useState<Set<string>>(new Set());
  const [searchQuery, setSearchQuery] = useState("");
  const [groupMode, setGroupMode] = useState<ImportGroupMode>("auto");
  const [targetGroupId, setTargetGroupId] = useState<string>("");

  const fileInputRef = useRef<HTMLInputElement>(null);

  const groupItems = groups.map((group) => ({
    value: group.id.toString(),
    label: group.name,
  }));

  const existingLinks = useMemo(
    () => new Set(existingFeeds.map((f) => f.link)),
    [existingFeeds],
  );

  const hasCandidateGroups = useMemo(
    () => candidates.some((c) => Boolean(c.groupName)),
    [candidates],
  );

  const hasPreview = candidates.length > 0;

  const resetPreview = () => {
    setCandidates([]);
    setSelectedLinks(new Set());
    setSearchQuery("");
    setGroupMode("auto");
    setTargetGroupId("");
  };

  const resetState = () => {
    setInputMode("file");
    setFile(null);
    setRemoteUrl("");
    setIsDragging(false);
    setIsFetchingUrl(false);
    resetPreview();
  };

  const handleClose = () => {
    setImportOpmlOpen(false);
    resetState();
  };

  const enterPreview = (nextCandidates: ImportCandidateFeed[]) => {
    const hasGroups = nextCandidates.some((c) => Boolean(c.groupName));
    setCandidates(nextCandidates);
    setSelectedLinks(new Set(nextCandidates.map((c) => c.link)));
    setSearchQuery("");
    setGroupMode(hasGroups ? "auto" : "single");
    setTargetGroupId(groups[0]?.id.toString() ?? "");
  };

  const handleFileSelect = async (selectedFile: File | null) => {
    if (!selectedFile) return;

    const validTypes = [
      "text/xml",
      "application/xml",
      "text/x-opml",
      "application/x-opml",
    ];
    const validExtensions = [".opml", ".xml"];
    const hasValidExtension = validExtensions.some((ext) =>
      selectedFile.name.toLowerCase().endsWith(ext),
    );

    if (!validTypes.includes(selectedFile.type) && !hasValidExtension) {
      toast.error(t("opml.toast.invalidFile"));
      return;
    }

    try {
      const content = await selectedFile.text();
      const parsed = parsedOpmlToCandidates(parseOPML(content));
      if (parsed.length === 0) {
        toast.info(t("opml.toast.noFeeds"));
        return;
      }
      setFile(selectedFile);
      enterPreview(parsed);
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t("opml.toast.failed"),
      );
    }
  };

  const handleFetchRemoteUrl = async () => {
    const trimmed = remoteUrl.trim();
    if (!trimmed) return;

    setIsFetchingUrl(true);
    try {
      const response = await feedAPI.validate({ url: trimmed });
      const discovered = discoveredToCandidates(response.data?.feeds ?? []);
      if (discovered.length === 0) {
        toast.info(t("opml.toast.noFeeds"));
        return;
      }
      enterPreview(discovered);
    } catch {
      toast.error(t("opml.toast.fetchUrlFailed"));
    } finally {
      setIsFetchingUrl(false);
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    const droppedFile = e.dataTransfer.files[0];
    void handleFileSelect(droppedFile);
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  };

  const handleImportSelected = async () => {
    const chosen = candidates.filter((c) => selectedLinks.has(c.link));
    if (chosen.length === 0) return;

    setIsImporting(true);
    try {
      const effectiveMode: ImportGroupMode = hasCandidateGroups
        ? groupMode
        : "single";
      const parsedTargetGroupId = targetGroupId
        ? parseInt(targetGroupId, 10)
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
        const errorMsg = errors?.join(", ") || t("opml.toast.failed");
        toast.warning(
          t("opml.toast.failedItems", { count: failed, errors: errorMsg }),
        );
      }
      if (created === 0 && failed === 0) {
        toast.info(t("opml.toast.noNewFeeds"));
      }
      handleClose();
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t("opml.toast.failed"),
      );
    } finally {
      setIsImporting(false);
    }
  };

  return (
    <Dialog
      open={isImportOpmlOpen}
      onOpenChange={(open) => {
        if (!open) {
          handleClose();
        } else {
          setImportOpmlOpen(true);
        }
      }}
    >
      <DialogContent
        className={cn(
          "flex w-full flex-col gap-0 overflow-hidden p-0 transition-[max-width]",
          hasPreview ? "max-w-[600px]" : "max-w-[500px]",
        )}
        showCloseButton={false}
      >
        <DialogHeader className="flex flex-row items-center justify-between border-b px-5 py-4">
          <DialogTitle className="text-base font-semibold">
            {t("opml.dialog.title")}
          </DialogTitle>
          <Button variant="ghost" size="icon-sm" onClick={handleClose}>
            <span className="sr-only">{t("common.cancel")}</span>
            <X className="h-[18px] w-[18px] text-muted-foreground" />
          </Button>
        </DialogHeader>

        <div className="space-y-4 p-5">
          {!hasPreview ? (
            <>
              {/* Mode Switch: Local File vs Remote URL */}
              <div
                className="np-seg w-full"
                role="tablist"
                aria-label={t("opml.dialog.title")}
              >
                <button
                  type="button"
                  role="tab"
                  aria-selected={inputMode === "file"}
                  aria-pressed={inputMode === "file"}
                  onClick={() => setInputMode("file")}
                  className="np-seg-btn flex flex-1 items-center justify-center gap-1.5"
                >
                  <FileUp className="h-3.5 w-3.5" />
                  {t("opml.dialog.tabFile")}
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={inputMode === "url"}
                  aria-pressed={inputMode === "url"}
                  onClick={() => setInputMode("url")}
                  className="np-seg-btn flex flex-1 items-center justify-center gap-1.5"
                >
                  <Link2 className="h-3.5 w-3.5" />
                  {t("opml.dialog.tabUrl")}
                </button>
              </div>

              {inputMode === "file" ? (
                <>
                  <input
                    ref={fileInputRef}
                    id={fileInputId}
                    type="file"
                    accept=".opml,.xml,text/xml,application/xml"
                    className="hidden"
                    aria-label={t("opml.dialog.title")}
                    onChange={(e) => {
                      const picked = e.target.files?.[0] || null;
                      e.target.value = "";
                      void handleFileSelect(picked);
                    }}
                  />

                  <label
                    htmlFor={fileInputId}
                    onDrop={handleDrop}
                    onDragOver={handleDragOver}
                    onDragLeave={handleDragLeave}
                    className={cn(
                      "flex cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed p-8 transition-colors",
                      isDragging
                        ? "border-primary bg-primary/5"
                        : "border-muted-foreground/25 hover:border-muted-foreground/50",
                    )}
                  >
                    <Upload className="mb-3 h-10 w-10 text-muted-foreground" />
                    <p className="text-sm font-medium">
                      {t("opml.dialog.dropHint")}
                    </p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {t("opml.dialog.supportHint")}
                    </p>
                  </label>
                </>
              ) : (
                <div className="space-y-3">
                  <div className="space-y-1.5">
                    <label
                      htmlFor="opml-remote-url"
                      className="text-[13px] font-medium"
                    >
                      {t("opml.dialog.urlLabel")}
                    </label>
                    <div className="flex gap-2">
                      <Input
                        id="opml-remote-url"
                        type="url"
                        inputMode="url"
                        placeholder={t("opml.dialog.urlPlaceholder")}
                        value={remoteUrl}
                        onChange={(e) => setRemoteUrl(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") {
                            e.preventDefault();
                            void handleFetchRemoteUrl();
                          }
                        }}
                        className="h-10"
                        autoComplete="off"
                        spellCheck={false}
                      />
                      <Button
                        type="button"
                        onClick={() => void handleFetchRemoteUrl()}
                        disabled={isFetchingUrl || !remoteUrl.trim()}
                        className="h-10 shrink-0 gap-1.5"
                      >
                        {isFetchingUrl && (
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        )}
                        {t("opml.dialog.fetchUrl")}
                      </Button>
                    </div>
                    <p className="text-xs text-muted-foreground">
                      {t("opml.dialog.urlHint")}
                    </p>
                  </div>
                </div>
              )}
            </>
          ) : (
            <>
              <div className="flex items-center justify-between gap-2 rounded-xl bg-[var(--glass-soft)] px-3 py-2 text-xs">
                <span className="truncate font-medium text-[var(--glass-ink)]">
                  {file ? file.name : remoteUrl}
                </span>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 shrink-0 gap-1 px-2 text-xs"
                  onClick={() => {
                    setFile(null);
                    resetPreview();
                  }}
                >
                  <ArrowLeft className="h-3 w-3" />
                  {t("opml.dialog.chooseDifferentSource")}
                </Button>
              </div>

              <FeedMultiSelectList
                candidates={candidates}
                selectedLinks={selectedLinks}
                onChangeSelected={setSelectedLinks}
                searchQuery={searchQuery}
                onChangeSearchQuery={setSearchQuery}
                existingLinks={existingLinks}
              />

              {/* Group Mapping Options */}
              <div className="space-y-2 rounded-xl border border-border/70 bg-muted/30 p-3">
                {hasCandidateGroups && (
                  <div className="flex flex-wrap gap-2">
                    <button
                      type="button"
                      className="np-pill-btn !h-7 !px-3 !text-xs"
                      aria-pressed={groupMode === "auto"}
                      onClick={() => setGroupMode("auto")}
                    >
                      {t("feed.batch.groupModeAuto")}
                    </button>
                    <button
                      type="button"
                      className="np-pill-btn !h-7 !px-3 !text-xs"
                      aria-pressed={groupMode === "single"}
                      onClick={() => setGroupMode("single")}
                    >
                      {t("feed.batch.groupModeSingle")}
                    </button>
                  </div>
                )}

                {(!hasCandidateGroups || groupMode === "single") && (
                  <div className="flex items-center gap-2">
                    <label
                      className="shrink-0 text-xs font-medium text-muted-foreground"
                      id="opml-target-group-label"
                    >
                      {t("feed.add.groupLabel")}
                    </label>
                    <Select
                      items={groupItems}
                      value={targetGroupId || null}
                      onValueChange={(v) => {
                        if (v) setTargetGroupId(v);
                      }}
                    >
                      <SelectTrigger
                        className="h-8 flex-1 text-xs"
                        aria-labelledby="opml-target-group-label"
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
            </>
          )}
        </div>

        <div className="flex items-center justify-end gap-3 border-t px-5 py-4">
          <Button variant="outline" onClick={handleClose} disabled={isImporting}>
            {t("common.cancel")}
          </Button>
          {hasPreview && (
            <Button
              onClick={() => void handleImportSelected()}
              disabled={selectedLinks.size === 0 || isImporting}
            >
              <Upload className="mr-1.5 h-4 w-4" />
              {isImporting
                ? t("common.importing")
                : t("opml.dialog.importSelected", { count: selectedLinks.size })}
            </Button>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
