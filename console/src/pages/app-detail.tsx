import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "react-router";
import { del, errMessage, get, patch, post, put } from "../api/client";
import { getSession } from "../auth/token";
import type {
  ApnsCreds,
  AppUsagePlanView,
  AppView,
  FcmCreds,
  InstanceCreated,
  InstanceUsagePlanView,
  InstanceView,
  RotatedToken,
  TransportsView,
  TurnstileView,
  WebPushCreds,
} from "../api/types";
import { useToast } from "../components/toast";
import {
  Badge,
  Btn,
  Card,
  CopyBtn,
  EmptyState,
  ErrorNote,
  Field,
  Input,
  Modal,
  Section,
  Select,
  Spinner,
  Textarea,
  TokenReveal,
  Toggle,
} from "../components/ui";
import { fmtDate, fmtId, fmtLimit } from "../lib/format";

export default function AppDetail() {
  const { appId = "" } = useParams();
  const toast = useToast();
  const queryClient = useQueryClient();
  const isAdmin = getSession()?.principal === "admin";

  const appQ = useQuery({
    queryKey: ["app", appId],
    queryFn: () => get<AppView>(`/apps/${appId}`),
  });
  const effPlanQ = useQuery({
    queryKey: ["app-eff-plan", appId],
    queryFn: () => get<AppUsagePlanView>(`/apps/${appId}/usage-plan`),
  });
  const plansQ = useQuery({
    // Admin-only endpoint; app maintainers skip it so a 401 can't drop their session.
    queryKey: ["app-plans"],
    queryFn: () => get<AppUsagePlanView[]>("/usage-plans"),
    enabled: isAdmin,
  });
  const instQ = useQuery({
    queryKey: ["instances", appId],
    queryFn: () => get<InstanceView[]>(`/apps/${appId}/instances`),
  });
  const iPlansQ = useQuery({
    queryKey: ["inst-plans", appId],
    queryFn: () => get<InstanceUsagePlanView[]>(`/apps/${appId}/usage-plans`),
  });
  const credsQ = useQuery({
    queryKey: ["creds", appId],
    queryFn: () => get<TransportsView>(`/apps/${appId}/creds`),
  });
  const tsQ = useQuery({
    queryKey: ["turnstile", appId],
    queryFn: () => get<TurnstileView>(`/apps/${appId}/turnstile`),
  });

  const [createInstOpen, setCreateInstOpen] = useState(false);
  const [createdInst, setCreatedInst] = useState<InstanceCreated | null>(null);
  const [renameInst, setRenameInst] = useState<InstanceView | null>(null);
  const [rotateInst, setRotateInst] = useState<InstanceView | null>(null);
  const [rotatedInstToken, setRotatedInstToken] = useState<string | null>(null);
  const [deleteInst, setDeleteInst] = useState<InstanceView | null>(null);
  const [createPlanOpen, setCreatePlanOpen] = useState(false);
  const [editPlan, setEditPlan] = useState<InstanceUsagePlanView | null>(null);
  const [rotateAppTokenOpen, setRotateAppTokenOpen] = useState(false);
  const [rotatedAppToken, setRotatedAppToken] = useState<string | null>(null);
  const [rotateKeyOpen, setRotateKeyOpen] = useState(false);

  function refresh(...keys: string[]) {
    for (const k of keys) void queryClient.invalidateQueries({ queryKey: [k] });
    void queryClient.invalidateQueries({ queryKey: ["stats"] });
  }

  const rotateInstTokenM = useMutation({
    mutationFn: (inst: InstanceView) =>
      post<RotatedToken>(`/apps/${appId}/instances/${inst.id}/rotate-token`),
    onSuccess: (r) => {
      setRotatedInstToken(r.token);
      refresh("instances");
    },
    onError: (e) => {
      toast("error", errMessage(e));
      setRotateInst(null);
    },
  });

  const deleteInstM = useMutation({
    mutationFn: (inst: InstanceView) =>
      del(`/apps/${appId}/instances/${inst.id}`),
    onSuccess: () => {
      toast("success", "instance deleted");
      setDeleteInst(null);
      refresh("instances");
    },
    onError: (e) => {
      toast("error", errMessage(e));
      setDeleteInst(null);
    },
  });

  const rotateAppTokenM = useMutation({
    mutationFn: () => post<RotatedToken>(`/apps/${appId}/rotate-token`),
    onSuccess: (r) => setRotatedAppToken(r.token),
    onError: (e) => {
      toast("error", errMessage(e));
      setRotateAppTokenOpen(false);
    },
  });

  if (appQ.isPending) return <Spinner />;
  if (appQ.isError) {
    return <EmptyState title="app not found" sub={errMessage(appQ.error)} />;
  }
  const app = appQ.data;
  const configured = new Set(credsQ.data?.transports ?? []);

  return (
    <div className="flex flex-col gap-6">
      <div>
        {isAdmin ? (
          <Link
            to="/apps"
            className="font-mono text-sm font-bold text-ink/60 hover:underline"
          >
            ← all apps
          </Link>
        ) : (
          <Link
            to="/"
            className="font-mono text-sm font-bold text-ink/60 hover:underline"
          >
            ← overview
          </Link>
        )}
        <div className="mt-2 flex flex-wrap items-center gap-3">
          <h1 className="font-display text-3xl sm:text-4xl">{app.name}</h1>
          {app.is_public ? (
            <Badge tone="pink" tilt>
              🌍 public
            </Badge>
          ) : (
            <Badge tone="white">🔒 private</Badge>
          )}
          <span className="flex items-center gap-2 font-mono text-sm text-ink/55">
            <span title={app.id}>{fmtId(app.id, 14)}</span>
            <CopyBtn text={app.id} label="copy id" />
          </span>
        </div>
      </div>

      <SettingsSection
        app={app}
        plans={plansQ.data ?? []}
        effective={effPlanQ.data}
        isAdmin={isAdmin}
        onSaved={() => refresh("app", "app-eff-plan")}
      />

      <TransportsSection
        appId={app.id}
        configured={configured}
        onSaved={() => refresh("creds")}
      />

      <TurnstileSection
        appId={app.id}
        turnstile={tsQ.data}
        onSaved={() => refresh("turnstile", "app")}
      />

      <InstancesSection
        appId={app.id}
        instances={instQ.data ?? []}
        plans={iPlansQ.data ?? []}
        onCreate={() => setCreateInstOpen(true)}
        onRename={(i) => setRenameInst(i)}
        onRotate={(i) => setRotateInst(i)}
        onDelete={(i) => setDeleteInst(i)}
        onAssigned={() => refresh("instances")}
      />

      <InstancePlansSection
        appId={app.id}
        instances={instQ.data ?? []}
        plans={iPlansQ.data ?? []}
        onCreate={() => setCreatePlanOpen(true)}
        onEdit={(p) => setEditPlan(p)}
        onChanged={() => refresh("inst-plans", "instances")}
      />

      <Section title="danger zone ⚠" tone="pink">
        <p className="mb-4 text-sm font-medium text-ink/70">
          Rotation is immediate and irreversible. Anything still holding the old
          secret gets locked out.
        </p>
        <div className="flex flex-wrap gap-3">
          <Btn variant="white" onClick={() => setRotateAppTokenOpen(true)}>
            🔑 rotate app token
          </Btn>
          <Btn variant="grape" onClick={() => setRotateKeyOpen(true)}>
            🔐 rotate endpoint key
          </Btn>
        </div>
        <p className="mt-3 font-mono text-xs text-ink/50">
          endpoint keys sign sealed push URLs · key v{app.key_version}, accepts
          ≥ v{app.min_key_version}
        </p>
      </Section>

      <CreateInstanceModal
        appId={app.id}
        open={createInstOpen}
        onClose={() => setCreateInstOpen(false)}
        onCreated={(ic) => {
          setCreateInstOpen(false);
          setCreatedInst(ic);
          refresh("instances");
        }}
      />
      <Modal
        open={createdInst !== null}
        onClose={() => setCreatedInst(null)}
        title="instance registered!"
      >
        {createdInst && (
          <div className="flex flex-col gap-4">
            <TokenReveal
              token={createdInst.token}
              kind="instance token (pit_)"
            />
            <div className="flex justify-end">
              <Btn onClick={() => setCreatedInst(null)}>done</Btn>
            </div>
          </div>
        )}
      </Modal>

      <RenameInstanceModal
        appId={app.id}
        instance={renameInst}
        onClose={() => setRenameInst(null)}
        onSaved={() => {
          setRenameInst(null);
          refresh("instances");
        }}
      />

      <Modal
        open={rotateInst !== null && rotatedInstToken === null}
        onClose={() => setRotateInst(null)}
        title="rotate instance token?"
      >
        <p className="text-sm font-medium">
          The instance's current <code className="font-mono">pit_</code> token
          stops working the moment a new one is minted. Devices must be updated
          with the new token.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Btn variant="white" onClick={() => setRotateInst(null)}>
            nevermind
          </Btn>
          <Btn
            variant="pink"
            onClick={() => rotateInst && rotateInstTokenM.mutate(rotateInst)}
            disabled={rotateInstTokenM.isPending}
          >
            {rotateInstTokenM.isPending ? "rotating…" : "rotate it"}
          </Btn>
        </div>
      </Modal>
      <Modal
        open={rotatedInstToken !== null}
        onClose={() => {
          setRotatedInstToken(null);
          setRotateInst(null);
        }}
        title="new instance token"
      >
        {rotatedInstToken && (
          <div className="flex flex-col gap-4">
            <TokenReveal
              token={rotatedInstToken}
              kind="instance token (pit_)"
            />
            <div className="flex justify-end">
              <Btn
                onClick={() => {
                  setRotatedInstToken(null);
                  setRotateInst(null);
                }}
              >
                done
              </Btn>
            </div>
          </div>
        )}
      </Modal>

      <Modal
        open={deleteInst !== null}
        onClose={() => setDeleteInst(null)}
        title="delete instance?"
      >
        <p className="text-sm font-medium">
          <span className="font-bold">
            {deleteInst?.label || "This instance"}
          </span>{" "}
          loses its sealed push endpoint permanently. There is no undo.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Btn variant="white" onClick={() => setDeleteInst(null)}>
            keep it
          </Btn>
          <Btn
            variant="pink"
            onClick={() => deleteInst && deleteInstM.mutate(deleteInst)}
            disabled={deleteInstM.isPending}
          >
            {deleteInstM.isPending ? "deleting…" : "delete forever"}
          </Btn>
        </div>
      </Modal>

      <InstancePlanModal
        appId={app.id}
        instances={instQ.data ?? []}
        plan={editPlan}
        open={createPlanOpen || editPlan !== null}
        onClose={() => {
          setCreatePlanOpen(false);
          setEditPlan(null);
        }}
        onSaved={() => {
          setCreatePlanOpen(false);
          setEditPlan(null);
          refresh("inst-plans", "instances");
        }}
      />

      <Modal
        open={rotateAppTokenOpen && rotatedAppToken === null}
        onClose={() => {
          setRotateAppTokenOpen(false);
          setRotatedAppToken(null);
        }}
        title="rotate app token?"
      >
        <p className="text-sm font-medium">
          The current <code className="font-mono">pat_</code> token for{" "}
          <span className="font-bold">{app.name}</span> is invalidated
          immediately. App owners must swap in the new one.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Btn
            variant="white"
            onClick={() => {
              setRotateAppTokenOpen(false);
              setRotatedAppToken(null);
            }}
          >
            nevermind
          </Btn>
          <Btn
            variant="pink"
            onClick={() => rotateAppTokenM.mutate()}
            disabled={rotateAppTokenM.isPending}
          >
            {rotateAppTokenM.isPending ? "rotating…" : "rotate it"}
          </Btn>
        </div>
      </Modal>
      <Modal
        open={rotatedAppToken !== null}
        onClose={() => {
          setRotatedAppToken(null);
          setRotateAppTokenOpen(false);
        }}
        title="new app token"
      >
        {rotatedAppToken && (
          <div className="flex flex-col gap-4">
            <TokenReveal token={rotatedAppToken} kind="app token (pat_)" />
            <div className="flex justify-end">
              <Btn
                onClick={() => {
                  setRotatedAppToken(null);
                  setRotateAppTokenOpen(false);
                }}
              >
                done
              </Btn>
            </div>
          </div>
        )}
      </Modal>

      <RotateKeyModal
        appId={app.id}
        open={rotateKeyOpen}
        onClose={() => setRotateKeyOpen(false)}
        onDone={() => {
          setRotateKeyOpen(false);
          refresh("app");
        }}
      />
    </div>
  );
}

