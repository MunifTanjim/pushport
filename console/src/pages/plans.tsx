import { useState, type FormEvent, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { del, errMessage, get, patch, post } from "../api/client";
import type { AppUsagePlanView, AppView } from "../api/types";
import { useToast } from "../components/toast";
import {
  Badge,
  Btn,
  EmptyState,
  ErrorNote,
  Field,
  Input,
  Modal,
  Section,
  Select,
  Spinner,
} from "../components/ui";
import { fmtLimit } from "../lib/format";

export default function Plans() {
  const toast = useToast();
  const queryClient = useQueryClient();

  const plansQ = useQuery({
    queryKey: ["app-plans"],
    queryFn: () => get<AppUsagePlanView[]>("/usage-plans"),
  });
  const appsQ = useQuery({
    queryKey: ["apps"],
    queryFn: () => get<AppView[]>("/apps"),
  });

  const [createOpen, setCreateOpen] = useState(false);
  const [editPlan, setEditPlan] = useState<AppUsagePlanView | null>(null);
  const [deletePlan, setDeletePlan] = useState<AppUsagePlanView | null>(null);

  const plans = plansQ.data ?? [];
  const appName = (id: string | null) =>
    (appsQ.data ?? []).find((a) => a.id === id)?.name;

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ["app-plans"] });
    void queryClient.invalidateQueries({ queryKey: ["apps"] });
    void queryClient.invalidateQueries({ queryKey: ["stats"] });
  }

  const deleteM = useMutation({
    mutationFn: (planId: string) => del(`/usage-plans/${planId}`),
    onSuccess: () => {
      toast("success", "plan deleted");
      setDeletePlan(null);
      refresh();
    },
    onError: (e) => {
      toast("error", errMessage(e));
      setDeletePlan(null);
    },
  });

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="font-display text-3xl sm:text-4xl">
          app usage plans
          <Badge tone="grape" tilt className="ml-3 align-middle">
            {plans.length}
          </Badge>
        </h1>
        <Btn variant="grape" onClick={() => setCreateOpen(true)}>
          + new plan
        </Btn>
      </div>

      <p className="max-w-2xl text-sm font-medium text-ink/65">
        App plans cap how fast an app can push across all its instances.
        Plans can be <span className="font-bold">shared</span> (any app) or{" "}
        <span className="font-bold">scoped</span> to one app. Instance plans
        live inside each app's page. Limits of 0 mean unlimited.
      </p>

      {plansQ.isPending ? (
        <Spinner />
      ) : plansQ.isError ? (
        <EmptyState
          title="plans wouldn't load"
          sub={errMessage(plansQ.error)}
        />
      ) : plans.length === 0 ? (
        <EmptyState
          title="no plans"
          sub="the server seeds a global default at startup"
        />
      ) : (
        <ul className="flex flex-col gap-3">
          {plans.map((p) => {
            const scopedTo = appName(p.app_id);
            return (
              <li key={p.id}>
                <Section
                  title={p.name}
                  tone={p.is_default ? "lime" : "paper"}
                  aside={
                    <div className="flex items-center gap-2">
                      {p.is_default && (
                        <Badge tone="ink" tilt>
                          global default
                        </Badge>
                      )}
                      {scopedTo ? (
                        <Badge tone="pink">scoped: {scopedTo}</Badge>
                      ) : (
                        !p.is_default && <Badge tone="cyan">shared</Badge>
                      )}
                    </div>
                  }
                >
                  <div className="flex flex-wrap items-center gap-x-6 gap-y-3">
                    <div className="flex flex-wrap items-center gap-2 font-mono">
                      <Badge tone="canvas">
                        {fmtLimit(p.push_per_min)}/min
                      </Badge>
                      <Badge tone="canvas">
                        {fmtLimit(p.push_daily_quota)} daily
                      </Badge>
                      <Badge tone="tang">
                        reg {fmtLimit(p.register_per_min)}/min
                      </Badge>
                      <Badge tone="grape">
                        sub {fmtLimit(p.subscribe_per_min)}/min
                      </Badge>
                    </div>
                    <div className="ml-auto flex items-center gap-2">
                      {p.is_default ? (
                        <span className="self-center font-mono text-xs text-ink/50">
                          the default is immortal (undeletable)
                        </span>
                      ) : (
                        <Btn
                          variant="pink"
                          className="px-3 py-1.5 text-sm"
                          onClick={() => setDeletePlan(p)}
                        >
                          🗑 delete
                        </Btn>
                      )}
                      <Btn
                        variant="white"
                        className="px-3 py-1.5 text-sm"
                        onClick={() => setEditPlan(p)}
                      >
                        ✏️ edit
                      </Btn>
                    </div>
                  </div>
                </Section>
              </li>
            );
          })}
        </ul>
      )}

      <PlanModal
        open={createOpen || editPlan !== null}
        plan={editPlan}
        onClose={() => {
          setCreateOpen(false);
          setEditPlan(null);
        }}
        onSaved={() => {
          setCreateOpen(false);
          setEditPlan(null);
          refresh();
        }}
      />

      <Modal
        open={deletePlan !== null}
        onClose={() => setDeletePlan(null)}
        title="delete plan?"
      >
        <p className="text-sm font-medium">
          Deleting <span className="font-bold">{deletePlan?.name}</span> fails
          while any app still uses it — the server will tell you. Apps then fall
          back to the global default.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Btn variant="white" onClick={() => setDeletePlan(null)}>
            keep it
          </Btn>
          <Btn
            variant="pink"
            disabled={deleteM.isPending}
            onClick={() => deletePlan && deleteM.mutate(deletePlan.id)}
          >
            {deleteM.isPending ? "deleting…" : "delete"}
          </Btn>
        </div>
      </Modal>
    </div>
  );
}

