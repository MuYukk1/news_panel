import { useEffect, useState } from "react";
import { createLazyFileRoute, useNavigate } from "@tanstack/react-router";
import { Layers } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useIsMobile } from "@/hooks/use-mobile";
import { translate, useI18n } from "@/lib/i18n";
import { oidcAPI, sessionAPI } from "@/lib/api";

export const Route = createLazyFileRoute("/login")({
  component: LoginPage,
});

function LoginPage() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const isMobile = useIsMobile();
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [oidcEnabled, setOidcEnabled] = useState(false);
  const [oidcLoading, setOidcLoading] = useState(false);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    if (params.get("error") === "oidc_failed") {
      toast.error(translate("login.oidcFailed"));
      window.history.replaceState({}, "", "/login");
    }

    oidcAPI
      .status()
      .then((res) => {
        if (res.data?.enabled) {
          setOidcEnabled(true);
        }
      })
      .catch(() => {
        // Keep password login as fallback when OIDC status is unavailable.
      });
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    setIsLoading(true);
    try {
      await sessionAPI.login({ password });
      navigate({
        to: "/overview",
      });
    } catch {
      toast.error(t("login.invalidPassword"));
    } finally {
      setIsLoading(false);
    }
  };

  const handleOIDCLogin = async () => {
    setOidcLoading(true);
    try {
      const res = await oidcAPI.login();
      if (res.data?.auth_url) {
        window.location.href = res.data.auth_url;
      }
    } catch {
      toast.error(t("login.oidcStartFailed"));
      setOidcLoading(false);
    }
  };

  return (
    <div className="np-canvas flex min-h-screen items-center justify-center p-4">
      <div className="np-glass w-full max-w-sm space-y-6 p-7">
        <div className="flex flex-col items-center gap-2.5 text-center">
          <span
            className="np-tile np-tile-lg"
            style={{
              background: "linear-gradient(135deg, #5B5BEA 0%, #9A7BFF 100%)",
            }}
          >
            <Layers className="h-6 w-6" strokeWidth={2.2} />
          </span>
          <div>
            <h1 className="text-2xl font-bold tracking-tight text-[var(--glass-ink)]">
              news_panel
            </h1>
            <p className="mt-1 text-sm text-[var(--glass-sub)]">
              {t("login.enterPassword")}
            </p>
          </div>
        </div>

        {oidcEnabled && (
          <>
            <Button
              type="button"
              variant="outline"
              className="w-full rounded-xl border-[var(--glass-border)] bg-[var(--glass-panel)]"
              onClick={handleOIDCLogin}
              disabled={oidcLoading}
            >
              {oidcLoading ? t("login.oidcRedirecting") : t("login.oidcSignIn")}
            </Button>
            <div className="relative">
              <div className="absolute inset-0 flex items-center">
                <span className="w-full border-t border-[var(--glass-inner-line)]" />
              </div>
              <div className="relative flex justify-center text-xs uppercase">
                <span className="rounded-full bg-[var(--glass-panel)] px-2.5 text-[var(--glass-faint)]">
                  {t("common.or")}
                </span>
              </div>
            </div>
          </>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            id="login-password"
            name="password"
            type="password"
            placeholder={t("login.passwordPlaceholder")}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={isLoading}
            autoComplete="current-password"
            spellCheck={false}
            aria-label={t("login.passwordPlaceholder")}
            autoFocus={!isMobile}
            className="h-10 rounded-xl border-[var(--glass-inner-line)] bg-[var(--glass-panel)]"
          />
          <Button
            type="submit"
            className="h-10 w-full rounded-xl"
            disabled={isLoading}
          >
            {isLoading ? t("login.signingIn") : t("login.signIn")}
          </Button>
        </form>
      </div>
    </div>
  );
}
