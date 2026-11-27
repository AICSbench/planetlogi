export type CurrencyKind = "SP" | "AC" | "XP" | "SCROLL";

export interface CurrencyPillProps {
  kind: CurrencyKind;
  value: string | number;
  label?: string;
}

const STYLE_MAP: Record<CurrencyKind, { icon: string; tone: string }> = {
  SP: { icon: "★", tone: "bg-amber-400/15 text-amber-300 border-amber-400/35" },
  AC: { icon: "◉", tone: "bg-cyan-400/15 text-cyan-300 border-cyan-400/35" },
  XP: { icon: "⚡", tone: "bg-indigo-400/20 text-indigo-200 border-indigo-400/40" },
  SCROLL: { icon: "✦", tone: "bg-emerald-400/15 text-emerald-300 border-emerald-400/35" },
};

export function CurrencyPill({ kind, value, label }: CurrencyPillProps) {
  const style = STYLE_MAP[kind];
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-sm font-bold planetlogi-tabular ${style.tone}`}
    >
      <span aria-hidden="true">{style.icon}</span>
      <span>{value}</span>
      {label ? <span className="text-xs font-medium opacity-85">{label}</span> : null}
    </span>
  );
}
