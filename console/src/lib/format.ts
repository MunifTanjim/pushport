export function fmtDate(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

// Limits of 0 mean "unlimited" on the server.
export function fmtLimit(n: number): string {
  return n > 0 ? n.toLocaleString() : "∞";
}

export function fmtNum(n: number): string {
  return n.toLocaleString();
}

export function fmtId(id: string, head = 10): string {
  return id.length > head + 3 ? `${id.slice(0, head)}…${id.slice(-3)}` : id;
}
