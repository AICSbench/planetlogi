import type { DiagramSpec } from "@planetlogi/api-client";

export function DiagramRenderer({ diagram }: { diagram?: DiagramSpec }) {
  if (!diagram) return null;

  if (diagram.type === "number_line") {
    const start = Number(diagram.params.start ?? 0);
    const jump = Number(diagram.params.jump ?? 0);
    const end = Number(diagram.params.end ?? start + jump);
    return (
      <svg
        viewBox="0 0 360 90"
        className="w-full max-w-md rounded-xl border border-slate-200 bg-slate-50 p-2"
        role="img"
        aria-label={`Number line from ${start} jumping ${jump} to ${end}`}
      >
        <line x1="30" y1="60" x2="330" y2="60" stroke="#334155" strokeWidth="3" />
        <path
          d="M 90 58 Q 180 8 270 58"
          fill="none"
          stroke="#6366F1"
          strokeWidth="3"
          strokeDasharray="6 4"
        />
        <text x="180" y="24" textAnchor="middle" fontSize="14" fontWeight="700" fill="#4F46E5">
          +{jump}
        </text>
        <circle cx="90" cy="60" r="6" fill="#0EA5E9" />
        <text x="90" y="82" textAnchor="middle" fontSize="14" fontWeight="700" fill="#0F172A">
          {start}
        </text>
        <circle cx="270" cy="60" r="6" fill="#10B981" />
        <text x="270" y="82" textAnchor="middle" fontSize="14" fontWeight="700" fill="#0F172A">
          {end}
        </text>
      </svg>
    );
  }

  if (diagram.type === "bar_model") {
    const bars = (diagram.params.bars as Array<{ label: string; units: number; value: string }>) ?? [];
    const maxUnits = Math.max(1, ...bars.map((b) => b.units));
    return (
      <div className="w-full max-w-md space-y-2 rounded-xl border border-slate-200 bg-slate-50 p-3">
        {bars.map((bar, i) => (
          <div key={i} className="flex items-center gap-2">
            <span className="w-14 text-xs font-bold text-slate-700">{bar.label}</span>
            <div
              className="flex h-8 items-center justify-center rounded-lg border-2 border-indigo-500 bg-indigo-100 text-xs font-extrabold text-indigo-900"
              style={{ width: `${Math.max(25, Math.round((bar.units / maxUnits) * 100))}%` }}
            >
              {bar.value}
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (diagram.type === "fraction_bar") {
    const num = Math.max(0, Number(diagram.params.numerator ?? 1));
    const den = Math.max(1, Number(diagram.params.denominator ?? 4));
    return (
      <div className="flex w-full max-w-xs gap-1 rounded-xl border border-slate-200 bg-slate-50 p-2">
        {Array.from({ length: den }).map((_, i) => (
          <div
            key={i}
            className={`h-8 flex-1 rounded border border-slate-400 ${
              i < num ? "bg-[#6366F1]" : "bg-white"
            }`}
          />
        ))}
      </div>
    );
  }

  if (diagram.type === "rectangle_grid") {
    const length = Number(diagram.params.length ?? 6);
    const width = Number(diagram.params.width ?? 4);
    const unit = String(diagram.params.unit ?? "cm");
    return (
      <svg
        viewBox="0 0 260 130"
        className="w-full max-w-xs rounded-xl border border-slate-200 bg-slate-50 p-2"
        role="img"
        aria-label={`Rectangle ${length} ${unit} by ${width} ${unit}`}
      >
        <rect x="45" y="25" width="160" height="80" fill="#E0E7FF" stroke="#4F46E5" strokeWidth="3" />
        <text x="125" y="18" textAnchor="middle" fontSize="13" fontWeight="700" fill="#0F172A">
          {length} {unit}
        </text>
        <text x="225" y="70" textAnchor="middle" fontSize="13" fontWeight="700" fill="#0F172A">
          {width} {unit}
        </text>
      </svg>
    );
  }

  return null;
}
