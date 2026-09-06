import {
  useEffect,
  useRef,
  useState,
  type ButtonHTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes,
} from "react";

// Palette lives in index.css @theme.

type Tone =
  | "canvas"
  | "white"
  | "paper"
  | "pink"
  | "cyan"
  | "lime"
  | "grape"
  | "tang"
  | "ink";

// Accents keep text-[#111] in both themes: it equals --color-ink in light, so
// light mode is unchanged and they need no dark: variant.
const bg: Record<Tone, string> = {
  canvas: "bg-canvas text-[#111]",
  white: "bg-white text-[#111] dark:bg-paper dark:text-ink",
  paper: "bg-paper",
  pink: "bg-pink text-[#111]",
  cyan: "bg-cyan text-[#111]",
  lime: "bg-lime text-[#111]",
  grape: "bg-grape text-[#111]",
  tang: "bg-tang text-[#111]",
  ink: "bg-ink text-canvas dark:bg-[#23232b] dark:text-ink",
};

type BtnVariant =
  | "primary"
  | "white"
  | "pink"
  | "cyan"
  | "lime"
  | "grape"
  | "tang";

const btnBg: Record<BtnVariant, string> = {
  primary: "bg-canvas text-[#111]",
  white: "bg-white text-[#111] dark:bg-paper dark:text-ink",
  pink: "bg-pink text-[#111]",
  cyan: "bg-cyan text-[#111]",
  lime: "bg-lime text-[#111]",
  grape: "bg-grape text-[#111]",
  tang: "bg-tang text-[#111]",
};

export function Btn({
  variant = "primary",
  className = "",
  type = "button",
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: BtnVariant }) {
  return (
    <button
      type={type}
      {...rest}
      className={`nb nb-hover nb-press rounded-lg px-4 py-2 font-bold disabled:cursor-not-allowed disabled:opacity-50 ${btnBg[variant]} ${className}`}
    />
  );
}

export function Card({
  className = "",
  children,
}: {
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={`nb rounded-xl bg-white dark:bg-paper ${className}`}>
      {children}
    </div>
  );
}

export function Section({
  title,
  tone = "canvas",
  aside,
  className = "",
  children,
}: {
  title: ReactNode;
  tone?: Tone;
  aside?: ReactNode;
  className?: string;
  children: ReactNode;
}) {
  return (
    <Card className={`overflow-hidden ${className}`}>
      <div
        className={`flex flex-wrap items-center justify-between gap-2 border-b-[3px] border-ink px-4 py-2.5 ${bg[tone]}`}
      >
        <h2 className="font-display text-sm tracking-wide uppercase">
          {title}
        </h2>
        {aside}
      </div>
      <div className="p-4 sm:p-5">{children}</div>
    </Card>
  );
}

export function StatCard({
  label,
  value,
  tone = "canvas",
  hint,
}: {
  label: string;
  value: ReactNode;
  tone?: Tone;
  hint?: string;
}) {
  return (
    <Card className="nb-hover overflow-hidden">
      <div
        className={`border-b-[3px] border-ink px-4 py-2 font-display text-sm tracking-wide uppercase ${bg[tone]}`}
      >
        {label}
      </div>
      <div className="px-4 pt-4 pb-1 font-mono text-4xl font-bold">{value}</div>
      {hint && (
        <div className="px-4 pb-3 text-xs font-bold tracking-wide text-ink/45 uppercase">
          {hint}
        </div>
      )}
    </Card>
  );
}

export function Badge({
  tone = "white",
  tilt = false,
  className = "",
  children,
}: {
  tone?: Tone;
  tilt?: boolean;
  className?: string;
  children: ReactNode;
}) {
  return (
    <span
      className={`nb-sm inline-block rounded-full px-2.5 py-0.5 text-xs leading-5 font-bold whitespace-nowrap ${bg[tone]} ${tilt ? "-rotate-2" : ""} ${className}`}
    >
      {children}
    </span>
  );
}

const fieldCls =
  "w-full rounded-md border-[2px] border-ink bg-white dark:bg-paper px-3 py-2 font-medium placeholder:text-ink/35 focus:shadow-[4px_4px_0_0_var(--color-ink)] focus:outline-none";

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  const { className = "", ...rest } = props;
  return <input {...rest} className={`${fieldCls} ${className}`} />;
}

export function Textarea(props: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  const { className = "", ...rest } = props;
  return (
    <textarea
      {...rest}
      className={`${fieldCls} font-mono text-sm ${className}`}
    />
  );
}

export function Select(props: SelectHTMLAttributes<HTMLSelectElement>) {
  const { className = "", children, ...rest } = props;
  return (
    <select {...rest} className={`${fieldCls} cursor-pointer ${className}`}>
      {children}
    </select>
  );
}