function PlanModal({
  open,
  plan,
  onClose,
  onSaved,
}: {
  open: boolean;
  plan: AppUsagePlanView | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const toast = useToast();
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={plan ? `edit plan: ${plan.name}` : "new app plan"}
    >
      {open && (
        <PlanForm
          key={plan?.id ?? "new"}
          plan={plan}
          onClose={onClose}
          onSaved={onSaved}
          toast={toast}
        />
      )}
    </Modal>
  );
}

function PlanForm({
  plan,
  onClose,
  onSaved,
  toast,
}: {
  plan: AppUsagePlanView | null;
  onClose: () => void;
  onSaved: () => void;
  toast: ReturnType<typeof useToast>;
}) {
  const [name, setName] = useState(plan?.name ?? "");
  const [perMin, setPerMin] = useState(String(plan?.push_per_min ?? 60));
  const [quota, setQuota] = useState(String(plan?.push_daily_quota ?? 5000));
  const [regPerMin, setRegPerMin] = useState(
    String(plan?.register_per_min ?? 0),
  );
  const [regBurst, setRegBurst] = useState(String(plan?.register_burst ?? 0));
  const [regIpPerMin, setRegIpPerMin] = useState(
    String(plan?.register_ip_per_min ?? 0),
  );
  const [regIpBurst, setRegIpBurst] = useState(
    String(plan?.register_ip_burst ?? 0),
  );
  const [subPerMin, setSubPerMin] = useState(
    String(plan?.subscribe_per_min ?? 0),
  );
  const [subBurst, setSubBurst] = useState(String(plan?.subscribe_burst ?? 0));
  const [scopeAppId, setScopeAppId] = useState(plan?.app_id ?? "");
  const [error, setError] = useState<string | null>(null);
  const editing = plan !== null;

  const saveM = useMutation({
    mutationFn: () => {
      const body = {
        name: name.trim(),
        push_per_min: Number(perMin),
        push_daily_quota: Number(quota),
        register_per_min: Number(regPerMin),
        register_burst: Number(regBurst),
        register_ip_per_min: Number(regIpPerMin),
        register_ip_burst: Number(regIpBurst),
        subscribe_per_min: Number(subPerMin),
        subscribe_burst: Number(subBurst),
      };
      return editing
        ? patch(`/usage-plans/${plan!.id}`, body)
        : post<AppUsagePlanView>(
            "/usage-plans",
            scopeAppId ? { ...body, app_id: scopeAppId } : body,
          );
    },
    onSuccess: () => {
      toast("success", editing ? "plan updated" : "plan created");
      onSaved();
    },
    onError: (e) => toast("error", errMessage(e)),
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) {
      setError("the plan needs a name");
      return;
    }
    const limits = [
      perMin,
      quota,
      regPerMin,
      regBurst,
      regIpPerMin,
      regIpBurst,
      subPerMin,
      subBurst,
    ];
    if (limits.some((v) => v === "" || Number(v) < 0)) {
      setError("limits must be non-negative numbers (0 = unlimited)");
      return;
    }
    setError(null);
    saveM.mutate();
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <Field label="name">
        <Input
          autoFocus
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g. free / pro / enterprise"
        />
      </Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="pushes per minute">
          <Input
            type="number"
            min={0}
            value={perMin}
            onChange={(e) => setPerMin(e.target.value)}
          />
        </Field>
        <Field label="daily quota" hint="0 = ∞">
          <Input
            type="number"
            min={0}
            value={quota}
            onChange={(e) => setQuota(e.target.value)}
          />
        </Field>
      </div>

      <FieldGroup
        label="self-registration"
        hint="public instance sign-ups; 0 = ∞"
      >
        <div className="grid grid-cols-2 gap-3">
          <Field label="per minute">
            <Input
              type="number"
              min={0}
              value={regPerMin}
              onChange={(e) => setRegPerMin(e.target.value)}
            />
          </Field>
          <Field label="burst">
            <Input
              type="number"
              min={0}
              value={regBurst}
              onChange={(e) => setRegBurst(e.target.value)}
            />
          </Field>
          <Field label="per-IP per minute">
            <Input
              type="number"
              min={0}
              value={regIpPerMin}
              onChange={(e) => setRegIpPerMin(e.target.value)}
            />
          </Field>
          <Field label="per-IP burst">
            <Input
              type="number"
              min={0}
              value={regIpBurst}
              onChange={(e) => setRegIpBurst(e.target.value)}
            />
          </Field>
        </div>
      </FieldGroup>

      <FieldGroup
        label="subscribe"
        hint="device subscribe endpoint, per client IP; 0 = ∞"
      >
        <div className="grid grid-cols-2 gap-3">
          <Field label="per minute">
            <Input
              type="number"
              min={0}
              value={subPerMin}
              onChange={(e) => setSubPerMin(e.target.value)}
            />
          </Field>
          <Field label="burst">
            <Input
              type="number"
              min={0}
              value={subBurst}
              onChange={(e) => setSubBurst(e.target.value)}
            />
          </Field>
        </div>
      </FieldGroup>

      {!editing && (
        <ScopeField scopeAppId={scopeAppId} onScopeChange={setScopeAppId} />
      )}
      {error && <ErrorNote text={error} />}
      <div className="flex justify-end gap-2">
        <Btn variant="white" onClick={onClose}>
          cancel
        </Btn>
        <Btn type="submit" variant="grape" disabled={saveM.isPending}>
          {saveM.isPending ? "saving…" : "save plan"}
        </Btn>
      </div>
    </form>
  );
}

function FieldGroup({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-2">
      <div className="text-xs font-bold tracking-wide text-ink/55 uppercase">
        {label}
        {hint && (
          <span className="ml-2 font-medium text-ink/45 normal-case">
            {hint}
          </span>
        )}
      </div>
      {children}
    </div>
  );
}

function ScopeField({
  scopeAppId,
  onScopeChange,
}: {
  scopeAppId: string;
  onScopeChange: (v: string) => void;
}) {
  const appsQ = useQuery({
    queryKey: ["apps"],
    queryFn: () => get<AppView[]>("/apps"),
  });
  return (
    <Field
      label="scope"
      hint="optional — scope the plan to a single app; an app can only have one scoped plan"
    >
      <Select
        value={scopeAppId}
        onChange={(e) => onScopeChange(e.target.value)}
      >
        <option value="">shared (any app)</option>
        {(appsQ.data ?? []).map((a) => (
          <option key={a.id} value={a.id}>
            {a.name}
          </option>
        ))}
      </Select>
    </Field>
  );
}
