const KEY = "pushport.session";

export type Principal = "admin" | "app";

export interface Session {
  token: string;
  principal: Principal;
  // Set for app maintainers: the resolved app id their pat_ token owns.
  appId?: string;
}

// Mirrors the CLI's rule for telling app tokens (pat_ prefix) from admin tokens.
export function principalOf(token: string): Principal {
  return token.startsWith("pat_") ? "app" : "admin";
}

export function getSession(): Session | null {
  const raw = sessionStorage.getItem(KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as Session;
  } catch {
    return null;
  }
}

export function setSession(session: Session) {
  sessionStorage.setItem(KEY, JSON.stringify(session));
}

export function clearSession() {
  sessionStorage.removeItem(KEY);
}

export function getToken(): string | null {
  return getSession()?.token ?? null;
}
