import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { errMessage, get, patch } from "../api/client";
import type { SettingsView } from "../api/types";
import { useToast } from "../components/toast";
import {
  Badge,
  Btn,
  EmptyState,
  ErrorNote,
  Field,
  Input,
  Section,
  Spinner,
} from "../components/ui";

export default function Settings() {
  const settingsQ = useQuery({
    queryKey: ["settings"],
    queryFn: () => get<SettingsView>("/settings"),
  });

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="font-display text-3xl sm:text-4xl">
          server settings
          <Badge tone="grape" tilt className="ml-3 align-middle">
            global
          </Badge>
        </h1>
      </div>

      <p className="max-w-2xl text-sm font-medium text-ink/65">
        Server-wide delivery and abuse-hardening knobs. Changes take effect
        immediately — no restart. Retry and auth-failure limits keep protective
        defaults; self-registration and subscribe limits live on{" "}
        <span className="font-bold">usage plans</span>.
      </p>

      {settingsQ.isPending ? (
        <Spinner />
      ) : settingsQ.isError ? (
        <EmptyState
          title="settings wouldn't load"
          sub={errMessage(settingsQ.error)}
        />
      ) : (
        <SettingsForm data={settingsQ.data} />
      )}
    </div>
  );
}

function SettingsForm({ data }: { data: SettingsView }) {
  const toast = useToast();
  const queryClient = useQueryClient();

  const [maxAttempts, setMaxAttempts] = useState(
    String(data.retry_max_attempts),
  );
  const [baseBackoff, setBaseBackoff] = useState(data.retry_base_backoff);
  const [maxBackoff, setMaxBackoff] = useState(data.retry_max_backoff);
  const [authPerMin, setAuthPerMin] = useState(
    String(data.auth_fail_ip_per_min),
  );
  const [authBurst, setAuthBurst] = useState(String(data.auth_fail_ip_burst));
  const [error, setError] = useState<string | null>(null);

  const saveM = useMutation({
    mutationFn: () =>
      patch("/settings", {
        retry_max_attempts: Number(maxAttempts),
        retry_base_backoff: baseBackoff.trim(),
        retry_max_backoff: maxBackoff.trim(),
        auth_fail_ip_per_min: Number(authPerMin),
        auth_fail_ip_burst: Number(authBurst),
      }),
    onSuccess: () => {
      toast("success", "settings saved");
      void queryClient.invalidateQueries({ queryKey: ["settings"] });
    },
    onError: (e) => setError(errMessage(e)),
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    if (
      [maxAttempts, authPerMin, authBurst].some(
        (v) => v === "" || Number(v) < 0,
      )
    ) {
      setError("limits must be non-negative numbers (0 = unlimited)");
      return;
    }
    if (!baseBackoff.trim() || !maxBackoff.trim()) {
      setError("backoffs are required (Go durations, e.g. 200ms, 5s)");
      return;
    }
    setError(null);
    saveM.mutate();
  }

  return (
    <form onSubmit={submit} className="flex max-w-2xl flex-col gap-6">
      <Section title="delivery retry" tone="paper">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <Field label="max attempts">
            <Input
              type="number"
              min={1}
              value={maxAttempts}
              onChange={(e) => setMaxAttempts(e.target.value)}
            />
          </Field>
          <Field label="base backoff" hint="e.g. 200ms">
            <Input
              value={baseBackoff}
              onChange={(e) => setBaseBackoff(e.target.value)}
            />
          </Field>
          <Field label="max backoff" hint="e.g. 5s">
            <Input
              value={maxBackoff}
              onChange={(e) => setMaxBackoff(e.target.value)}
            />
          </Field>
        </div>
      </Section>

      <Section title="auth-failure throttle" tone="paper">
        <p className="mb-3 text-sm font-medium text-ink/60">
          Per client IP; guards bearer credentials (esp. the admin token)
          against online brute force. 0 = unlimited.
        </p>
        <div className="grid grid-cols-2 gap-3">
          <Field label="failures per minute">
            <Input
              type="number"
              min={0}
              value={authPerMin}
              onChange={(e) => setAuthPerMin(e.target.value)}
            />
          </Field>
          <Field label="burst">
            <Input
              type="number"
              min={0}
              value={authBurst}
              onChange={(e) => setAuthBurst(e.target.value)}
            />
          </Field>
        </div>
      </Section>

      {error && <ErrorNote text={error} />}

      <div className="flex justify-end">
        <Btn type="submit" variant="grape" disabled={saveM.isPending}>
          {saveM.isPending ? "saving…" : "save settings"}
        </Btn>
      </div>
    </form>
  );
}
