import { useMemo, useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useSearchParams } from "react-router";
import { errMessage, get, post } from "../api/client";
import type { AppCreated, AppUsagePlanView, AppView } from "../api/types";
import { useToast } from "../components/toast";
import {
  Badge,
  Btn,
  Card,
  EmptyState,
  ErrorNote,
  Field,
  Input,
  Modal,
  Spinner,
  TokenReveal,
  Toggle,
} from "../components/ui";
import { fmtDate, fmtId } from "../lib/format";

export default function Apps() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  const appsQ = useQuery({
    queryKey: ["apps"],
    queryFn: () => get<AppView[]>("/apps"),
  });
  const plansQ = useQuery({
    queryKey: ["app-plans"],
    queryFn: () => get<AppUsagePlanView[]>("/usage-plans"),
  });

  const [search, setSearch] = useState("");
  const [createOpen, setCreateOpen] = useState(
    searchParams.get("create") === "1",
  );
  const [created, setCreated] = useState<AppCreated | null>(null);

  const apps = appsQ.data ?? [];
  const plans = plansQ.data ?? [];
  const planName = (id: string | null) => plans.find((p) => p.id === id)?.name;

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return apps;
    return apps.filter(
      (a) => a.name.toLowerCase().includes(q) || a.id.toLowerCase().includes(q),
    );
  }, [apps, search]);

  function openCreate(next: boolean) {
    setCreateOpen(next);
    if (searchParams.get("create")) {
      const sp = new URLSearchParams(searchParams);
      sp.delete("create");
      setSearchParams(sp, { replace: true });
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="font-display text-3xl sm:text-4xl">
          apps
          <Badge tone="pink" tilt className="ml-3 align-middle">
            {apps.length}
          </Badge>
        </h1>
        <div className="flex flex-wrap items-center gap-2">
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="search by name or id…"
            aria-label="Search apps"
            className="w-56"
          />
          <Btn onClick={() => openCreate(true)}>+ new app</Btn>
        </div>
      </div>

      {appsQ.isPending ? (
        <Spinner />
      ) : appsQ.isError ? (
        <EmptyState title="apps wouldn't load" sub={errMessage(appsQ.error)} />
      ) : filtered.length === 0 ? (
        <EmptyState
          title={
            apps.length === 0 ? "no apps yet" : "nothing matches that search"
          }
          sub={
            apps.length === 0
              ? "every relay needs a first tenant — make one!"
              : undefined
          }
          action={
            apps.length === 0 ? (
              <Btn onClick={() => openCreate(true)}>create an app</Btn>
            ) : undefined
          }
        />
      ) : (
        <ul className="flex flex-col gap-4">
          {filtered.map((a) => (
            <li key={a.id}>
              <Link to={`/apps/${a.id}`} className="group block">
                <Card className="nb-hover flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3.5 sm:px-5">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-display text-lg">{a.name}</span>
                      {a.is_public ? (
                        <Badge tone="pink" tilt>
                          🌍 public
                        </Badge>
                      ) : (
                        <Badge tone="white">🔒 private</Badge>
                      )}
                      {planName(a.usage_plan_id) && (
                        <Badge tone="grape">
                          ⚡ {planName(a.usage_plan_id)}
                        </Badge>
                      )}
                    </div>
                    <div className="mt-1 flex flex-wrap items-center gap-x-4 font-mono text-xs text-ink/55">
                      <span title={a.id}>{fmtId(a.id, 14)}</span>
                      <span>created {fmtDate(a.created_at)}</span>
                      <span>
                        keys v{a.key_version} / min v{a.min_key_version}
                      </span>
                    </div>
                  </div>
                  <span
                    aria-hidden
                    className="font-display text-xl transition-transform group-hover:translate-x-1"
                  >
                    →
                  </span>
                </Card>
              </Link>
            </li>
          ))}
        </ul>
      )}

      <CreateAppModal
        open={createOpen}
        onClose={() => openCreate(false)}
        onCreated={(a) => {
          openCreate(false);
          setCreated(a);
          void queryClient.invalidateQueries({ queryKey: ["apps"] });
          void queryClient.invalidateQueries({ queryKey: ["stats"] });
          toast("success", `app "${a.name}" is live`);
        }}
      />

      <Modal
        open={created !== null}
        onClose={() => setCreated(null)}
        title="app created!"
      >
        {created && (
          <div className="flex flex-col gap-4">
            <TokenReveal token={created.token} kind="app token (pat_)" />
            <div className="flex flex-wrap justify-end gap-2">
              <Btn variant="white" onClick={() => setCreated(null)}>
                done
              </Btn>
              <Btn
                variant="lime"
                onClick={() => {
                  const id = created.id;
                  setCreated(null);
                  navigate(`/apps/${id}`);
                }}
              >
                open the app →
              </Btn>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}

function CreateAppModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (a: AppCreated) => void;
}) {
  const [name, setName] = useState("");
  const [isPublic, setIsPublic] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const createM = useMutation({
    mutationFn: (body: { name: string; is_public: boolean }) =>
      post<AppCreated>("/apps", body),
    onSuccess: (a) => {
      setName("");
      setIsPublic(false);
      setError(null);
      onCreated(a);
    },
    onError: (e) => setError(errMessage(e)),
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) {
      setError("the app needs a name");
      return;
    }
    createM.mutate({ name: name.trim(), is_public: isPublic });
  }

  return (
    <Modal open={open} onClose={onClose} title="create an app">
      <form onSubmit={submit} className="flex flex-col gap-4">
        <Field label="name" hint="how your backends will refer to this tenant">
          <Input
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. acme-production"
          />
        </Field>
        <Field
          label="self-registration"
          hint="public apps let anyone register an instance at /apps/{id}/instances/register (rate-limited, turnstile-optional)"
        >
          <Toggle checked={isPublic} onChange={setIsPublic} label="public" />
        </Field>
        {error && <ErrorNote text={error} />}
        <div className="flex justify-end gap-2">
          <Btn variant="white" onClick={onClose}>
            cancel
          </Btn>
          <Btn type="submit" disabled={createM.isPending}>
            {createM.isPending ? "creating…" : "create it ⚡"}
          </Btn>
        </div>
      </form>
    </Modal>
  );
}
