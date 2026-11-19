import { createLazyFileRoute } from "@tanstack/react-router";
import { OverviewPage } from "@/components/panel/overview-page";

export const Route = createLazyFileRoute("/overview")({
  component: OverviewPage,
});
