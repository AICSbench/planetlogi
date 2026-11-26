import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Calculator, Columns3, PenTool, Sigma } from "lucide-react";
import { StarSketchpad } from "./StarSketchpad";
import { EquationPad } from "./EquationPad";
import { BarModelBuilder } from "./BarModelBuilder";
import { ColumnOperationsGrid } from "./ColumnOperationsGrid";

type ActiveTool = "sketchpad" | "equationPad" | "barModel" | "columnCalc" | null;

export function MathToolsDrawer() {
  const { t } = useTranslation("player");
  const [active, setActive] = useState<ActiveTool>(null);

  const tools = [
    { key: "sketchpad" as const, icon: PenTool, label: t("tools.sketchpad") },
    { key: "equationPad" as const, icon: Sigma, label: t("tools.equationPad") },
    { key: "barModel" as const, icon: Columns3, label: t("tools.barModel") },
    { key: "columnCalc" as const, icon: Calculator, label: t("tools.columnCalc") },
  ];

  return (
    <div className="rounded-2xl border border-indigo-400/25 bg-[#131B3A]/95 p-3 space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-xs font-bold uppercase tracking-wider text-slate-300 mr-1">
          {t("tools.title")}:
        </span>
        {tools.map(({ key, icon: Icon, label }) => (
          <button
            key={key}
            type="button"
            onClick={() => setActive((prev) => (prev === key ? null : key))}
            className={`inline-flex min-h-11 items-center gap-1.5 rounded-xl px-3 text-xs font-bold transition-colors cursor-pointer ${
              active === key
                ? "bg-[#6366F1] text-white shadow"
                : "bg-[#1E2A55] text-slate-100 hover:bg-[#2A3A73]"
            }`}
          >
            <Icon className="h-4 w-4" aria-hidden="true" />
            <span>{label}</span>
          </button>
        ))}
      </div>

      {active ? (
        <div className="rounded-xl bg-[#FAFBFD] p-3 text-[#111827] border border-slate-200">
          {active === "sketchpad" ? <StarSketchpad /> : null}
          {active === "equationPad" ? <EquationPad /> : null}
          {active === "barModel" ? <BarModelBuilder /> : null}
          {active === "columnCalc" ? <ColumnOperationsGrid /> : null}
        </div>
      ) : null}
    </div>
  );
}
