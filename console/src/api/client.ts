import { getToken } from "../auth/token";

// Typed envelope error from the server (code + message), or a transport/parse
// failure with a synthetic code.
export class ApiError extends Error {
  code: string;
  status: number;
  requestId: string | null;

  constructor(
    code: string,
    message: string,
    status: number,
    requestId: string | null,
  ) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
    this.requestId = requestId;
  }
}

export function errMessage(err: unknown): string {
  if (err instanceof ApiError) {
    return err.code === "unauthorized"
      ? "rejected — the admin token looks wrong"
      : err.message || err.code;
  }
  if (err instanceof Error && err.message === "Failed to fetch") {
    return "can't reach the server";
  }
  return err instanceof Error ? err.message : String(err);
}

interface Envelope<T> {
  request_id?: string;
  data?: T;
  error?: { code: string; message: string; status_code: number };
}

export async function api<T>(
  method: string,
  path: string,
  body?: unknown,
  opts?: { token?: string },
): Promise<T> {
  const headers: Record<string, string> = {};
  const token = opts?.token ?? getToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  let payload: string | undefined;
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    payload = JSON.stringify(body);
  }

  // In dev, /api is proxied same-origin to the local API (the Vite server strips /api).
  const base =
    import.meta.env.VITE_API_BASE_URL ??
    (import.meta.env.PROD ? "https://pushport.muniftanjim.dev" : "/api");
  const res = await fetch(`${base}${path}`, { method, headers, body: payload });

  if (res.status === 204) return null as T;

  let env: Envelope<T>;
  try {
    env = (await res.json()) as Envelope<T>;
  } catch {
    throw new ApiError(
      "bad_response",
      `unexpected response (${res.status})`,
      res.status,
      null,
    );
  }

  if (!res.ok || env.error) {
    const e = env.error;
    const apiErr = new ApiError(
      e?.code ?? "error",
      e?.message ?? `request failed (${res.status})`,
      res.status,
      env.request_id ?? null,
    );
    // 401 with a stored token means the session died (rotated/revoked); tell the app to reset.
    if (res.status === 401 && token && getToken()) {
      window.dispatchEvent(new CustomEvent("pp:unauthorized"));
    }
    throw apiErr;
  }
  return env.data as T;
}

export const get = <T>(path: string) => api<T>("GET", path);
export const post = <T>(path: string, body?: unknown) =>
  api<T>("POST", path, body);
export const put = <T>(path: string, body?: unknown) =>
  api<T>("PUT", path, body);
export const patch = <T>(path: string, body?: unknown) =>
  api<T>("PATCH", path, body);
export const del = <T>(path: string) => api<T>("DELETE", path);
