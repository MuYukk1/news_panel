import {
  parseTriageSearch,
  type TriageSearch,
} from "@/lib/triage-search";
import {
  useLocation,
  useNavigate,
  useParams,
  useSearch,
} from "@tanstack/react-router";
import { useCallback, useMemo } from "react";
import {
  defaultArticleFilter,
  isArticleFilter,
  type ArticleFilter,
} from "@/lib/article-filter";
import { parsePositiveIntegerParam } from "@/lib/route-params";

export type { ArticleFilter } from "@/lib/article-filter";

export function useUrlState() {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const params = useParams({ strict: false }) as {
    filter?: string;
    feedId?: string;
    groupId?: string;
  };
  const search = useSearch({ strict: false }) as Record<string, unknown>;

  const routeFeedId = parsePositiveIntegerParam(params.feedId);
  const routeGroupId = parsePositiveIntegerParam(params.groupId);
  const selectedArticleId = parsePositiveIntegerParam(search.article);
  const routeFilter =
    typeof params.filter === "string" && isArticleFilter(params.filter)
      ? params.filter
      : defaultArticleFilter;

  const triageSearch = useMemo(() => parseTriageSearch(search), [search]);

  const selectedFeedId = routeFeedId;
  const selectedGroupId = routeGroupId;
  const articleFilter = routeFilter;
  const isOverviewRoute = pathname === "/overview";

  const navigateToList = useCallback(
    ({
      filter,
      feedId,
      groupId,
      articleId,
      triagePatch,
      resetTriage = false,
      replace,
    }: {
      filter?: ArticleFilter;
      feedId?: number | null;
      groupId?: number | null;
      articleId?: number | null;
      triagePatch?: Partial<Omit<TriageSearch, "article">>;
      resetTriage?: boolean;
      replace?: boolean;
    } = {}) => {
      const nextFilter = filter ?? articleFilter;
      const nextFeedId = feedId === undefined ? selectedFeedId : feedId;
      const nextGroupId = groupId === undefined ? selectedGroupId : groupId;
      const nextArticleId =
        articleId === undefined ? selectedArticleId : articleId;
      const baseSearch: TriageSearch = resetTriage
        ? {}
        : parseTriageSearch(search);
      const nextSearch: TriageSearch = {
        ...baseSearch,
        ...triagePatch,
        article: nextArticleId ?? undefined,
      };

      if (nextGroupId !== null) {
        navigate({
          to: "/groups/$groupId/$filter",
          params: {
            groupId: String(nextGroupId),
            filter: nextFilter,
          },
          search: nextSearch,
          replace: replace ?? true,
        });
        return;
      }

      if (nextFeedId !== null) {
        navigate({
          to: "/feeds/$feedId/$filter",
          params: {
            feedId: String(nextFeedId),
            filter: nextFilter,
          },
          search: nextSearch,
          replace: replace ?? true,
        });
        return;
      }

      navigate({
        to: "/$filter",
        params: { filter: nextFilter },
        search: nextSearch,
        replace: replace ?? true,
      });
    },
    [
      articleFilter,
      search,
      navigate,
      selectedArticleId,
      selectedFeedId,
      selectedGroupId,
    ],
  );

  const setSelectedFeed = useCallback(
    (
      feedId: number | null,
      triagePatch?: Partial<Omit<TriageSearch, "article">>,
    ) => {
      navigateToList({
        feedId,
        groupId: null,
        articleId: null,
        triagePatch: { ...triagePatch, feed_id: undefined },
      });
    },
    [navigateToList],
  );

  const setSelectedGroup = useCallback(
    (groupId: number | null) => {
      navigateToList({
        groupId,
        feedId: null,
        articleId: null,
      });
    },
    [navigateToList],
  );

  const setSelectedArticle = useCallback(
    (articleId: number | null) => {
      if (isOverviewRoute) {
        const nextSearch: TriageSearch = {
          ...parseTriageSearch(search),
          article: articleId ?? undefined,
        };
        navigate({
          to: "/overview",
          search: nextSearch,
          replace: articleId === null ? true : selectedArticleId !== null,
        });
        return;
      }

      if (articleId === null) {
        navigateToList({ articleId: null, replace: true });
        return;
      }

      navigateToList({
        articleId,
        replace: selectedArticleId !== null,
      });
    },
    [isOverviewRoute, navigate, navigateToList, search, selectedArticleId],
  );

  const setArticleFilter = useCallback(
    (filter: ArticleFilter) => {
      navigateToList({
        filter,
        articleId: null,
      });
    },
    [navigateToList],
  );

  const setTriageSearch = useCallback(
    (
      patch: Partial<Omit<TriageSearch, "article">>,
      options?: { reset?: boolean; filter?: ArticleFilter },
    ) => {
      navigateToList({
        filter: options?.filter,
        triagePatch: patch,
        resetTriage: options?.reset ?? false,
        articleId: null,
        replace: true,
      });
    },
    [navigateToList],
  );

  const selectTopLevelFilter = useCallback(
    (
      filter: ArticleFilter,
      triagePatch?: Partial<Omit<TriageSearch, "article">>,
    ) => {
      navigateToList({
        filter,
        feedId: null,
        groupId: null,
        articleId: null,
        resetTriage: true,
        triagePatch,
      });
    },
    [navigateToList],
  );

  const selectAll = useCallback(() => {
    selectTopLevelFilter("all");
  }, [selectTopLevelFilter]);

  return {
    selectedFeedId,
    selectedGroupId,
    selectedArticleId,
    articleFilter,
    triageSearch,
    isOverviewRoute,
    setSelectedFeed,
    setSelectedGroup,
    setSelectedArticle,
    setArticleFilter,
    setTriageSearch,
    selectTopLevelFilter,
    selectAll,
  };
}
