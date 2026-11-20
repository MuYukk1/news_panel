import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { pluginAPI, type PluginHeartbeat } from "@/lib/api";
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

export function useUpdatePlugin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      sourceId,
      enabled,
    }: {
      sourceId: string;
      enabled: boolean;
    }) => {
      const res = await pluginAPI.update(sourceId, { enabled });
      return res.data!;
    },
    onMutate: async ({ sourceId, enabled }) => {
      await qc.cancelQueries({ queryKey: queryKeys.plugins.heartbeats() });
      const prevHeartbeats = qc.getQueryData<PluginHeartbeat[]>(
        queryKeys.plugins.heartbeats(),
      );
      qc.setQueryData<PluginHeartbeat[]>(
        queryKeys.plugins.heartbeats(),
        (old) =>
          old?.map((hb) =>
            hb.source_id === sourceId ? { ...hb, enabled } : hb,
          ),
      );
      return { prevHeartbeats };
    },
    onError: (_error, _variables, context) => {
      if (context?.prevHeartbeats) {
        qc.setQueryData(queryKeys.plugins.heartbeats(), context.prevHeartbeats);
      }
    },
    onSuccess: (updated) => {
      qc.setQueryData<PluginHeartbeat[]>(
        queryKeys.plugins.heartbeats(),
        (old) =>
          old?.map((hb) =>
            hb.source_id === updated.source_id ? updated : hb,
          ),
      );
    },
    onSettled: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: queryKeys.plugins.all }),
        qc.invalidateQueries({ queryKey: queryKeys.feeds.all }),
      ]);
    },
  });
}
