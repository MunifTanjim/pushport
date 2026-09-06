import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import { get } from "../api/client";
import type { AppUsagePlanView, AppView, StatsView } from "../api/types";
import {
  Badge,
  Card,
  EmptyState,
  Section,
  Spinner,
  StatCard,
} from "../components/ui";
import { fmtNum } from "../lib/format";

const tickerText =
  "⚡ admin console ⚡ all systems go ⚡ pushes being pushed ⚡ quotas being respected ⚡ ";

const barTones = [
  "bg-pink",
  "bg-cyan",
  "bg-lime",
  "bg-grape",
  "bg-tang",
  "bg-canvas",
];

export default function Overview() {
  const statsQ = useQuery({
    queryKey: ["stats"],
    queryFn: () => get<StatsView>("/stats"),
  });
  const appsQ = useQuery({
    queryKey: ["apps"],
    queryFn: () => get<AppView[]>("/apps"),
  });
  const plansQ = useQuery({
    queryKey: ["app-plans"],
    queryFn: () => get<AppUsagePlanView[]>("/usage-plans"),
  });

  const stats = statsQ.data;
  const apps = appsQ.data ?? [];
  const plans = plansQ.data ?? [];

  const byApp = stats
    ? Object.entries(stats.pushes_by_app)
        .map(([id, count]) => ({
          id,
          count,
          name: apps.find((a) => a.id === id)?.name ?? id,
        }))
        .sort((a, b) => b.count - a.count)
    : [];
  const max = byApp[0]?.count ?? 0;

  return (
    <div className="flex flex-col gap-6">
      <div className="overflow-hidden rounded-lg border-[3px] border-ink bg-ink text-canvas dark:bg-[#23232b]">
        <div className="flex animate-[marquee_36s_linear_infinite] whitespace-nowrap py-1 font-mono text-xs font-bold">
          <span className="pr-8">{tickerText.repeat(4)}</span>
          <span aria-hidden className="pr-8">
            {tickerText.repeat(4)}
          </span>
        </div>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="font-display text-3xl sm:text-4xl">
          today at a glance
          {stats && (
            <Badge tone="cyan" tilt className="ml-3 align-middle">
              {stats.date}
            </Badge>
          )}
        </h1>
        <div className="flex gap-2">
          <Link
            to="/apps?create=1"
            className="nb nb-hover nb-press rounded-lg bg-lime px-4 py-2 font-bold text-[#111]"
          >
            + new app
          </Link>
          <Link
            to="/plans"
            className="nb nb-hover nb-press rounded-lg bg-white px-4 py-2 font-bold dark:bg-paper"
          >
            usage plans →
          </Link>
        </div>
      </div>

      {statsQ.isError ? (
        <EmptyState
          title="stats wouldn't load"
          sub="is the server feeling okay?"
        />
      ) : !stats ? (
        <Spinner />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
            <StatCard
              label="apps"
              value={stats.total_apps}
              tone="pink"
              hint="registered tenants"
            />
            <StatCard
              label="instances"
              value={stats.total_instances}
              tone="cyan"
              hint="instances registered"
            />
            <StatCard
              label="usage plans"
              value={plans.length}
              tone="grape"
              hint="app plans"
            />
            <StatCard
              label="pushes today"
              value={fmtNum(stats.total_pushes)}
              tone="lime"
              hint="counted durably"
            />
          </div>

          <Section
            title="pushes by app — today"
            tone="canvas"
            aside={
              <Badge tone="ink">
                top {Math.min(8, Math.max(1, byApp.length))}
              </Badge>
            }
          >
            {byApp.length === 0 ? (
              <EmptyState
                title="zero pushes today"
                sub="either suspiciously peaceful, or nobody wired up their senders yet"
              />
            ) : (
              <ol className="flex flex-col gap-3">
                {byApp.slice(0, 8).map((a, i) => (
                  <li key={a.id} className="flex items-center gap-3">
                    <Link
                      to={`/apps/${a.id}`}
                      className="w-44 shrink-0 truncate font-bold hover:underline sm:w-56"
                      title={a.name}
                    >
                      {a.name}
                    </Link>
                    <div className="min-w-0 flex-1">
                      <div
                        className={`h-7 rounded-md border-[3px] border-ink ${barTones[i % barTones.length]}`}
                        style={{
                          width: `${Math.max(4, (a.count / max) * 100)}%`,
                        }}
                        role="img"
                        aria-label={`${a.name}: ${a.count} pushes`}
                      />
                    </div>
                    <span className="w-16 shrink-0 text-right font-mono text-sm font-bold">
                      {fmtNum(a.count)}
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

          <Section title="jump back in" tone="paper">
            <div className="flex flex-wrap gap-3">
              {apps.slice(0, 6).map((a) => (
                <Link key={a.id} to={`/apps/${a.id}`}>
                  <Card className="nb-hover nb-press px-4 py-2 font-bold">
                    {a.name} →
                  </Card>
                </Link>
              ))}
              {apps.length > 6 && (
                <Link
                  to="/apps"
                  className="self-center font-bold hover:underline"
                >
                  all {apps.length} apps →
                </Link>
              )}
              {apps.length === 0 && (
                <Link to="/apps?create=1" className="font-bold hover:underline">
                  create your first app →
                </Link>
              )}
            </div>
          </Section>
        </>
      )}
    </div>
  );
}
