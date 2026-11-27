import { useEffect, useRef, useState, type ReactNode } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  APIError,
  aiSettingsAPI,
  type AISettingsPatch,
  type AITriageSettings,
  type AITriageStats,
  type DecisionTestResult,
} from "@/lib/api";
import { useI18n } from "@/lib/i18n";

const control =
  "min-h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground focus-visible:outline-2 focus-visible:outline-ring";
const section = "grid gap-4 border-t border-border pt-4";

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="grid min-w-0 gap-1.5 text-sm font-medium">
      {label}
      {children}
    </label>
  );
}

export function AISettingsTab() {
  const { t } = useI18n();
  const [settings, setSettings] = useState<AITriageSettings>();
  const [stats, setStats] = useState<AITriageStats>();
  const [draft, setDraft] = useState<AISettingsPatch>({});
  const [operation, setOperation] = useState<"save" | "test" | "reset">();
  const [error, setError] = useState("");
  const [probe, setProbe] = useState<DecisionTestResult>();
  const [attempt, setAttempt] = useState(0);
  const form = useRef<HTMLFormElement>(null);
  const advanced = useRef<HTMLDetailsElement>(null);

  useEffect(() => {
    let active = true;
    setError("");
    Promise.all([aiSettingsAPI.get(), aiSettingsAPI.stats()])
      .then(([config, counts]) => {
        if (active) {
          setSettings(config.data);
          setStats(counts.data);
        }
      })
      .catch((err: unknown) => {
        if (active)
          setError(
            err instanceof APIError && err.status === 403
              ? t("settings.ai.denied")
              : err instanceof Error
                ? err.message
                : t("settings.ai.error"),
          );
      });
    return () => {
      active = false;
    };
  }, [attempt, t]);

  if (!settings || !stats) {
    return error ? (
      <div className="grid justify-items-start gap-3">
        <p role="alert">{error}</p>
        <Button variant="outline" onClick={() => setAttempt((n) => n + 1)}>
          {t("settings.ai.retry")}
        </Button>
      </div>
    ) : (
      <p role="status">{t("settings.ai.load")}</p>
    );
  }

  const values = { ...settings.effective, ...draft };
  const preset = settings.presets.find(
    (p) => p.id === values.decision_provider,
  );
  const busy = !!operation;
  const dirty = Object.keys(draft).length > 0;
  function change(patch: AISettingsPatch) {
    setDraft((previous) => ({ ...previous, ...patch }));
    setProbe(undefined);
    setError("");
  }
  async function act(action: "save" | "test" | "reset") {
    if (action === "reset") {
      if (!window.confirm(t("settings.ai.confirm"))) return;
    } else if (!form.current?.reportValidity()) return;
    setOperation(action);
    setError("");
    setProbe(undefined);
    try {
      if (action === "test") {
        setProbe((await aiSettingsAPI.test(draft)).data);
      } else {
        const result = await aiSettingsAPI.update(
          action === "reset" ? { reset_to_env: true } : draft,
        );
        setSettings(result.data);
        setDraft({});
        toast.success(t("settings.ai.saved"));
        setStats((await aiSettingsAPI.stats()).data);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("settings.ai.error"));
    } finally {
      setOperation(undefined);
    }
  }

  return (
    <form
      ref={form}
      className="space-y-5 pr-1"
      onSubmit={(e) => {
        e.preventDefault();
        void act("save");
      }}
      onInvalidCapture={(e) => {
        if (advanced.current?.contains(e.target as Node))
          advanced.current.open = true;
      }}
    >
      <section
        aria-label={t("settings.ai.status")}
        className="space-y-3 rounded-md bg-muted/50 p-3"
      >
        <p className="break-words text-sm font-medium">
          L2 · {stats.l2_enabled ? stats.l2_provider : t("settings.ai.off")}{" "}
          <span className="mx-1 text-muted-foreground">/</span> L3 ·{" "}
          {settings.l3_enabled
            ? [settings.l3_provider, settings.l3_model]
                .filter(Boolean)
                .join(" / ")
            : t("settings.ai.off")}
        </p>
        <dl className="grid grid-cols-2 gap-2 text-xs sm:grid-cols-4">
          {(
            [
              [t("settings.ai.l1"), stats.l1_filtered],
              [t("settings.ai.l2"), stats.l2_filtered],
              [t("settings.ai.skipped"), stats.l2_bypassed_l3],
              [t("settings.ai.calls"), stats.l3_called],
            ] as const
          ).map(([label, count]) => (
            <div key={label}>
              <dt className="text-muted-foreground">{label}</dt>
              <dd className="mt-1 text-base font-medium tabular-nums">
                {count}
              </dd>
            </div>
          ))}
        </dl>
      </section>
      <fieldset disabled={busy} className="grid min-w-0 gap-4">
        <label className="flex min-h-10 items-center gap-2 text-sm font-medium">
          <input
            type="checkbox"
            checked={
              values.decision_provider !== "off" &&
              (draft.decision_enabled ?? settings.decision_requested)
            }
            disabled={values.decision_provider === "off"}
            onChange={(e) => change({ decision_enabled: e.target.checked })}
          />
          {t("settings.ai.enabled")}
        </label>
        <Field label={t("settings.ai.provider")}>
          <select
            className={control}
            value={values.decision_provider}
            onChange={(e) =>
              change({
                decision_provider: e.target.value,
                decision_base_url: "",
                decision_model: "",
              })
            }
          >
            {settings.presets.map((p) => (
              <option key={p.id} value={p.id}>
                {p.id === "off" ? t("settings.ai.off") : p.label}
              </option>
            ))}
          </select>
        </Field>
        {preset?.requires_account_id && (
          <Field label={t("settings.ai.account")}>
            <input
              className={control}
              value={values.decision_account_id}
              pattern="[A-Za-z0-9_\-]{1,64}"
              onChange={(e) => change({ decision_account_id: e.target.value })}
            />
            <span className="text-xs font-normal text-muted-foreground">
              {t("settings.ai.accountHint")}
            </span>
          </Field>
        )}
        {values.decision_provider !== "off" && (
          <>
            <Field label={t("settings.ai.key")}>
              <input
                type="password"
                autoComplete="off"
                className={control}
                value={draft.decision_api_key ?? ""}
                placeholder={t("settings.ai.keep")}
                onChange={(e) =>
                  change({
                    decision_api_key: e.target.value,
                    clear_decision_api_key: false,
                  })
                }
              />
              <span className="text-xs font-normal text-muted-foreground">
                {t(`settings.ai.${settings.decision_api_key_source}`)}{" "}
                {settings.decision_api_key_masked}
              </span>
            </Field>
            {settings.decision_api_key_source === "db" && (
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={!!draft.clear_decision_api_key}
                  onChange={(e) =>
                    change({
                      clear_decision_api_key: e.target.checked,
                      decision_api_key: "",
                    })
                  }
                />
                {t("settings.ai.clear")}
              </label>
            )}
            <details
              ref={advanced}
              className="rounded-md border border-border p-3"
            >
              <summary className="cursor-pointer text-sm font-medium">
                {t("settings.ai.advanced")}
              </summary>
              <div className="mt-4 grid gap-3 sm:grid-cols-2">
                <Field label={t("settings.ai.base")}>
                  <input
                    type="url"
                    className={control}
                    value={values.decision_base_url}
                    placeholder={preset?.default_base_url}
                    onChange={(e) =>
                      change({ decision_base_url: e.target.value })
                    }
                  />
                </Field>
                <Field label={t("settings.ai.model")}>
                  <input
                    className={control}
                    value={values.decision_model}
                    placeholder={preset?.default_model}
                    onChange={(e) => change({ decision_model: e.target.value })}
                  />
                </Field>
                {values.decision_provider === "http_json" && (
                  <Field label={t("settings.ai.header")}>
                    <input
                      className={control}
                      value={values.decision_auth_header}
                      onChange={(e) =>
                        change({ decision_auth_header: e.target.value })
                      }
                    />
                  </Field>
                )}
                {values.decision_provider === "openai" && (
                  <label className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={values.decision_json_format}
                      onChange={(e) =>
                        change({ decision_json_format: e.target.checked })
                      }
                    />
                    {t("settings.ai.json")}
                  </label>
                )}
                <Field label={t("settings.ai.timeout")}>
                  <input
                    type="number"
                    min="1"
                    max="60"
                    required
                    className={control}
                    value={
                      Number.isFinite(values.decision_timeout_seconds)
                        ? values.decision_timeout_seconds
                        : ""
                    }
                    onChange={(e) =>
                      change({
                        decision_timeout_seconds: e.target.valueAsNumber,
                      })
                    }
                  />
                </Field>
                <Field label={t("settings.ai.priorities")}>
                  <input
                    required
                    className={control}
                    value={values.decision_l3_priorities}
                    placeholder="critical,high,normal"
                    onChange={(e) =>
                      change({ decision_l3_priorities: e.target.value })
                    }
                  />
                </Field>
                <Field label={t("settings.ai.low")}>
                  <select
                    className={control}
                    value={values.decision_low_action}
                    onChange={(e) =>
                      change({ decision_low_action: e.target.value })
                    }
                  >
                    {(["keep_unscored", "filter"] as const).map((v) => (
                      <option key={v} value={v}>
                        {t(`settings.ai.${v}`)}
                      </option>
                    ))}
                  </select>
                </Field>
                <Field label={t("settings.ai.fallback")}>
                  <select
                    className={control}
                    value={values.decision_fallback_mode}
                    onChange={(e) =>
                      change({ decision_fallback_mode: e.target.value })
                    }
                  >
                    {(["skip_to_l3", "keep_l1", "fail"] as const).map((v) => (
                      <option key={v} value={v}>
                        {t(`settings.ai.${v}`)}
                      </option>
                    ))}
                  </select>
                </Field>
              </div>
            </details>
            <div className="space-y-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => void act("test")}
              >
                {t(
                  operation === "test"
                    ? "settings.ai.testing"
                    : "settings.ai.test",
                )}
              </Button>
              {probe && (
                <p
                  role="status"
                  className={
                    probe.ok
                      ? "text-sm text-green-700 dark:text-green-400"
                      : "text-sm text-destructive"
                  }
                >
                  {probe.ok
                    ? `${t("settings.ai.success")} · ${probe.latency_ms} ms · ${probe.outcome.action} / ${probe.outcome.priority} / ${probe.outcome.domain}`
                    : probe.error}
                </p>
              )}
            </div>
          </>
        )}
        <section className={section}>
          <h3 className="text-sm font-semibold">{t("settings.ai.budget")}</h3>
          <Field label={t("settings.ai.topics")}>
            <textarea
              rows={3}
              className={control}
              value={values.topics}
              onChange={(e) => change({ topics: e.target.value })}
            />
          </Field>
          <Field label={t("settings.ai.limit")}>
            <input
              type="number"
              min="0"
              max="500"
              required
              className={control}
              value={
                Number.isFinite(values.rss_max_items_per_pull)
                  ? values.rss_max_items_per_pull
                  : ""
              }
              onChange={(e) =>
                change({ rss_max_items_per_pull: e.target.valueAsNumber })
              }
            />
          </Field>
        </section>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <div className="flex flex-wrap gap-2 border-t border-border pt-4">
          <Button type="submit" disabled={!dirty}>
            {t(
              operation === "save" ? "settings.ai.saving" : "settings.ai.save",
            )}
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => void act("reset")}
          >
            {t("settings.ai.reset")}
          </Button>
        </div>
      </fieldset>
    </form>
  );
}
