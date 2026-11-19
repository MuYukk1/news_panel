import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

type ContentHeaderProps = ComponentProps<"header">;

export function ContentHeader({ className, ...props }: ContentHeaderProps) {
  return (
    <header
      className={cn(
        "np-header-glass flex h-[60px] shrink-0 items-center justify-between px-4 sm:px-6",
        className,
      )}
      {...props}
    />
  );
}
