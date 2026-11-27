export type ProficiencyTier = "MASTERY" | "COMPETENT" | "BEGINNING" | "INCOMPLETE";

export interface ProficiencyPillProps {
  tier: ProficiencyTier;
  label: string;
  pct?: number;
  attemptedCount?: number;
  targetCount?: number;
}

const TIER_MAP: Record<ProficiencyTier, { icon: string; cls: string }> = {
  MASTERY: { icon: "✓", cls: "bg-emerald-500/15 text-emerald-300 border-emerald-400/40" },
  COMPETENT: { icon: "◆", cls: "bg-blue-500/15 text-blue-300 border-blue-400/40" },
  BEGINNING: { icon: "▲", cls: "bg-amber-500/15 text-amber-300 border-amber-400/40" },
  INCOMPLETE: { icon: "○", cls: "bg-slate-500/20 text-slate-300 border-slate-400/35" },
};

export function ProficiencyPill({
  tier,
  label,
  pct,
  attemptedCount,
  targetCount = 10,
}: ProficiencyPillProps) {
  const cfg = TIER_MAP[tier];
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold planetlogi-tabular ${cfg.cls}`}
    >
      <span aria-hidden="true">{cfg.icon}</span>
      <span>{label}</span>
      {pct !== undefined && tier !== "INCOMPLETE" ? <span>({pct}%)</span> : null}
      {tier === "INCOMPLETE" && attemptedCount !== undefined ? (
        <span>
          ({attemptedCount} / {targetCount})
        </span>
      ) : null}
    </span>
  );
}
