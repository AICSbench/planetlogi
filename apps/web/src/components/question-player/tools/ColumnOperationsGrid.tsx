import { useState } from "react";
import { useTranslation } from "react-i18next";

type Op = "+" | "−" | "×" | "÷";
const COLS = ["Th", "H", "T", "O"] as const;

function emptyRow() {
  return ["", "", "", ""];
}

// Pedagogical manual column workspace (NOT an auto-solving calculator!)
export function ColumnOperationsGrid() {
  const { t } = useTranslation("player");
  const [op, setOp] = useState<Op>("+");
  const [carry, setCarry] = useState<string[]>(emptyRow);
  const [rowA, setRowA] = useState<string[]>(emptyRow);
  const [rowB, setRowB] = useState<string[]>(emptyRow);
  const [result, setResult] = useState<string[]>(emptyRow);

  function updateCell(
    setter: React.Dispatch<React.SetStateAction<string[]>>,
    idx: number,
    val: string
  ) {
    const clean = val.replace(/[^0-9]/g, "").slice(-1);
    setter((prev) => prev.map((c, i) => (i === idx ? clean : c)));
  }

  function reset() {
    setCarry(emptyRow());
    setRowA(emptyRow());
    setRowB(emptyRow());
    setResult(emptyRow());
  }

  return (
    <div className="space-y-2.5">
      <div className="flex items-center justify-between gap-2">
        <div className="flex gap-1">
          {(["+", "−", "×", "÷"] as const).map((o) => (
            <button
              key={o}
              type="button"
              onClick={() => setOp(o)}
              className={`min-h-10 min-w-10 rounded-lg text-base font-extrabold cursor-pointer ${
                op === o ? "bg-[#6366F1] text-white" : "bg-slate-200 text-slate-800"
              }`}
            >
              {o}
            </button>
          ))}
        </div>
        <button
          type="button"
          onClick={reset}
          className="min-h-10 rounded-lg bg-slate-200 px-3 text-xs font-semibold text-slate-800 cursor-pointer"
        >
          {t("tools.clear")}
        </button>
      </div>

      <p className="text-xs text-slate-600">{t("tools.carryHint")}</p>

      <div className="rounded-xl border border-slate-300 bg-white p-3 font-mono">
        <div className="grid grid-cols-5 gap-1.5 text-center text-[11px] font-bold text-slate-400">
          <span />
          {COLS.map((c) => (
            <span key={c}>{c}</span>
          ))}
        </div>

        {/* Carry / Borrow scratch row */}
        <div className="mt-1 grid grid-cols-5 gap-1.5 items-center">
          <span className="text-[10px] text-slate-400 text-right pr-1">c/b</span>
          {carry.map((val, i) => (
            <input
              key={i}
              inputMode="numeric"
              value={val}
              onChange={(e) => updateCell(setCarry, i, e.target.value)}
              aria-label={`Carry ${COLS[i]}`}
              className="h-7 w-full rounded border border-dashed border-amber-400 bg-amber-50 text-center text-xs font-bold text-amber-800"
            />
          ))}
        </div>

        {/* Operand A */}
        <div className="mt-1.5 grid grid-cols-5 gap-1.5 items-center">
          <span />
          {rowA.map((val, i) => (
            <input
              key={i}
              inputMode="numeric"
              value={val}
              onChange={(e) => updateCell(setRowA, i, e.target.value)}
              aria-label={`First number ${COLS[i]}`}
              className="h-10 w-full rounded-lg border border-slate-300 text-center text-lg font-bold text-slate-900"
            />
          ))}
        </div>

        {/* Operand B */}
        <div className="mt-1.5 grid grid-cols-5 gap-1.5 items-center">
          <span className="text-center text-xl font-extrabold text-indigo-600">{op}</span>
          {rowB.map((val, i) => (
            <input
              key={i}
              inputMode="numeric"
              value={val}
              onChange={(e) => updateCell(setRowB, i, e.target.value)}
              aria-label={`Second number ${COLS[i]}`}
              className="h-10 w-full rounded-lg border border-slate-300 text-center text-lg font-bold text-slate-900"
            />
          ))}
        </div>

        <hr className="my-2 border-2 border-slate-800" />

        {/* Student Result Row */}
        <div className="grid grid-cols-5 gap-1.5 items-center">
          <span className="text-center text-lg font-bold text-slate-500">=</span>
          {result.map((val, i) => (
            <input
              key={i}
              inputMode="numeric"
              value={val}
              onChange={(e) => updateCell(setResult, i, e.target.value)}
              aria-label={`Result ${COLS[i]}`}
              className="h-11 w-full rounded-lg border-2 border-indigo-400 bg-indigo-50 text-center text-lg font-extrabold text-indigo-900"
            />
          ))}
        </div>
      </div>
    </div>
  );
}