function SettingsSection({
  app,
  plans,
  effective,
  isAdmin,
  onSaved,
}: {
  app: AppView;
  plans: AppUsagePlanView[];
  effective?: AppUsagePlanView;
  isAdmin: boolean;
  onSaved: () => void;
}) {
  const toast = useToast();
  const [name, setName] = useState(app.name);
  const [nameErr, setNameErr] = useState<string | null>(null);

  const saveM = useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      patch(`/apps/${app.id}`, body),
    onSuccess: () => {
      toast("success", "saved");
      onSaved();
    },
    onError: (e) => toast("error", errMessage(e)),
  });

  const assignable = plans.filter(
    (p) => p.app_id === null || p.app_id === app.id,
  );

  return (
    <Section title="settings" tone="canvas">
      <div className="grid gap-6 md:grid-cols-2">
        <div className="flex flex-col gap-4">
          <Field label="name">
            <div className="flex gap-2">
              <Input value={name} onChange={(e) => setName(e.target.value)} />
              <Btn
                variant="cyan"
                onClick={() => {
                  if (!name.trim()) {
                    setNameErr("the name cannot be empty");
                    return;
                  }
                  setNameErr(null);
                  saveM.mutate({ name: name.trim() });
                }}
                disabled={name.trim() === app.name}
              >
                rename
              </Btn>
            </div>
            {nameErr && <ErrorNote text={nameErr} />}
          </Field>
          <Field
            label="self-registration"
            hint="public apps expose a rate-limited instance registration page"
          >
            <Toggle
              checked={app.is_public}
              onChange={(v) => saveM.mutate({ is_public: v })}
              label="public"
            />
          </Field>
        </div>

        <div className="flex flex-col gap-4">
          {isAdmin && (
            <Field
              label="app usage plan"
              hint="assignment is admin-only; empty falls back to the global default"
            >
              <Select
                value={app.usage_plan_id ?? ""}
                onChange={(e) =>
                  saveM.mutate({ usage_plan_id: e.target.value })
                }
              >
                <option value="">— global default —</option>
                {assignable.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                    {p.is_default ? " (default)" : ""}
                  </option>
                ))}
              </Select>
            </Field>
          )}
          {effective && (
            <Card className="bg-paper px-4 py-3">
              <div className="text-xs font-bold tracking-wide text-ink/55 uppercase">
                effective plan
              </div>
              <div className="mt-1 flex flex-wrap items-center gap-2">
                <span className="font-display">{effective.name}</span>
                <Badge tone="lime">
                  {fmtLimit(effective.push_per_min)}/min
                </Badge>
                <Badge tone="cyan">
                  {fmtLimit(effective.push_daily_quota)} daily
                </Badge>
                <Badge tone="tang">
                  reg {fmtLimit(effective.register_per_min)}/min
                </Badge>
                <Badge tone="grape">
                  sub {fmtLimit(effective.subscribe_per_min)}/min
                </Badge>
              </div>
            </Card>
          )}
        </div>
      </div>
    </Section>
  );
}

