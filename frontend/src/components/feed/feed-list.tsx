import { useGroups } from "@/queries/groups";
import { useFeedLookup } from "@/queries/feeds";
import { FeedGroup } from "./feed-group";
import { FeedItem } from "./feed-item";

export function FeedList() {
  const { data: groups = [], isLoading } = useGroups();
  const { feeds, getFeedsByGroup } = useFeedLookup();

  if (isLoading && groups.length === 0) {
    return (
      <div className="space-y-1.5 px-1 py-1">
        {[1, 2, 3].map((i) => (
          <div
            key={i}
            className="h-7 animate-pulse rounded-lg bg-[var(--glass-soft)]"
          />
        ))}
      </div>
    );
  }

  return (
    <div className="w-full min-w-0 space-y-0.5">
      {groups.map((group) => {
        const groupFeeds = getFeedsByGroup(group.id);
        return (
          <FeedGroup
            key={group.id}
            groupId={group.id}
            name={group.name}
            feeds={groupFeeds}
          />
        );
      })}

      {/* Ungrouped feeds (group_id = 0) */}
      {feeds
        .filter((f) => f.group_id === 0)
        .map((feed) => (
          <FeedItem key={feed.id} feed={feed} />
        ))}
    </div>
  );
}
