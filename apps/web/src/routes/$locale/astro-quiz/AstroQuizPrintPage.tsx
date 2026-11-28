import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Printer } from "lucide-react";
import type { InputPart } from "@planetlogi/api-client";
import type { Locale } from "@planetlogi/i18n";
import { DiagramRenderer } from "../../../components/question-player/DiagramRenderer";
import { apiClient } from "../../../store/authStore";

export function AstroQuizPrintPage({
  locale,
  paperCode,
}: {
  locale: Locale;
  paperCode: string;
}) {
  const { t } = useTranslation(["astro-quiz", "common"]);
  const [showKey, setShowKey] = useState(false);

  const printQuery = useQuery({
    queryKey: ["astro-quiz-print", paperCode, locale],
    queryFn: () => apiClient.getPrintablePaper(paperCode, locale),
  });

  const paper = printQuery.data;
  if (printQuery.isError) {
    return (
      <p role="alert" className="p-8 text-center text-sm font-bold text-red-300">
        {t("astro-quiz:print.loadError")}
      </p>
    );
  }
  if (!paper) {
    return <p className="p-8 text-center text-slate-300">{t("common:common.loading")}</p>;
  }

  return (
    <div className="min-h-screen bg-white text-black p-6 sm:p-10 print:p-0">
      {/* Screen-only Print Toolbar */}
      <div className="mx-auto mb-6 flex max-w-3xl flex-wrap items-center justify-between gap-3 rounded-xl border border-slate-300 bg-slate-100 p-3 print:hidden">
        <label className="inline-flex items-center gap-2 text-sm font-semibold text-slate-800 cursor-pointer">
          <input
            type="checkbox"
            checked={showKey}
            onChange={(e) => setShowKey(e.target.checked)}
          />
          <span>{t("astro-quiz:print.toggleAnswerKey")}</span>
        </label>
        <button
          type="button"
          onClick={() => window.print()}
          className="inline-flex min-h-11 items-center gap-2 rounded-xl bg-indigo-600 px-4 text-sm font-bold text-white hover:bg-indigo-700 cursor-pointer"
        >
          <Printer className="h-4 w-4" aria-hidden="true" />
          <span>{t("astro-quiz:print.printNow")}</span>
        </button>
      </div>

      {/* Crisp A4 Worksheet Surface */}
      <article className="mx-auto max-w-3xl space-y-6">
        <header className="border-b-2 border-black pb-4 space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-extrabold uppercase tracking-widest">
                PlanetLogi • {paper.paperCode}
              </p>
              <h1 className="text-2xl font-extrabold">{paper.title}</h1>
            </div>
            <div className="border-2 border-black px-4 py-2 text-center">
              <p className="text-[11px] font-bold uppercase">{t("astro-quiz:print.scoreBox")}</p>
              <p className="text-lg font-extrabold">___ / {paper.questions.length}</p>
            </div>
          </div>

          <div className="grid grid-cols-3 gap-4 text-sm pt-1">
            <p>
              <strong>{t("astro-quiz:print.studentName")}:</strong> ____________________
            </p>
            <p>
              <strong>{t("astro-quiz:print.className")}:</strong> ____________
            </p>
            <p>
              <strong>{t("astro-quiz:print.date")}:</strong> ____________
            </p>
          </div>
        </header>

        <ol className="space-y-6">
          {paper.questions.map((q, idx) => (
            <li
              key={q.id}
              className="break-inside-avoid rounded-lg border border-slate-300 p-4 space-y-3"
            >
              <p className="text-base font-semibold leading-relaxed">
                <span className="mr-2 font-extrabold">{idx + 1}.</span>
                {q.stem}
              </p>
              {q.options && q.options.length > 0 ? (
                <ul className="grid grid-cols-1 gap-1 pl-6 text-sm sm:grid-cols-2">
                  {q.options.map((opt) => (
                    <li key={opt.key}>
                      <strong>{opt.key}.</strong> {opt.label}
                    </li>
                  ))}
                </ul>
              ) : null}
              {q.diagram ? <DiagramRenderer diagram={q.diagram} /> : null}
              <div className="pt-2">
                <p className="text-xs text-slate-500">{t("astro-quiz:print.workingSpace")}</p>
                <div className="mt-2 h-20 border-b border-dashed border-slate-400" />
              </div>
              <div className="space-y-1">
                {((q.parts && q.parts.length > 0 ? q.parts : [{ key: "answer" }]) as InputPart[]).map(
                  (part) => (
                    <p key={part.key} className="text-right text-sm font-bold">
                      {part.label ?? part.key}: ___________________
                    </p>
                  )
                )}
              </div>
            </li>
          ))}
        </ol>

        {showKey ? (
          <section className="break-before-page border-t-2 border-black pt-4 space-y-2">
            <h2 className="text-lg font-extrabold">
              {t("astro-quiz:print.answerKey")} ({paper.paperCode})
            </h2>
            <ol className="grid grid-cols-2 gap-2 text-sm">
              {paper.answerKeys.map((ans, idx) => {
                const parts: InputPart[] =
                  paper.questions[idx]?.parts && paper.questions[idx]!.parts!.length > 0
                    ? paper.questions[idx]!.parts!
                    : [{ key: "answer" }];
                return (
                  <li key={idx}>
                    <strong>{idx + 1}.</strong>{" "}
                    {parts
                      .filter((part) => ans[part.key] !== undefined)
                      .map((part) => `${part.label ?? part.key}: ${ans[part.key]}`)
                      .join(" · ")}
                  </li>
                );
              })}
            </ol>
          </section>
        ) : null}
      </article>
    </div>
  );
}