const TRANSPORT_LABELS: Record<string, string> = {
  apns: "APNs",
  fcm: "FCM",
  webpush: "WebPush",
};

function TransportsSection({
  appId,
  configured,
  onSaved,
}: {
  appId: string;
  configured: Set<string>;
  onSaved: () => void;
}) {
  const toast = useToast();
  async function remove(transport: string) {
    try {
      await del(`/apps/${appId}/creds/${transport}`);
      toast(
        "success",
        `${TRANSPORT_LABELS[transport] ?? transport} credentials removed`,
      );
      onSaved();
    } catch (e) {
      toast("error", errMessage(e));
    }
  }

  return (
    <div className="grid gap-6 xl:grid-cols-3">
      <ApnsCard
        appId={appId}
        configured={configured.has("apns")}
        onSaved={onSaved}
        onRemove={() => remove("apns")}
      />
      <FcmCard
        appId={appId}
        configured={configured.has("fcm")}
        onSaved={onSaved}
        onRemove={() => remove("fcm")}
      />
      <WebPushCard
        appId={appId}
        configured={configured.has("webpush")}
        onSaved={onSaved}
        onRemove={() => remove("webpush")}
      />
    </div>
  );
}

function TransportShell({
  title,
  desc,
  configured,
  onRemove,
  children,
}: {
  title: string;
  desc: string;
  configured: boolean;
  onRemove: () => void;
  children: React.ReactNode;
}) {
  return (
    <Card className="flex flex-col overflow-hidden">
      <div className="flex items-center justify-between gap-2 border-b-[3px] border-ink px-4 py-2.5">
        <h3 className="font-display text-sm tracking-wide uppercase">
          {title}
        </h3>
        {configured ? (
          <Badge tone="lime" tilt>
            configured
          </Badge>
        ) : (
          <Badge tone="white">not configured</Badge>
        )}
      </div>
      <div className="flex flex-1 flex-col gap-3 p-4">
        <p className="text-xs font-medium text-ink/55">{desc}</p>
        <div className="flex flex-1 flex-col gap-3">{children}</div>
        {configured && (
          <Btn
            variant="white"
            className="mt-1 self-start px-3 py-1.5 text-sm"
            onClick={onRemove}
          >
            remove credentials
          </Btn>
        )}
      </div>
    </Card>
  );
}

