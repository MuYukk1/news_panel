import { useTheme } from "next-themes";
import { Moon, Sun } from "lucide-react";
import { settingsAPI } from "@/lib/api";
import { waitForSettingsSeed } from "@/lib/settings-sync";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";

interface ThemeToggleButtonProps {
  className?: string;
}

export function ThemeToggleButton({ className }: ThemeToggleButtonProps) {
  const { t } = useI18n();
  const { resolvedTheme, setTheme } = useTheme();
  const isDark = resolvedTheme === "dark";

  const handleToggle = () => {
    const nextTheme = isDark ? "light" : "dark";
    setTheme(nextTheme);
    void waitForSettingsSeed()
      .then(() => settingsAPI.update({ theme: nextTheme }))
      .catch(() => undefined);
  };

  const label = isDark
    ? t("panel.theme.switchToLight")
    : t("panel.theme.switchToDark");

  return (
    <button
      type="button"
      onClick={handleToggle}
      aria-label={label}
      title={label}
      className={cn(
        "inline-flex h-9 w-9 items-center justify-center rounded-xl border border-[var(--glass-border)] bg-[var(--glass-panel)] text-[var(--glass-sub)] shadow-xs backdrop-blur-md transition-colors hover:bg-[var(--glass-soft-hover)] hover:text-[var(--glass-ink)]",
        className,
      )}
    >
      {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
    </button>
  );
}
