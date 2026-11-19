import { queryOptions, useQuery } from "@tanstack/react-query";
import { pluginAPI } from "@/lib/api";
import { queryKeys } from "./keys";

export const pluginQueries = {
  heartbeats: () =>
    queryOptions({
      queryKey: queryKeys.plugins.heartbeats(),
      queryFn: async () => {
        const res = await pluginAPI.heartbeats();
        return res.data ?? [];
      },
      staleTime: 30_000,
    }),
};

export function usePluginHeartbeats(enabled = true) {
  return useQuery({
    ...pluginQueries.heartbeats(),
    enabled,
  });
}