function ApnsCard({
  appId,
  configured,
  onSaved,
  onRemove,
}: {
  appId: string;
  configured: boolean;
  onSaved: () => void;
  onRemove: () => void;
}) {
  const toast = useToast();
  const [form, setForm] = useState<ApnsCreds>({
    key_p8: "",
    key_id: "",
    team_id: "",
    topic: "",
  });
  const [error, setError] = useState<string | null>(null);

  const putM = useMutation({
    mutationFn: () => put(`/apps/${appId}/creds/apns`, form),
    onSuccess: () => {
      toast("success", "APNs credentials saved");
      onSaved();
    },
    onError: (e) => toast("error", errMessage(e)),
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    if (
      !form.key_p8.trim() ||
      !form.key_id.trim() ||
      !form.team_id.trim() ||
      !form.topic.trim()
    ) {
      setError("all fields are required");
      return;
    }
    setError(null);
    putM.mutate();
  }

  return (
    <TransportShell
      title="🍎 APNs"
      desc="Apple Push Notification service — paste the .p8 signing key from your Apple developer account."
      configured={configured}
      onRemove={onRemove}
    >
      <form onSubmit={submit} className="flex flex-col gap-3">
        <Field label="key (.p8 contents)">
          <Textarea
            rows={3}
            value={form.key_p8}
            onChange={(e) => setForm({ ...form, key_p8: e.target.value })}
            placeholder="-----BEGIN PRIVATE KEY-----"
          />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="key id">
            <Input
              value={form.key_id}
              onChange={(e) => setForm({ ...form, key_id: e.target.value })}
            />
          </Field>
          <Field label="team id">
            <Input
              value={form.team_id}
              onChange={(e) => setForm({ ...form, team_id: e.target.value })}
            />
          </Field>
        </div>
        <Field label="topic">
          <Input
            value={form.topic}
            onChange={(e) => setForm({ ...form, topic: e.target.value })}
          />
        </Field>
        {error && <ErrorNote text={error} />}
        <Btn type="submit" variant="cyan" disabled={putM.isPending}>
          save credentials
        </Btn>
      </form>
    </TransportShell>
  );
}

function FcmCard({
  appId,
  configured,
  onSaved,
  onRemove,
}: {
  appId: string;
  configured: boolean;
  onSaved: () => void;
  onRemove: () => void;
}) {
  const toast = useToast();
  const [form, setForm] = useState<FcmCreds>({
    service_account_json: "",
  });
  const [error, setError] = useState<string | null>(null);

  const putM = useMutation({
    mutationFn: () => put(`/apps/${appId}/creds/fcm`, form),
    onSuccess: () => {
      toast("success", "FCM credentials saved");
      onSaved();
    },
    onError: (e) => toast("error", errMessage(e)),
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!form.service_account_json.trim()) {
      setError("all fields are required");
      return;
    }
    let sa: { project_id?: string };
    try {
      sa = JSON.parse(form.service_account_json);
    } catch {
      setError("service account JSON doesn't parse");
      return;
    }
    if (!sa.project_id) {
      setError("service account JSON has no project_id");
      return;
    }
    setError(null);
    putM.mutate();
  }

  return (
    <TransportShell
      title="🤖 FCM"
      desc="Firebase Cloud Messaging — paste the service-account JSON from your Firebase project."
      configured={configured}
      onRemove={onRemove}
    >
      <form onSubmit={submit} className="flex flex-col gap-3">
        <Field label="service account json">
          <Textarea
            rows={6}
            value={form.service_account_json}
            onChange={(e) =>
              setForm({ ...form, service_account_json: e.target.value })
            }
            placeholder='{ "type": "service_account", … }'
          />
        </Field>
        {error && <ErrorNote text={error} />}
        <Btn type="submit" variant="cyan" disabled={putM.isPending}>
          save credentials
        </Btn>
      </form>
    </TransportShell>
  );
}

function WebPushCard({
  appId,
  configured,
  onSaved,
  onRemove,
}: {
  appId: string;
  configured: boolean;
  onSaved: () => void;
  onRemove: () => void;
}) {
  const toast = useToast();
  const [form, setForm] = useState<WebPushCreds>({
    vapid_private_key: "",
    subject: "",
  });
  const [error, setError] = useState<string | null>(null);

  const putM = useMutation({
    mutationFn: () => put(`/apps/${appId}/creds/webpush`, form),
    onSuccess: () => {
      toast("success", "WebPush credentials saved");
      onSaved();
    },
    onError: (e) => toast("error", errMessage(e)),
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!form.vapid_private_key.trim() || !form.subject.trim()) {
      setError("all fields are required");
      return;
    }
    setError(null);
    putM.mutate();
  }

  return (
    <TransportShell
      title="🌐 WebPush"
      desc="VAPID keys for the WebPush transport — generate with `npx web-push generate-vapid-keys`."
      configured={configured}
      onRemove={onRemove}
    >
      <form onSubmit={submit} className="flex flex-col gap-3">
        <Field label="vapid private key">
          <Input
            value={form.vapid_private_key}
            onChange={(e) =>
              setForm({ ...form, vapid_private_key: e.target.value })
            }
          />
        </Field>
        <Field label="subject" hint="usually a mailto: contact URL">
          <Input
            value={form.subject}
            onChange={(e) => setForm({ ...form, subject: e.target.value })}
            placeholder="mailto:ops@example.com"
          />
        </Field>
        {error && <ErrorNote text={error} />}
        <Btn type="submit" variant="cyan" disabled={putM.isPending}>
          save credentials
        </Btn>
      </form>
    </TransportShell>
  );
}

function TurnstileSection({
  appId,
  turnstile,
  onSaved,
}: {
  appId: string;
  turnstile?: TurnstileView;
  onSaved: () => void;
}) {
  const toast = useToast();
  const [siteKey, setSiteKey] = useState("");
  const [secretKey, setSecretKey] = useState("");
  const [error, setError] = useState<string | null>(null);

  const putM = useMutation({
    mutationFn: () =>
      put(`/apps/${appId}/turnstile`, {
        site_key: siteKey,
        secret_key: secretKey,
      }),
    onSuccess: () => {
      setSecretKey("");
      toast("success", "turnstile configured");
      onSaved();
    },
    onError: (e) => toast("error", errMessage(e)),
  });
  const delM = useMutation({
    mutationFn: () => del(`/apps/${appId}/turnstile`),
    onSuccess: () => {
      toast("success", "turnstile removed");
      onSaved();
    },
    onError: (e) => toast("error", errMessage(e)),
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!siteKey.trim() || !secretKey.trim()) {
      setError("both site key and secret key are required");
      return;
    }
    setError(null);
    putM.mutate();
  }

  return (
    <Section
      title="turnstile 🛡"
      tone="cyan"
      aside={
        turnstile?.configured ? (
          <Badge tone="lime" tilt>
            protecting self-registration
          </Badge>
        ) : (
          <Badge tone="white">off</Badge>
        )
      }
    >
      <div className="grid gap-6 md:grid-cols-2">
        <div className="flex flex-col gap-3">
          <p className="text-sm font-medium text-ink/70">
            Cloudflare Turnstile gates the public registration page.{" "}
            {turnstile?.configured && (
              <>
                Current site key:{" "}
                <code className="rounded bg-canvas px-1.5 py-0.5 font-mono text-xs text-[#111]">
                  {turnstile.site_key}
                </code>
              </>
            )}
          </p>
          {turnstile?.configured && (
            <Btn
              variant="white"
              className="self-start px-3 py-1.5 text-sm"
              onClick={() => delM.mutate()}
              disabled={delM.isPending}
            >
              remove turnstile
            </Btn>
          )}
        </div>
        <form onSubmit={submit} className="flex flex-col gap-3">
          <Field label="site key">
            <Input
              value={siteKey}
              onChange={(e) => setSiteKey(e.target.value)}
            />
          </Field>
          <Field
            label="secret key"
            hint="stored sealed — never shown again after saving"
          >
            <Input
              type="password"
              value={secretKey}
              onChange={(e) => setSecretKey(e.target.value)}
            />
          </Field>
          {error && <ErrorNote text={error} />}
          <Btn
            type="submit"
            variant="cyan"
            className="self-start"
            disabled={putM.isPending}
          >
            {turnstile?.configured ? "replace keys" : "configure"}
          </Btn>
        </form>
      </div>
    </Section>
  );
}

function InstancesSection({
  appId,
  instances,
  plans,
  onCreate,
  onRename,
  onRotate,
  onDelete,
  onAssigned,
}: {
  appId: string;
  instances: InstanceView[];
  plans: InstanceUsagePlanView[];
  onCreate: () => void;
  onRename: (i: InstanceView) => void;
  onRotate: (i: InstanceView) => void;
  onDelete: (i: InstanceView) => void;
  onAssigned: () => void;
}) {
  const toast = useToast();

  const assignM = useMutation({
    mutationFn: (vars: { inst: InstanceView; planId: string }) =>
      patch(`/apps/${appId}/instances/${vars.inst.id}`, {
        usage_plan_id: vars.planId,
      }),
    onSuccess: () => {
      toast("success", "usage plan assigned");
      onAssigned();
    },
    onError: (e) => toast("error", errMessage(e)),
  });

  return (
    <Section
      title={`instances (${instances.length})`}
      tone="grape"
      aside={
        <Btn variant="white" className="px-3 py-1 text-xs" onClick={onCreate}>
          + new instance
        </Btn>
      }
    >
      {instances.length === 0 ? (
        <EmptyState
          title="no instances yet"
          sub="public apps get them via self-registration; private ones are minted here"
          action={
            <Btn variant="grape" onClick={onCreate}>
              mint one
            </Btn>
          }
        />
      ) : (
        <ul className="flex flex-col gap-3">
          {instances.map((i) => (
            <li
              key={i.id}
              className="rounded-lg border-[2px] border-ink bg-paper px-3.5 py-3"
            >
              <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-display">
                      {i.label || (
                        <span className="text-ink/40">unlabeled</span>
                      )}
                    </span>
                    <span className="flex items-center gap-1.5 font-mono text-xs text-ink/55">
                      <span title={i.id}>{fmtId(i.id, 10)}</span>
                      <CopyBtn text={i.id} label="copy" />
                    </span>
                  </div>
                  <div className="mt-0.5 font-mono text-xs text-ink/45">
                    joined {fmtDate(i.created_at)}
                  </div>
                </div>

                <Select
                  aria-label={`usage plan for ${i.label || i.id}`}
                  className="w-44"
                  value={i.usage_plan_id ?? ""}
                  onChange={(e) =>
                    assignM.mutate({ inst: i, planId: e.target.value })
                  }
                >
                  <option value="">app default</option>
                  {plans.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </Select>

                <div className="flex gap-2">
                  <Btn
                    variant="white"
                    className="px-2.5 py-1 text-xs"
                    onClick={() => onRename(i)}
                  >
                    ✏️ rename
                  </Btn>
                  <Btn
                    variant="white"
                    className="px-2.5 py-1 text-xs"
                    onClick={() => onRotate(i)}
                  >
                    🔑 rotate
                  </Btn>
                  <Btn
                    variant="pink"
                    className="px-2.5 py-1 text-xs"
                    onClick={() => onDelete(i)}
                  >
                    🗑 delete
                  </Btn>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}
    </Section>
  );
}

function CreateInstanceModal({
  appId,
  open,
  onClose,
  onCreated,
}: {
  appId: string;
  open: boolean;
  onClose: () => void;
  onCreated: (ic: InstanceCreated) => void;
}) {
  const [label, setLabel] = useState("");

  const createM = useMutation({
    mutationFn: () =>
      post<InstanceCreated>(
        `/apps/${appId}/instances`,
        label.trim() ? { label: label.trim() } : {},
      ),
    onSuccess: (ic) => {
      setLabel("");
      onCreated(ic);
    },
  });

  return (
    <Modal open={open} onClose={onClose} title="mint an instance">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          createM.mutate();
        }}
        className="flex flex-col gap-4"
      >
        <Field
          label="label"
          hint="optional — a human nickname for whoever runs it"
        >
          <Input
            autoFocus
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder="e.g. alice's homelab"
          />
        </Field>
        {createM.isError && <ErrorNote text={errMessage(createM.error)} />}
        <div className="flex justify-end gap-2">
          <Btn variant="white" onClick={onClose}>
            cancel
          </Btn>
          <Btn type="submit" variant="grape" disabled={createM.isPending}>
            {createM.isPending ? "minting…" : "mint it ⚡"}
          </Btn>
        </div>
      </form>
    </Modal>
  );
}

function RenameInstanceModal({
  appId,
  instance,
  onClose,
  onSaved,
}: {
  appId: string;
  instance: InstanceView | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  return (
    <Modal open={instance !== null} onClose={onClose} title="rename instance">
      {instance && (
        <RenameForm
          key={instance.id}
          appId={appId}
          instance={instance}
          onClose={onClose}
          onSaved={onSaved}
        />
      )}
    </Modal>
  );
}

function RenameForm({
  appId,
  instance,
  onClose,
  onSaved,
}: {
  appId: string;
  instance: InstanceView;
  onClose: () => void;
  onSaved: () => void;
}) {
  const toast = useToast();
  const [label, setLabel] = useState(instance.label);

  const renameM = useMutation({
    mutationFn: () =>
      patch(`/apps/${appId}/instances/${instance.id}`, { label: label.trim() }),
    onSuccess: () => {
      toast("success", "renamed");
      onSaved();
    },
    onError: (e) => toast("error", errMessage(e)),
  });

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        renameM.mutate();
      }}
      className="flex flex-col gap-4"
    >
      <Field label="label">
        <Input
          autoFocus
          value={label}
          onChange={(e) => setLabel(e.target.value)}
          placeholder="a human nickname"
        />
      </Field>
      <div className="flex justify-end gap-2">
        <Btn variant="white" onClick={onClose}>
          cancel
        </Btn>
        <Btn
          type="submit"
          disabled={renameM.isPending || label.trim() === instance.label}
        >
          save
        </Btn>
      </div>
    </form>
  );
}

function InstancePlansSection({
  appId,
  instances,
  plans,
  onCreate,
  onEdit,
  onChanged,
}: {
  appId: string;
  instances: InstanceView[];
  plans: InstanceUsagePlanView[];
  onCreate: () => void;
  onEdit: (p: InstanceUsagePlanView) => void;
  onChanged: () => void;
}) {
  const toast = useToast();

  const deleteM = useMutation({
    mutationFn: (planId: string) => del(`/apps/${appId}/usage-plans/${planId}`),
    onSuccess: () => {
      toast("success", "plan deleted");
      onChanged();
    },
    onError: (e) => toast("error", errMessage(e)),
  });

  const instanceLabel = (id: string | null) =>
    id ? (instances.find((i) => i.id === id)?.label ?? fmtId(id, 8)) : null;

  return (
    <Section
      title={`instance usage plans (${plans.length})`}
      tone="tang"
      aside={
        <Btn variant="white" className="px-3 py-1 text-xs" onClick={onCreate}>
          + new plan
        </Btn>
      }
    >
      {plans.length === 0 ? (
        <EmptyState
          title="no instance plans"
          sub="the app's default plan (minted at startup) always exists in spirit — create explicit ones to fine-tune"
        />
      ) : (
        <ul className="flex flex-col gap-3">
          {plans.map((p) => {
            const scoped = instanceLabel(p.instance_id);
            return (
              <li
                key={p.id}
                className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border-[2px] border-ink bg-paper px-3.5 py-3"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-display">{p.name}</span>
                    {p.is_default ? (
                      <Badge tone="lime" tilt>
                        default
                      </Badge>
                    ) : scoped ? (
                      <Badge tone="grape">scoped: {scoped}</Badge>
                    ) : (
                      <Badge tone="cyan">shared</Badge>
                    )}
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-2 font-mono text-sm">
                  <Badge tone="canvas">{fmtLimit(p.push_per_min)}/min</Badge>
                  <Badge tone="canvas">{fmtLimit(p.push_burst)} burst</Badge>
                  <Badge tone="canvas">
                    {fmtLimit(p.push_daily_quota)} daily
                  </Badge>
                </div>
                <div className="flex items-center gap-2">
                  {p.is_default ? (
                    <span className="self-center font-mono text-xs text-ink/50">
                      the default is immortal (undeletable)
                    </span>
                  ) : (
                    <Btn
                      variant="pink"
                      className="px-2.5 py-1 text-xs"
                      onClick={() => deleteM.mutate(p.id)}
                      disabled={deleteM.isPending}
                    >
                      🗑 delete
                    </Btn>
                  )}
                  <Btn
                    variant="white"
                    className="px-2.5 py-1 text-xs"
                    onClick={() => onEdit(p)}
                  >
                    ✏️ edit
                  </Btn>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </Section>
  );
}

function InstancePlanModal({
  appId,
  instances,
  plan,
  open,
  onClose,
  onSaved,
}: {
  appId: string;
  instances: InstanceView[];
  plan: InstanceUsagePlanView | null;
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  // Key by plan id (or "new") so the form state re-initializes per target.
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={plan ? `edit plan: ${plan.name}` : "new instance plan"}
    >
      {open && (
        <PlanForm
          key={plan?.id ?? "new"}
          appId={appId}
          instances={instances}
          plan={plan}
          onClose={onClose}
          onSaved={onSaved}
        />
      )}
    </Modal>
  );
}

function PlanForm({
  appId,
  instances,
  plan,
  onClose,
  onSaved,
}: {
  appId: string;
  instances: InstanceView[];
  plan: InstanceUsagePlanView | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const toast = useToast();
  const [name, setName] = useState(plan?.name ?? "");
  const [perMin, setPerMin] = useState(String(plan?.push_per_min ?? 60));
  const [burst, setBurst] = useState(String(plan?.push_burst ?? 10));
  const [quota, setQuota] = useState(String(plan?.push_daily_quota ?? 1000));
  const [scopeId, setScopeId] = useState(plan?.instance_id ?? "");
  const [error, setError] = useState<string | null>(null);
  const editing = plan !== null;

  const saveM = useMutation({
    mutationFn: () => {
      const body = {
        name: name.trim(),
        push_per_min: Number(perMin),
        push_burst: Number(burst),
        push_daily_quota: Number(quota),
      };
      return editing
        ? patch(`/apps/${appId}/usage-plans/${plan!.id}`, body)
        : post(
            `/apps/${appId}/usage-plans`,
            scopeId ? { ...body, instance_id: scopeId } : body,
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
    if (![perMin, burst, quota].every((v) => v !== "" && Number(v) >= 0)) {
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
          placeholder="e.g. power-users"
        />
      </Field>
      <div className="grid grid-cols-3 gap-3">
        <Field label="per minute">
          <Input
            type="number"
            min={0}
            value={perMin}
            onChange={(e) => setPerMin(e.target.value)}
          />
        </Field>
        <Field label="burst">
          <Input
            type="number"
            min={0}
            value={burst}
            onChange={(e) => setBurst(e.target.value)}
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
      {!editing && (
        <Field
          label="scope"
          hint="optional — scope the plan to a single instance instead of the whole app"
        >
          <Select value={scopeId} onChange={(e) => setScopeId(e.target.value)}>
            <option value="">all instances (shared)</option>
            {instances.map((i) => (
              <option key={i.id} value={i.id}>
                {i.label || fmtId(i.id, 8)}
              </option>
            ))}
          </Select>
        </Field>
      )}
      {error && <ErrorNote text={error} />}
      <div className="flex justify-end gap-2">
        <Btn variant="white" onClick={onClose}>
          cancel
        </Btn>
        <Btn type="submit" variant="tang" disabled={saveM.isPending}>
          {saveM.isPending ? "saving…" : "save plan"}
        </Btn>
      </div>
    </form>
  );
}

function RotateKeyModal({
  appId,
  open,
  onClose,
  onDone,
}: {
  appId: string;
  open: boolean;
  onClose: () => void;
  onDone: () => void;
}) {
  const toast = useToast();
  const [revokeOld, setRevokeOld] = useState(false);

  const rotateM = useMutation({
    mutationFn: () =>
      post(`/apps/${appId}/rotate-endpoint-key`, { revoke_old: revokeOld }),
    onSuccess: () => {
      toast(
        "success",
        revokeOld
          ? "endpoint key rotated — old one revoked"
          : "endpoint key rotated — old one stays until min version catches up",
      );
      onDone();
    },
    onError: (e) => toast("error", errMessage(e)),
  });

  return (
    <Modal open={open} onClose={onClose} title="rotate endpoint key?">
      <div className="flex flex-col gap-4">
        <p className="text-sm font-medium">
          Endpoint keys sign the sealed{" "}
          <code className="font-mono">/push/…</code> URLs devices hold. Rotating
          mints a new key; existing endpoints keep working until they expire —
          unless you revoke the old key now.
        </p>
        <Toggle
          checked={revokeOld}
          onChange={setRevokeOld}
          label="revoke old key immediately"
        />
        {revokeOld && (
          <ErrorNote text="Revoking immediately invalidates every sealed push endpoint that hasn't re-subscribed yet." />
        )}
        <div className="flex justify-end gap-2">
          <Btn variant="white" onClick={onClose}>
            nevermind
          </Btn>
          <Btn
            variant="pink"
            onClick={() => rotateM.mutate()}
            disabled={rotateM.isPending}
          >
            {rotateM.isPending ? "rotating…" : "rotate"}
          </Btn>
        </div>
      </div>
    </Modal>
  );
}
