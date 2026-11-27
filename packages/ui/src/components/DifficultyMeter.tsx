export interface DifficultyMeterProps {
  level: 1 | 2 | 3 | 4 | 5;
  label?: string;
}

export function DifficultyMeter({ level, label }: DifficultyMeterProps) {
  return (
    <span
      className="inline-flex items-center gap-0.5"
      role="img"
      aria-label={label ?? `Difficulty ${level} of 5`}
    >
      {[1, 2, 3, 4, 5].map((idx) => (
        <span
          key={idx}
          aria-hidden="true"
          className={idx <= level ? "text-[#F97316]" : "text-slate-600"}
        >
          ●
        </span>
      ))}
    </span>
  );
}
