import {
  createContext,
  useCallback,
  useContext,
  useRef,
  useState,
  type ReactNode,
} from "react";

type ToastKind = "success" | "error" | "info";

interface ToastItem {
  id: number;
  kind: ToastKind;
  text: string;
}

type ToastFn = (kind: ToastKind, text: string) => void;

const ToastCtx = createContext<ToastFn | null>(null);

const tone: Record<ToastKind, string> = {
  success: "bg-lime text-[#111]",
  error: "bg-pink text-[#111]",
  info: "bg-cyan text-[#111]",
};

const icon: Record<ToastKind, string> = {
  success: "✓",
  error: "✗",
  info: "ℹ",
};

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const nextId = useRef(1);

  const toast = useCallback((kind: ToastKind, text: string) => {
    const id = nextId.current++;
    setItems((ts) => [...ts, { id, kind, text }]);
    setTimeout(() => {
      setItems((ts) => ts.filter((t) => t.id !== id));
    }, 4200);
  }, []);

  return (
    <ToastCtx.Provider value={toast}>
      {children}
      <div className="pointer-events-none fixed right-4 bottom-4 z-[100] flex w-[min(92vw,26rem)] flex-col gap-3">
        {items.map((t) => (
          <div
            key={t.id}
            className={`nb animate-[toast-in_.18s_ease-out] flex items-start gap-2 rounded-lg px-4 py-3 font-bold ${tone[t.kind]}`}
          >
            <span aria-hidden>{icon[t.kind]}</span>
            <span className="min-w-0 break-words">{t.text}</span>
          </div>
        ))}
      </div>
    </ToastCtx.Provider>
  );
}

export function useToast(): ToastFn {
  const ctx = useContext(ToastCtx);
  if (!ctx) throw new Error("useToast must be used inside <ToastProvider>");
  return ctx;
}
