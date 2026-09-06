import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { ApiError, api, errMessage } from "../api/client";
import { principalOf, setSession } from "../auth/token";
import type { AppView } from "../api/types";
import { Btn, Card, ErrorNote, Input } from "../components/ui";

export default function Login() {
  const navigate = useNavigate();
  const [token, setTokenVal] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    const t = token.trim();
    if (!t) {
      setError("paste your token first");
      return;
    }
    setBusy(true);
    setError(null);
    const principal = principalOf(t);
    try {
      if (principal === "app") {
        // Resolves @app to a concrete id we remember for the session.
        const app = await api<AppView>("GET", "/apps/@app", undefined, {
          token: t,
        });
        setSession({ token: t, principal: "app", appId: app.id });
      } else {
        // The admin token validates against the cheapest admin-only read.
        await api("GET", "/apps", undefined, { token: t });
        setSession({ token: t, principal: "admin" });
      }
      navigate("/", { replace: true });
    } catch (err) {
      const rejected =
        err instanceof ApiError && (err.status === 401 || err.status === 403);
      setError(
        rejected
          ? principal === "app"
            ? "that app token was rejected — paste the pat_ token for your app"
            : "that token was rejected — it must match PUSHPORT_ADMIN_TOKEN on the server"
          : errMessage(err),
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex min-h-dvh flex-col items-center justify-center px-4 py-10">
      <div className="mb-8 text-center">
        <img
          src={`${import.meta.env.BASE_URL}logo-mark.svg`}
          alt=""
          aria-hidden
          className="mx-auto mb-6 h-20 w-20"
        />
        <h1 className="font-display inline-block -rotate-2 text-5xl sm:text-7xl">
          pushport
          <span className="ml-3 inline-block rotate-3 bg-ink px-3 text-canvas shadow-[6px_6px_0_0_var(--color-pink)] dark:bg-[#23232b]">
            console
          </span>
        </h1>
        <p className="mt-4 font-mono text-sm font-bold text-ink/60">
          ⚡ admins & app maintainers ⚡ no boring dashboards allowed ⚡
        </p>
      </div>

      <Card className="w-full max-w-md p-6 sm:p-8">
        <form onSubmit={submit} className="flex flex-col gap-4">
          <label className="block">
            <span className="mb-1 block text-xs font-bold tracking-wide uppercase">
              token
            </span>
            <Input
              type="password"
              autoFocus
              value={token}
              onChange={(e) => setTokenVal(e.target.value)}
              placeholder="pushport token"
              className="font-mono"
              aria-label="Admin or app token"
            />
          </label>

          {error && <ErrorNote text={error} />}

          <Btn type="submit" disabled={busy} className="text-lg">
            {busy ? "checking…" : "let me in →"}
          </Btn>
        </form>

        <p className="mt-5 border-t-[3px] border-dashed border-ink/30 pt-4 font-mono text-xs leading-relaxed text-ink/60">
          Admins use the server's{" "}
          <span className="font-bold">PUSHPORT_ADMIN_TOKEN</span>; app
          maintainers use their app token (
          <span className="font-bold">pat_</span>). Either stays in this browser
          only and never reaches anywhere but your pushport server.
        </p>
      </Card>
    </div>
  );
}
