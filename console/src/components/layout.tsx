import { useEffect, useState } from "react";
import { NavLink, Outlet, useNavigate } from "react-router";
import { clearSession, getSession } from "../auth/token";
import {
  applyTheme,
  currentTheme,
  getStoredTheme,
  setTheme,
  systemTheme,
  type Theme,
} from "../lib/theme";
import { Btn } from "./ui";

const navLinkCls = ({ isActive }: { isActive: boolean }) =>
  `nb-sm rounded-full px-3.5 py-1.5 text-sm font-bold whitespace-nowrap transition-transform hover:-translate-y-0.5 ${
    isActive
      ? "bg-canvas text-ink dark:text-[#111]"
      : "bg-ink text-canvas dark:bg-[#23232b] dark:text-ink"
  }`;

function ThemeToggle() {
  const [theme, setThemeState] = useState<Theme>(() => currentTheme());

  // When the user hasn't picked a preference, keep following the OS live.
  useEffect(() => {
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => {
      if (getStoredTheme()) return;
      const next = systemTheme();
      applyTheme(next);
      setThemeState(next);
    };
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  const isDark = theme === "dark";
  return (
    <button
      type="button"
      aria-label={isDark ? "Switch to light theme" : "Switch to dark theme"}
      aria-pressed={isDark}
      title={isDark ? "Switch to light theme" : "Switch to dark theme"}
      onClick={() => {
        const next: Theme = isDark ? "light" : "dark";
        setTheme(next);
        setThemeState(next);
      }}
      className="nb-sm nb-press rounded-full bg-canvas px-3 py-1.5 text-sm font-bold text-[#111]"
    >
      <span aria-hidden>{isDark ? "☀" : "☾"}</span>
    </button>
  );
}

export function Layout() {
  const navigate = useNavigate();
  const session = getSession();
  const isAdmin = session?.principal === "admin";

  return (
    <div className="flex min-h-dvh flex-col">
      <header className="pp-topbar sticky top-0 z-40 border-b-[3px] border-ink bg-ink text-canvas shadow-[0_4px_0_0_rgba(17,17,17,0.25)] dark:bg-[#23232b]">
        <div className="mx-auto flex w-full max-w-6xl flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3 sm:px-6">
          <NavLink to="/" className="group flex items-center gap-2">
            <img
              src={`${import.meta.env.BASE_URL}logo-mark.svg`}
              alt=""
              aria-hidden
              className="h-8 w-8 transition-transform group-hover:animate-[wiggle_.4s_ease-in-out]"
            />
            <span className="font-display text-lg tracking-tight">
              pushport <span className="text-pink">console</span>
            </span>
          </NavLink>

          <nav className="ml-auto flex items-center gap-2" aria-label="Main">
            <NavLink to="/" end className={navLinkCls}>
              overview
            </NavLink>
            {isAdmin ? (
              <>
                <NavLink to="/apps" className={navLinkCls}>
                  apps
                </NavLink>
                <NavLink to="/plans" className={navLinkCls}>
                  plans
                </NavLink>
                <NavLink to="/settings" className={navLinkCls}>
                  settings
                </NavLink>
              </>
            ) : (
              session?.appId && (
                <NavLink to={`/apps/${session.appId}`} className={navLinkCls}>
                  settings
                </NavLink>
              )
            )}
          </nav>

          <ThemeToggle />

          <Btn
            variant="pink"
            className="px-3 py-1.5 text-sm"
            title="Forget the stored token"
            onClick={() => {
              clearSession();
              navigate("/login", { replace: true });
            }}
          >
            forget token
          </Btn>
        </div>
      </header>

      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-8 sm:px-6">
        <Outlet />
      </main>

      <footer className="border-t-[3px] border-ink bg-paper/80">
        <div className="mx-auto w-full max-w-6xl px-4 py-3 text-center font-mono text-xs text-ink/60 sm:px-6">
          pushport console · built for the people holding the keys
        </div>
      </footer>
    </div>
  );
}