export function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs font-bold tracking-wide uppercase">
        {label}
      </span>
      {children}
      {hint && (
        <span className="mt-1 block text-xs font-medium text-ink/55">
          {hint}
        </span>
      )}
    </label>
  );
}

export function Toggle({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
}) {
  return (
    <button
      type="button"
      aria-pressed={checked}
      onClick={() => onChange(!checked)}
      className={`nb-sm nb-press inline-flex cursor-pointer items-center gap-2 rounded-full px-3 py-1.5 text-sm font-bold ${checked ? "bg-lime text-[#111]" : "bg-white dark:bg-paper dark:text-ink"}`}
    >
      <span
        aria-hidden
        className={`inline-block h-3.5 w-3.5 rounded-full border-2 border-ink ${checked ? "bg-ink dark:bg-[#111]" : "bg-white dark:bg-[#23232b]"}`}
      />
      {label}: {checked ? "ON" : "OFF"}
    </button>
  );
}

export function Modal({
  open,
  onClose,
  title,
  children,
}: {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  children: ReactNode;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-ink/60 p-4 dark:bg-black/70"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label={typeof title === "string" ? title : undefined}
        className="nb animate-[pop-in_.16s_ease-out] max-h-[88dvh] w-full max-w-lg overflow-y-auto rounded-xl bg-paper shadow-[9px_9px_0_0_var(--color-ink)]"
      >
        <div className="sticky top-0 flex items-center justify-between gap-3 border-b-[3px] border-ink bg-canvas px-5 py-3 text-[#111]">
          <h3 className="font-display text-lg">{title}</h3>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="nb-sm nb-press cursor-pointer rounded-md bg-white px-2.5 py-0.5 font-bold hover:bg-pink dark:bg-paper dark:text-ink hover:text-[#111]"
          >
            ✕
          </button>
        </div>
        <div className="px-5 py-4">{children}</div>
      </div>
    </div>
  );
}

export function Spinner({ label = "loading…" }: { label?: string }) {
  return (
    <div
      className="flex items-center gap-3 font-bold text-ink/60"
      role="status"
    >
      <span
        aria-hidden
        className="inline-block h-5 w-5 animate-spin rounded-full border-[3px] border-ink border-t-transparent"
      />
      {label}
    </div>
  );
}

export function EmptyState({
  title,
  sub,
  action,
}: {
  title: string;
  sub?: string;
  action?: ReactNode;
}) {
  return (
    <div className="rounded-xl border-[3px] border-dashed border-ink/40 bg-white/70 px-6 py-10 text-center dark:bg-paper/70">
      <p className="font-display text-xl">{title}</p>
      {sub && <p className="mt-1 text-sm font-medium text-ink/60">{sub}</p>}
      {action && <div className="mt-4 flex justify-center">{action}</div>}
    </div>
  );
}

export function ErrorNote({ text }: { text: string }) {
  return (
    <div className="nb-sm animate-[shake_.3s_ease-in-out] rounded-lg bg-pink px-3 py-2 text-sm font-bold text-[#111]">
      {text}
    </div>
  );
}

export function CopyBtn({
  text,
  label = "copy",
}: {
  text: string;
  label?: string;
}) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  return (
    <button
      type="button"
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(text);
        } catch {
          // Clipboard API can be denied.
        }
        setCopied(true);
        if (timer.current) clearTimeout(timer.current);
        timer.current = setTimeout(() => setCopied(false), 1500);
      }}
      className={`nb-sm nb-press shrink-0 cursor-pointer rounded-md px-2.5 py-1 text-xs font-bold ${copied ? "bg-lime text-[#111]" : "bg-white hover:bg-cyan hover:text-[#111] dark:bg-paper dark:text-ink"}`}
    >
      {copied ? "copied!" : label}
    </button>
  );
}

export function TokenReveal({ token, kind }: { token: string; kind: string }) {
  return (
    <div className="rounded-xl border-[3px] border-dashed border-ink bg-white p-4 dark:bg-paper">
      <Badge tone="pink" tilt>
        {kind} — shown ONCE
      </Badge>
      <div className="mt-3 flex items-stretch gap-2">
        <code className="min-w-0 flex-1 overflow-x-auto rounded-md border-[2px] border-ink bg-canvas px-3 py-2 font-mono text-sm break-all text-[#111]">
          {token}
        </code>
        <div className="flex items-center">
          <CopyBtn text={token} label="copy token" />
        </div>
      </div>
      <p className="mt-2 text-sm font-medium text-ink/65">
        Copy it somewhere safe — after this screen it's gone forever.
      </p>
    </div>
  );
}
