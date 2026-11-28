import { useEffect, useState } from "react";
import { Link } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Clock, Gift, Printer } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import { GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { QuestionPlayer } from "../../../components/question-player/QuestionPlayer";
import { apiClient, useAuthStore } from "../../../store/authStore";

function formatTimer(sec: number): string {
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  return [h, m, s].map((v) => String(v).padStart(2, "0")).join(":");
}

export function AstroQuizPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation(["astro-quiz", "common"]);
  const queryClient = useQueryClient();
  const user = useAuthStore((s) => s.user);
  const [grade, setGrade] = useState<number>(user?.grade ?? 1);
  const [category, setCategory] = useState("ALL");
  const [activePaperCode, setActivePaperCode] = useState<string | null>(null);
  const [qIdx, setQIdx] = useState(1);
  const [elapsedSec, setElapsedSec] = useState(0);
  const [drafts, setDrafts] = useState<Record<number, Record<string, string>>>({});

  const papersQuery = useQuery({
    queryKey: ["astro-quiz-papers", grade, category, locale],
    queryFn: () => apiClient.getAstroQuizPapers(grade, category, locale),
  });

  const attemptQuery = useQuery({
    queryKey: ["astro-quiz-attempt", activePaperCode, locale],
    queryFn: () => apiClient.openAstroQuizPaper(activePaperCode!, locale),
    enabled: Boolean(activePaperCode),
  });

  const attempt = attemptQuery.data;

  // Live elapsed timer while paper is IN_PROGRESS
  useEffect(() => {
    if (!activePaperCode || attempt?.status === "SUBMITTED") return;
    const id = window.setInterval(() => setElapsedSec((s) => s + 1), 1000);
    return () => window.clearInterval(id);
  }, [activePaperCode, attempt?.status]);

  const saveMutation = useMutation({
    mutationFn: () => apiClient.saveAstroQuizPaper(activePaperCode!, elapsedSec, drafts, locale),
    onSuccess: (updated) => {
      queryClient.setQueryData(["astro-quiz-attempt", activePaperCode, locale], updated);
    },
  });

  const submitMutation = useMutation({
    mutationFn: () => apiClient.submitAstroQuizPaper(activePaperCode!, elapsedSec, drafts, locale),
    onSuccess: (updated) => {
      queryClient.setQueryData(["astro-quiz-attempt", activePaperCode, locale], updated);
      void queryClient.invalidateQueries({ queryKey: ["astro-quiz-papers"] });
    },
  });

  if (activePaperCode && attempt) {
    const activeQ = attempt.questions[qIdx - 1] ?? attempt.questions[0]!;
    const submitted = attempt.status === "SUBMITTED";

    return (
      <div className="space-y-4">
        <GalaxyCard className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-xs font-bold uppercase text-cyan-300">{attempt.paperCode}</p>
            <h1 className="text-xl font-extrabold text-white">{attempt.title}</h1>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <span className="inline-flex min-h-10 items-center gap-1.5 rounded-xl border border-indigo-400/30 bg-[#0B1026] px-3 font-mono text-sm font-bold text-cyan-200">
              <Clock className="h-4 w-4" aria-hidden="true" />
              {formatTimer(submitted ? attempt.elapsedSec : elapsedSec)}
            </span>

            {/* Printing (and its answer key) is only offered once the paper is submitted. */}
            {submitted ? (
              <Link
                to="/$locale/astro-quiz/print/$paperCode"
                params={{ locale, paperCode: attempt.paperCode }}
              >
                <GalaxyButton variant="secondary">
                  <Printer className="h-4 w-4" aria-hidden="true" />
                  <span>{t("astro-quiz:printWorksheet")}</span>
                </GalaxyButton>
              </Link>
            ) : null}

            {!submitted ? (
              <>
                <GalaxyButton variant="secondary" onClick={() => saveMutation.mutate()}>
                  {t("astro-quiz:savePaper")}
                </GalaxyButton>
                <GalaxyButton
                  variant="gold"
                  onClick={() => submitMutation.mutate()}
                  disabled={submitMutation.isPending}
                >
                  {t("astro-quiz:submitPaper")}
                </GalaxyButton>
              </>
            ) : null}

            <GalaxyButton variant="ghost" onClick={() => setActivePaperCode(null)}>
              {t("astro-quiz:backToPapers")}
            </GalaxyButton>
          </div>
        </GalaxyCard>

        {submitted ? (
          <GalaxyCard className="border-emerald-400/50 bg-emerald-500/15 flex flex-wrap items-center justify-between gap-2">
            <p className="text-base font-extrabold text-emerald-200">
              🎉{" "}
              {t("astro-quiz:scoreBanner", {
                score: attempt.score,
                total: attempt.totalQuestions,
              })}
            </p>
            {attempt.supplyPodAwarded ? (
              <span className="rounded-full bg-amber-400/20 px-3 py-1 text-xs font-bold text-amber-200">
                🎁 +5 AC Supply Pod!
              </span>
            ) : null}
          </GalaxyCard>
        ) : null}

        {/* Question Proficiency Sidebar Strip */}
        <div className="flex flex-wrap gap-1.5">
          {attempt.questions.map((pq) => {
            const hasDraft =
              Boolean(drafts[pq.index] && Object.values(drafts[pq.index]!).some((v) => v.trim() !== "")) ||
              Boolean(pq.draftAnswer && Object.values(pq.draftAnswer).some((v) => v.trim() !== ""));
            const tone = submitted
              ? pq.isCorrect
                ? "border-emerald-400 bg-emerald-500/20 text-emerald-200"
                : "border-red-400 bg-red-500/20 text-red-200"
              : hasDraft
                ? "border-cyan-400 bg-cyan-500/20 text-cyan-100"
                : "border-indigo-400/25 bg-[#1E2A55] text-slate-200";
            return (
              <button
                key={pq.index}
                type="button"
                onClick={() => setQIdx(pq.index)}
                className={`min-h-10 min-w-10 rounded-xl border text-xs font-bold cursor-pointer ${tone} ${
                  pq.index === activeQ.index ? "ring-2 ring-[#FBBF24]" : ""
                }`}
              >
                Q{pq.index}
              </button>
            );
          })}
        </div>

        <QuestionPlayer
          mode="ASTRO_QUIZ"
          locale={locale}
          question={activeQ.question}
          locked={submitted}
          isCorrect={submitted ? (activeQ.isCorrect ?? false) : null}
          submittedAnswer={drafts[activeQ.index] ?? activeQ.draftAnswer}
          workedSolution={submitted ? (activeQ.workedSolution ?? null) : null}
          onSaveDraft={(answers) => {
            setDrafts((prev) => ({ ...prev, [activeQ.index]: answers }));
          }}
          onSubmit={(answers) => {
            setDrafts((prev) => ({ ...prev, [activeQ.index]: answers }));
            if (activeQ.index < attempt.questions.length) {
              setQIdx(activeQ.index + 1);
            }
          }}
          onNext={
            activeQ.index < attempt.questions.length ? () => setQIdx(activeQ.index + 1) : undefined
          }
        />
      </div>
    );
  }

  const claimed = papersQuery.data?.supplyPodClaimedThisWeek ?? false;

  return (
    <div className="space-y-6">
      <GalaxyCard className="space-y-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 className="text-2xl font-extrabold text-white">📝 {t("astro-quiz:title")}</h1>
            <p className="text-sm text-slate-300">{t("astro-quiz:subtitle")}</p>
          </div>
          <div className="inline-flex items-center gap-2 rounded-xl border border-amber-400/35 bg-amber-400/15 px-3.5 py-2 text-xs font-bold text-amber-200">
            <Gift className="h-4 w-4 shrink-0" aria-hidden="true" />
            <span>
              {claimed ? t("astro-quiz:supplyPodClaimed") : t("astro-quiz:supplyPodBanner")}
            </span>
          </div>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap gap-1.5">
            {[1, 2, 3, 4, 5, 6].map((g) => (
              <button
                key={g}
                type="button"
                onClick={() => setGrade(g)}
                className={`min-h-10 rounded-xl px-3.5 text-xs font-bold cursor-pointer ${
                  grade === g ? "bg-[#6366F1] text-white" : "bg-[#1E2A55] text-slate-200"
                }`}
              >
                {t("common:common.grade", { grade: g })}
              </button>
            ))}
          </div>

          <div className="flex flex-wrap gap-1.5">
            {(["ALL", "ESSENTIAL", "TOP_SCHOOL", "HOTS"] as const).map((cat) => (
              <button
                key={cat}
                type="button"
                onClick={() => setCategory(cat)}
                className={`min-h-10 rounded-xl px-3 text-xs font-bold cursor-pointer ${
                  category === cat ? "bg-[#22D3EE] text-slate-950" : "bg-[#1E2A55] text-slate-200"
                }`}
              >
                {t(`astro-quiz:categories.${cat}`)}
              </button>
            ))}
          </div>
        </div>
      </GalaxyCard>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
        {(papersQuery.data?.papers ?? []).map((paper) => (
          <GalaxyCard key={paper.code} className="flex flex-col justify-between gap-4">
            <div className="space-y-1.5">
              <div className="flex items-center justify-between text-xs">
                <span className="rounded-lg bg-indigo-500/30 px-2 py-0.5 font-bold text-indigo-200">
                  {t(`astro-quiz:categories.${paper.category}`)}
                </span>
                <span className="font-mono text-slate-400">{paper.code}</span>
              </div>
              <h2 className="text-base font-extrabold text-white">{paper.title}</h2>
              <p className="text-xs text-slate-300">
                {paper.questionCount} Qs • {paper.status}
                {paper.status === "SUBMITTED" ? ` (${paper.bestScore}/${paper.questionCount})` : ""}
              </p>
            </div>

            <div className="flex items-center gap-2">
              <GalaxyButton
                className="flex-1"
                onClick={() => {
                  setActivePaperCode(paper.code);
                  setQIdx(1);
                  setElapsedSec(0);
                  setDrafts({});
                }}
              >
                {t("astro-quiz:openPaper")}
              </GalaxyButton>
              <Link
                to="/$locale/astro-quiz/print/$paperCode"
                params={{ locale, paperCode: paper.code }}
              >
                <GalaxyButton variant="secondary" aria-label={t("astro-quiz:printWorksheet")}>
                  <Printer className="h-4 w-4" aria-hidden="true" />
                </GalaxyButton>
              </Link>
            </div>
          </GalaxyCard>
        ))}
      </div>
    </div>
  );
}
