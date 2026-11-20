import type { MouseEvent } from "react";
import { cn } from "@/lib/utils";

interface GlassSwitchProps {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  loading?: boolean;
  size?: "sm" | "md";
  id?: string;
  className?: string;
  title?: string;
  "aria-label": string;
}

export function GlassSwitch({
  checked,
  onCheckedChange,
  disabled = false,
  loading = false,
  size = "md",
  id,
  className,
  title,
  "aria-label": ariaLabel,
}: GlassSwitchProps) {
  const isDisabled = disabled || loading;

  const handleClick = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    if (isDisabled) return;
    onCheckedChange(!checked);
  };

  return (
    <button
      id={id}
      type="button"
      role="switch"
      aria-checked={checked}
      aria-busy={loading || undefined}
      aria-label={ariaLabel}
      title={title}
      disabled={isDisabled}
      onClick={handleClick}
      className={cn(
        "np-switch",
        size === "sm" && "np-switch-sm",
        loading && "animate-pulse",
        className,
      )}
    >
      <span aria-hidden="true" className="np-switch-thumb" />
    </button>
  );
}
