import { useState } from "react";
import { useTranslation } from "react-i18next";

const SYMBOLS = ["+", "−", "×", "÷", "=", "(", ")", "<", ">", "/", "cm", "m", "kg", "%"] as const;

export function EquationPad() {
  const { t } = useTranslation("player");
  const [lines, setLines] = useState<string[]>([""]);
  const [activeIdx, setActiveIdx] = useState(0);

  function insertSymbol(sym: string) {
    setLines((prev) =>
      prev.map((line, i) => (i === activeIdx ? `${line}${line.endsWith(" ") || !line ? "" : " "}${sym} ` : line))
    );
  }

  return (
    <div className="space-y-2.5">
      <div className="flex flex-wrap gap-1">
        {SYMBOLS.map((sym) => (
          <button
            key={sym}
            type="button"
            onClick={() => insertSymbol(sym)}
            className="min-h-10 min-w-10 rounded-lg border border-slate-300 bg-white px-2 text-sm font-bold text-slate-800 hover:bg-indigo-50 cursor-pointer"
          >
            {sym}
          </button>
        ))}
      </div>

      <div className="space-y-1.5">
        {lines.map((line, idx) => (
          <div key={idx} className="flex items-center gap-2">
            <span className="w-6 text-xs font-bold text-slate-500">{idx + 1}.</span>
            <input
              type="text"
              value={line}
              onFocus={() => setActiveIdx(idx)}
              onChange={(e) =>
                setLines((prev) => prev.map((l, i) => (i === idx ? e.target.value : l)))
              }
              placeholder="e.g. 12 + 8 = 20"
              className="min-h-10 flex-1 rounded-lg border border-slate-300 bg-white px-3 text-sm font-mono text-slate-900 focus:border-[#6366F1] focus:outline-none"
            />
          </div>
        ))}
      </div>

      <div className="flex gap-2">
        <button
          type="button"
          onClick={() => {
            setLines((prev) => [...prev, ""]);
            setActiveIdx(lines.length);
          }}
          className="min-h-10 rounded-lg bg-[#6366F1] px-3 text-xs font-semibold text-white hover:bg-[#5558E6] cursor-pointer"
        >
          + Step
        </button>
        <button
          type="button"
          onClick={() => {
            setLines([""]);
            setActiveIdx(0);
          }}
          className="min-h-10 rounded-lg bg-slate-200 px-3 text-xs font-semibold text-slate-800 hover:bg-slate-300 cursor-pointer"
        >
          {t("tools.clear")}
        </button>
      </div>
    </div>
  );
}
