import { useState } from "react";
import { useTranslation } from "react-i18next";

interface BarRow {
  id: number;
  label: string;
  units: number;
  shaded: number;
  braceText: string;
}

const BAR_COLORS = ["#6366F1", "#10B981", "#F59E0B", "#0EA5E9"];

export function BarModelBuilder() {
  const { t } = useTranslation("player");
  const [bars, setBars] = useState<BarRow[]>([
    { id: 1, label: "A", units: 4, shaded: 0, braceText: "?" },
    { id: 2, label: "B", units: 6, shaded: 2, braceText: "24" },
  ]);

  function updateBar(id: number, patch: Partial<BarRow>) {
    setBars((prev) =>
      prev.map((b) => {
        if (b.id !== id) return b;
        const next = { ...b, ...patch };
        next.units = Math.min(20, Math.max(1, next.units));
        next.shaded = Math.min(next.units, Math.max(0, next.shaded));
        return next;
      })
    );
  }

  return (
    <div className="space-y-3">
      {bars.map((bar, idx) => (
        <div key={bar.id} className="rounded-xl border border-slate-300 bg-white p-2.5 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <input
              type="text"
              value={bar.label}
              onChange={(e) => updateBar(bar.id, { label: e.target.value })}
              aria-label="Bar label"
              className="w-20 rounded-lg border border-slate-300 px-2 py-1 text-xs font-bold text-slate-900"
            />
            <div className="flex items-center gap-1 text-xs font-semibold text-slate-700">
              <span>{t("tools.units")}:</span>
              <button
                type="button"
                onClick={() => updateBar(bar.id, { units: bar.units - 1 })}
                className="h-7 w-7 rounded bg-slate-200 font-bold cursor-pointer"
              >
                −
              </button>
              <span className="w-6 text-center planetlogi-tabular">{bar.units}</span>
              <button
                type="button"
                onClick={() => updateBar(bar.id, { units: bar.units + 1 })}
                className="h-7 w-7 rounded bg-slate-200 font-bold cursor-pointer"
              >
                +
              </button>
            </div>
            <input
              type="text"
              value={bar.braceText}
              onChange={(e) => updateBar(bar.id, { braceText: e.target.value })}
              placeholder="?"
              aria-label="Bracket value"
              className="ml-auto w-16 rounded-lg border border-slate-300 px-2 py-1 text-xs font-bold text-center text-indigo-700"
            />
          </div>

          <div className="flex gap-0.5 overflow-x-auto py-1">
            {Array.from({ length: bar.units }).map((_, uIdx) => {
              const isShaded = uIdx < bar.shaded;
              return (
                <button
                  key={uIdx}
                  type="button"
                  onClick={() =>
                    updateBar(bar.id, { shaded: uIdx + 1 === bar.shaded ? uIdx : uIdx + 1 })
                  }
                  className="h-7 min-w-5 flex-1 rounded border border-slate-400 transition-colors cursor-pointer"
                  style={{
                    backgroundColor: isShaded ? BAR_COLORS[idx % BAR_COLORS.length] : "#F1F5F9",
                  }}
                  title={`Unit ${uIdx + 1}`}
                />
              );
            })}
          </div>
        </div>
      ))}

      <div className="flex gap-2">
        {bars.length < 4 ? (
          <button
            type="button"
            onClick={() =>
              setBars((prev) => [
                ...prev,
                { id: Date.now(), label: `Bar ${prev.length + 1}`, units: 4, shaded: 0, braceText: "?" },
              ])
            }
            className="min-h-10 rounded-lg bg-[#6366F1] px-3 text-xs font-semibold text-white hover:bg-[#5558E6] cursor-pointer"
          >
            + {t("tools.addBar")}
          </button>
        ) : null}
        <button
          type="button"
          onClick={() =>
            setBars([
              { id: 1, label: "A", units: 4, shaded: 0, braceText: "?" },
              { id: 2, label: "B", units: 6, shaded: 0, braceText: "?" },
            ])
          }
          className="min-h-10 rounded-lg bg-slate-200 px-3 text-xs font-semibold text-slate-800 hover:bg-slate-300 cursor-pointer"
        >
          {t("tools.clear")}
        </button>
      </div>
    </div>
  );
}
