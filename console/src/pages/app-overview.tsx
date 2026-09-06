import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import { errMessage, get } from "../api/client";
import type {
  AppStatsView,
  AppUsagePlanView,
  AppView,
  InstanceView,
  TransportsView,
  TurnstileView,
} from "../api/types";
import {
  Badge,
  Btn,
  Card,
  CopyBtn,
  EmptyState,
  Section,
  Spinner,
  StatCard,
} from "../components/ui";
import { fmtId, fmtLimit, fmtNum } from "../lib/format";

const TRANSPORTS: { key: string; label: string }[] = [
  { key: "apns", label: "🍎 APNs" },
  { key: "fcm", label: "🤖 FCM" },
  { key: "webpush", label: "🌐 WebPush" },
];

// Everything reads through @app; no admin-only endpoints (which would 401 for app tokens).
export default function AppOverview() {
  const appQ = useQuery({
    queryKey: ["app", "@app"],
    queryFn: () => get<AppView>("/apps/@app"),
  });
  const planQ = useQuery({
    queryKey: ["app-eff-plan", "@app"],
    queryFn: () => get<AppUsagePlanView>("/apps/@app/usage-plan"),
  });
  const instQ = useQuery({
    queryKey: ["instances", "@app"],
    queryFn: () => get<InstanceView[]>("/apps/@app/instances"),
  });
  const credsQ = useQuery({
    queryKey: ["creds", "@app"],
    queryFn: () => get<TransportsView>("/apps/@app/creds"),
  });
  const tsQ = useQuery({
    queryKey: ["turnstile", "@app"],
    queryFn: () => get<TurnstileView>("/apps/@app/turnstile"),
  });
  const statsQ = useQuery({
    queryKey: ["app-stats", "@app"],
    queryFn: () => get<AppStatsView>("/apps/@app/stats"),
  });

  if (appQ.isPending) return <Spinner />;
  if (appQ.isError)
    return (
      <EmptyState title="couldn't load your app" sub={errMessage(appQ.error)} />
    );

  const app = appQ.data;
  const plan = planQ.data;
  const configured = new Set(credsQ.data?.transports ?? []);
  const instances = instQ.data ?? [];
  const stats = statsQ.data;
  const labelFor = (id: string) =>
    instances.find((i) => i.id === id)?.label || fmtId(id, 10);

  const byInstance = stats
    ? Object.entries(stats.pushes_by_instance)
        .map(([id, count]) => ({ id, count, label: labelFor(id) }))
        .sort((a, b) => b.count - a.count)
    : [];
  const maxCount = byInstance[0]?.count ?? 0;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="font-display text-3xl sm:text-4xl">{app.name}</h1>
            {app.is_public ? (
              <Badge tone="pink" tilt>
                🌍 public
              </Badge>
            ) : (
              <Badge tone="white">🔒 private</Badge>
            )}
          </div>
          <span className="mt-2 flex items-center gap-2 font-mono text-sm text-ink/55">
            <span title={app.id}>{fmtId(app.id, 14)}</span>
            <CopyBtn text={app.id} label="copy id" />
          </span>
        </div>
        <Link to={`/apps/${app.id}`}>
          <Btn variant="lime">manage settings →</Btn>
        </Link>
      </div>

      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard
          label="instances"
          value={instances.length}
          tone="grape"
          hint="instances registered"
        />
        <StatCard
          label="pushes today"
          value={stats ? fmtNum(stats.total_pushes) : "—"}
          tone="cyan"
          hint="counted durably"
        />
        <StatCard
          label="per minute"
          value={plan ? fmtLimit(plan.push_per_min) : "—"}
          tone="pink"
          hint="push rate limit"
        />
        <StatCard
          label="daily quota"
          value={plan ? fmtLimit(plan.push_daily_quota) : "—"}
          tone="lime"
          hint="pushes per day"
        />
      </div>

      <Section
        title="pushes by instance — today"
        tone="grape"
        aside={stats && <Badge tone="ink">{stats.date}</Badge>}
      >
        {statsQ.isError ? (
          <EmptyState
            title="stats wouldn't load"
            sub={errMessage(statsQ.error)}
          />
        ) : !stats ? (
          <Spinner />
        ) : byInstance.length === 0 ? (
          <EmptyState
            title="zero pushes today"
            sub="either a quiet day, or no senders have fired yet"
          />
        ) : (
          <ol className="flex flex-col gap-3">
            {byInstance.slice(0, 8).map((i) => (
              <li key={i.id} className="flex items-center gap-3">
                <span
                  className="w-40 shrink-0 truncate font-bold sm:w-52"
                  title={i.label}
                >
                  {i.label}
                </span>
                <div className="min-w-0 flex-1">
                  <div
                    className="h-7 rounded-md border-[3px] border-ink bg-grape"
                    style={{
                      width: `${Math.max(4, (i.count / maxCount) * 100)}%`,
                    }}
                    role="img"
                    aria-label={`${i.label}: ${i.count} pushes`}
                  />
                </div>
                <span className="w-16 shrink-0 text-right font-mono text-sm font-bold">
                  {fmtNum(i.count)}
                </span>
              </li>
            ))}
          </ol>
        )}
        <p className="mt-4 font-mono text-xs text-ink/50">
          counts are durable on the server; the last quota flush (≤30s) may
          still be in flight
        </p>
      </Section>

      <Section
        title="your plan"
        tone="canvas"
        aside={plan && <Badge tone="grape">{plan.name}</Badge>}
      >
        {plan ? (
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-display">{plan.name}</span>
            <Badge tone="lime">{fmtLimit(plan.push_per_min)}/min</Badge>
            <Badge tone="cyan">{fmtLimit(plan.push_daily_quota)} daily</Badge>
          </div>
        ) : (
          <Spinner />
        )}
        <p className="mt-3 font-mono text-xs text-ink/50">
          plans are set by the admin; reach out if you need a bump
        </p>
      </Section>

      <Section title="delivery transports" tone="paper">
        <ul className="flex flex-wrap gap-3">
          {TRANSPORTS.map((t) => (
            <li key={t.key}>
              <Card className="flex items-center gap-2 px-4 py-2">
                <span className="font-bold">{t.label}</span>
                {configured.has(t.key) ? (
                  <Badge tone="lime" tilt>
                    configured
                  </Badge>
                ) : (
                  <Badge tone="white">not set</Badge>
                )}
              </Card>
            </li>
          ))}
        </ul>
        <p className="mt-3 font-mono text-xs text-ink/50">
          configure credentials from{" "}
          <Link to={`/apps/${app.id}`} className="font-bold hover:underline">
            settings
          </Link>
        </p>
      </Section>

      <Section
        title="self-registration"
        tone="cyan"
        aside={
          !app.is_public ? (
            <Badge tone="white">private</Badge>
          ) : tsQ.data?.configured ? (
            <Badge tone="lime" tilt>
              turnstile on
            </Badge>
          ) : (
            <Badge tone="pink">turnstile off</Badge>
          )
        }
      >
        <p className="text-sm font-medium text-ink/70">
          {app.is_public
            ? "Anyone can register an instance at your public registration page."
            : "This app is private — instances are minted from settings."}
          {app.is_public && !tsQ.data?.configured && (
            <> Consider turning on Turnstile to gate the page.</>
          )}
        </p>
      </Section>
    </div>
  );
}
