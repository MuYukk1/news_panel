import { createLazyFileRoute } from "@tanstack/react-router";
import { SourcesPage } from "@/components/panel/sources-page";

export const Route = createLazyFileRoute("/sources")({
  component: SourcesPage,
});
