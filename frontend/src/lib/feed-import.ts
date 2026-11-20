import type { QueryClient } from "@tanstack/react-query";
import {
  feedAPI,
  groupAPI,
  type BatchCreateFeedsRequest,
  type BatchCreateFeedsResponse,
  type DiscoveredFeed,
} from "@/lib/api";
import type { ParsedFeed } from "@/lib/opml";
import { queryKeys } from "@/queries/keys";

export interface ImportCandidateFeed {
  name: string;
  link: string;
  siteUrl?: string;
  groupName?: string;
}

export type ImportGroupMode = "auto" | "single";

export function discoveredToCandidates(
  feeds: readonly DiscoveredFeed[],
): ImportCandidateFeed[] {
  const seen = new Set<string>();
  const result: ImportCandidateFeed[] = [];
  for (const feed of feeds) {
    const link = feed.link.trim();
    if (!link || seen.has(link)) continue;
    seen.add(link);
    const name = feed.title.trim() || link;
    const siteUrl = feed.site_url?.trim() || undefined;
    const groupName = feed.group_name?.trim() || undefined;
    result.push({ name, link, siteUrl, groupName });
  }
  return result;
}

export function parsedOpmlToCandidates(
  feeds: readonly ParsedFeed[],
): ImportCandidateFeed[] {
  const seen = new Set<string>();
  const result: ImportCandidateFeed[] = [];
  for (const feed of feeds) {
    const link = feed.link.trim();
    if (!link || seen.has(link)) continue;
    seen.add(link);
    const name = feed.name.trim() || link;
    const siteUrl = feed.siteUrl?.trim() || undefined;
    const groupName = feed.groupName?.trim() || undefined;
    result.push({ name, link, siteUrl, groupName });
  }
  return result;
}

export function filterImportCandidates(
  candidates: readonly ImportCandidateFeed[],
  query: string,
): ImportCandidateFeed[] {
  const normalized = query.trim().toLowerCase();
  if (!normalized) return [...candidates];
  return candidates.filter((candidate) => {
    if (candidate.name.toLowerCase().includes(normalized)) return true;
    if (candidate.link.toLowerCase().includes(normalized)) return true;
    if (candidate.groupName?.toLowerCase().includes(normalized)) return true;
    return false;
  });
}

export function toggleCandidateSelection(
  selected: ReadonlySet<string>,
  link: string,
): Set<string> {
  const next = new Set(selected);
  if (next.has(link)) {
    next.delete(link);
  } else {
    next.add(link);
  }
  return next;
}

export function areAllVisibleSelected(
  selected: ReadonlySet<string>,
  visible: readonly ImportCandidateFeed[],
): boolean {
  return visible.length > 0 && visible.every((item) => selected.has(item.link));
}

export function toggleAllVisibleSelection(
  selected: ReadonlySet<string>,
  visible: readonly ImportCandidateFeed[],
): Set<string> {
  const next = new Set(selected);
  if (areAllVisibleSelected(selected, visible)) {
    for (const item of visible) {
      next.delete(item.link);
    }
  } else {
    for (const item of visible) {
      next.add(item.link);
    }
  }
  return next;
}

export function collectMissingGroupNames(
  candidates: readonly ImportCandidateFeed[],
  existingGroupNames: ReadonlySet<string>,
): string[] {
  const missing: string[] = [];
  const seen = new Set<string>();
  for (const candidate of candidates) {
    const name = candidate.groupName?.trim();
    if (!name || existingGroupNames.has(name) || seen.has(name)) continue;
    seen.add(name);
    missing.push(name);
  }
  return missing;
}

export function mapCandidatesToBatchItems(
  candidates: readonly ImportCandidateFeed[],
  options: {
    groupMode: ImportGroupMode;
    targetGroupId: number;
    groupNameToId: ReadonlyMap<string, number>;
    fallbackGroupId: number;
  },
): BatchCreateFeedsRequest["feeds"] {
  return candidates.map((candidate) => {
    let groupId = options.targetGroupId;
    if (options.groupMode === "auto") {
      groupId = candidate.groupName
        ? (options.groupNameToId.get(candidate.groupName) ??
          options.fallbackGroupId)
        : options.fallbackGroupId;
    }
    return {
      group_id: groupId,
      name: candidate.name,
      link: candidate.link,
      ...(candidate.siteUrl ? { site_url: candidate.siteUrl } : {}),
    };
  });
}

export async function batchImportCandidates({
  candidates,
  groupMode,
  targetGroupId,
  defaultGroupName,
  queryClient,
}: {
  candidates: readonly ImportCandidateFeed[];
  groupMode: ImportGroupMode;
  targetGroupId?: number;
  defaultGroupName: string;
  queryClient: QueryClient;
}): Promise<BatchCreateFeedsResponse> {
  const groupsRes = await groupAPI.list();
  const existingGroups = groupsRes.data;
  const groupNameToId = new Map(existingGroups.map((g) => [g.name, g.id]));

  if (groupMode === "auto") {
    const missingNames = collectMissingGroupNames(
      candidates,
      new Set(groupNameToId.keys()),
    );
    for (const name of missingNames) {
      const created = await groupAPI.create({ name });
      if (created.data) {
        groupNameToId.set(name, created.data.id);
      }
    }
  }

  let fallbackGroupId: number | undefined =
    targetGroupId ?? existingGroups[0]?.id ?? groupNameToId.values().next().value;
  if (fallbackGroupId === undefined) {
    const created = await groupAPI.create({ name: defaultGroupName });
    if (created.data) {
      fallbackGroupId = created.data.id;
      groupNameToId.set(defaultGroupName, created.data.id);
    }
  }

  const resolvedTargetGroupId = targetGroupId ?? fallbackGroupId ?? 1;
  const feedsToCreate = mapCandidatesToBatchItems(candidates, {
    groupMode,
    targetGroupId: resolvedTargetGroupId,
    groupNameToId,
    fallbackGroupId: fallbackGroupId ?? resolvedTargetGroupId,
  });

  const response = await feedAPI.batchCreate({ feeds: feedsToCreate });

  await Promise.all([
    queryClient.invalidateQueries({ queryKey: queryKeys.groups.all }),
    queryClient.invalidateQueries({ queryKey: queryKeys.feeds.all }),
    queryClient.invalidateQueries({ queryKey: queryKeys.items.all }),
  ]);

  return response.data ?? { created: 0, failed: 0, errors: null };
}
