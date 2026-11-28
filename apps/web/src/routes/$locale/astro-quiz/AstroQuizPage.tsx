import { useEffect, useMemo, useRef, useState } from "react";
import { Link } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Clock, Gift, Printer } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import { GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { QuestionPlayer } from "../../../components/question-player/QuestionPlayer";
import { apiClient, useAccountScope, useAuthStore } from "../../../store/authStore";

const MAX_ELAPSED_SEC = 24 * 60 * 60;

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
  const account = useAccountScope();
  const [selectedGrade, setSelectedGrade] = useState<number | null>(null);
  const grade = selectedGrade ?? user?.grade ?? 1;
  const [category, setCategory] = useState("ALL");
  const [activePaperCode, setActivePaperCode] = useState<string | null>(null);
  const [qIdx, setQIdx] = useState(1);
  const [sessionSec, setSessionSec] = useState(0);
  const [draftEdits, setDraftEdits] = useState<Record<number, Record<string, string>>>({});
  const [actionError, setActionError] = useState<string | null>(null);

  const papersQuery = useQuery({
    queryKey: ["astro-quiz-papers", account, grade, category, locale],
    queryFn: () => apiClient.getAstroQuizPapers(grade, category, locale),
  });

  const attemptQuery = useQuery({
    queryKey: ["astro-quiz-attempt", account, activePaperCode, locale],
    queryFn: () => apiClient.openAstroQuizPaper(activePaperCode!, locale),
    enabled: Boolean(activePaperCode),
  });

  const attempt = attemptQuery.data;

  // Saved answers come from the attempt; local edits win until the next save.
  const drafts = useMemo(() => {
    const merged: Record<number, Record<string, string>> = {};
    for (const pq of attempt?.questions ?? []) {
      if (pq.draftAnswer) merged[pq.index] = pq.draftAnswer;
    }
    return { ...merged, ...draftEdits };
  }, [attempt, draftEdits]);

  // Server-measured elapsed time plus the seconds since the last server response, capped at the
  // server's 24-hour limit (display only: the server ignores the client's elapsedSec).
  const elapsedSec = Math.min(MAX_ELAPSED_SEC, (attempt?.elapsedSec ?? 0) + sessionSec);
  const timerCapped = elapsedSec >= MAX_ELAPSED_SEC;

  // Live elapsed timer while paper is IN_PROGRESS
  useEffect(() => {
    if (!activePaperCode || attempt?.status === "SUBMITTED") return;
    const stored = attempt?.elapsedSec ?? 0;
    const id = window.setInterval(
      () => setSessionSec((s) => Math.min(s + 1, Math.max(0, MAX_ELAPSED_SEC - stored))),
      1000
    );
    return () => window.clearInterval(id);
  }, [activePaperCode, attempt?.status, attempt?.elapsedSec]);

  // The request identity (account, paper, locale) and send time travel with the payload so a
  // late response is written under the key it belongs to and seconds counted while the request
  // was in flight are not lost from the display.
  type PaperPayload = {
    account: string;
    paperCode: string;
    locale: string;
    sentAt: number;
    elapsedSec: number;
    drafts: Record<number, Record<string, string>>;
  };
  const payload = (d: Record<number, Record<string, string>>): PaperPayload => ({
    account,
    paperCode: activePaperCode!,
    locale,
    sentAt: Date.now(),
    elapsedSec,
    drafts: d,
  });
  const secondsSince = (sentAt: number) => Math.max(0, Math.floor((Date.now() - sentAt) / 1000));

  const checkpointRef = useRef<PaperPayload | null>(null);
  useEffect(() => {
    checkpointRef.current =
      activePaperCode && attempt?.status !== "SUBMITTED" ? payload(drafts) : null;
  });

  const saveMutation = useMutation({
    mutationFn: (p: PaperPayload) =>
      apiClient.saveAstroQuizPaper(p.paperCode, p.elapsedSec, p.drafts, p.locale),
    onSuccess: (updated, p) => {
      queryClient.setQueryData(["astro-quiz-attempt", p.account, p.paperCode, p.locale], updated);
      if (p.account === account && p.paperCode === activePaperCode) {
        setSessionSec(secondsSince(p.sentAt));
        setActionError(null);
      }
    },
    onError: (_err, p) => {
      if (p.paperCode === activePaperCode) setActionError(t("astro-quiz:actionError"));
    },
  });

  const submitMutation = useMutation({
    mutationFn: (p: PaperPayload) =>
      apiClient.submitAstroQuizPaper(p.paperCode, p.elapsedSec, p.drafts, p.locale),
    onSuccess: (updated, p) => {
      queryClient.setQueryData(["astro-quiz-attempt", p.account, p.paperCode, p.locale], updated);
      void queryClient.invalidateQueries({ queryKey: ["astro-quiz-papers", p.account] });
      if (p.account === account && p.paperCode === activePaperCode) {
        setSessionSec(0);
        setActionError(null);
      }
    },
    onError: (_err, p) => {
      if (p.paperCode === activePaperCode) setActionError(t("astro-quiz:submitError"));
    },
  });

  // Periodic checkpoints keep the server-measured timer current even when
  // the learner works for more than the 15-minute idle gap without saving.
  const saveCheckpoint = saveMutation.mutate;
  useEffect(() => {
    if (!activePaperCode || attempt?.status === "SUBMITTED") return;
    const id = window.setInterval(() => {
      const next = checkpointRef.current;
      if (next && next.elapsedSec > 0) saveCheckpoint(next);
    }, 60_000);
    return () => window.clearInterval(id);
  }, [activePaperCode, attempt?.status, saveCheckpoint]);

  if (activePaperCode && attemptQuery.isError) {
    return (
      <GalaxyCard className="space-y-3 text-center">
        <p role="alert" className="text-sm font-bold text-red-300">
          {t("astro-quiz:loadError")}
        </p>
        <GalaxyButton variant="secondary" onClick={() => setActivePaperCode(null)}>
          {t("astro-quiz:backToPapers")}
        </GalaxyButton>
      </GalaxyCard>
    );
  }

  if (activePaperCode && !attempt) {
    return (
      <GalaxyCard className="text-center text-slate-300">{t("common:common.loading")}</GalaxyCard>
    );
  }

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
            <span className="inline-flex min-h-11 items-center gap-1.5 rounded-xl border border-indigo-400/30 bg-[#0B1026] px-3 font-mono text-sm font-bold text-cyan-200">
              <Clock className="h-4 w-4" aria-hidden="true" />
              {formatTimer(submitted ? attempt.elapsedSec : elapsedSec)}
            </span>

            <Link
              to="/$locale/astro-quiz/print/$paperCode"
              params={{ locale, paperCode: attempt.paperCode }}
            >
              <GalaxyButton variant="secondary">
                <Printer className="h-4 w-4" aria-hidden="true" />
                <span>{t("astro-quiz:printWorksheet")}</span>
              </GalaxyButton>
            </Link>

            {!submitted ? (
              <>
                <GalaxyButton
                  variant="secondary"
                  onClick={() => saveMutation.mutate(payload(drafts))}
                  disabled={saveMutation.isPending || submitMutation.isPending}
                >
                  {t("astro-quiz:savePaper")}
                </GalaxyButton>
                <GalaxyButton
                  variant="gold"
                  onClick={() => submitMutation.mutate(payload(drafts))}
                  disabled={submitMutation.isPending || saveMutation.isPending}
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

        {actionError ? (
          <p role="alert" className="text-sm font-bold text-red-300">
            {actionError}
          </p>
        ) : null}
        {timerCapped ? (
          <p role="status" className="text-sm font-bold text-amber-200">
            {t("astro-quiz:timerLimitReached")}
          </p>
        ) : null}

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
                {t("astro-quiz:supplyPodBadge")}
              </span>
            ) : !attempt.supplyPodClaimedThisWeek ? (
              <GalaxyButton
                variant="gold"
                onClick={() => submitMutation.mutate(payload(drafts))}
                disabled={submitMutation.isPending}
              >
                <Gift className="h-4 w-4" aria-hidden="true" />
                <span>{t("astro-quiz:retrySupplyPod")}</span>
              </GalaxyButton>
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
                className={`min-h-11 min-w-11 rounded-xl border text-xs font-bold cursor-pointer ${tone} ${
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
          submittedAnswer={
            submitted
              ? (activeQ.draftAnswer ?? drafts[activeQ.index])
              : (drafts[activeQ.index] ?? activeQ.draftAnswer)
          }
          submitting={submitMutation.isPending}
          workedSolution={submitted ? (activeQ.workedSolution ?? null) : null}
          onDraftChange={(answers) => {
            setDraftEdits((prev) => ({ ...prev, [activeQ.index]: answers }));
          }}
          onSaveDraft={(answers) => {
            const next = { ...drafts, [activeQ.index]: answers };
            setDraftEdits((prev) => ({ ...prev, [activeQ.index]: answers }));
            return saveMutation.mutateAsync(payload(next));
          }}
          onSubmit={(answers) => {
            setDraftEdits((prev) => ({ ...prev, [activeQ.index]: answers }));
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
                onClick={() => setSelectedGrade(g)}
                className={`min-h-11 rounded-xl px-3.5 text-xs font-bold cursor-pointer ${
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
                className={`min-h-11 rounded-xl px-3 text-xs font-bold cursor-pointer ${
                  category === cat ? "bg-[#22D3EE] text-slate-950" : "bg-[#1E2A55] text-slate-200"
                }`}
              >
                {t(`astro-quiz:categories.${cat}`)}
              </button>
            ))}
          </div>
        </div>
      </GalaxyCard>

      {papersQuery.isError ? (
        <GalaxyCard className="space-y-3 text-center">
          <p role="alert" className="text-sm font-bold text-red-300">
            {t("astro-quiz:loadError")}
          </p>
          <GalaxyButton variant="secondary" onClick={() => void papersQuery.refetch()}>
            {t("common:common.retry")}
          </GalaxyButton>
        </GalaxyCard>
      ) : papersQuery.isPending ? (
        <GalaxyCard className="text-center text-slate-300">{t("common:common.loading")}</GalaxyCard>
      ) : null}

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
                {t("astro-quiz:questionShort", { count: paper.questionCount })} •{" "}
                {t(`astro-quiz:statuses.${paper.status}`)}
                {paper.status === "SUBMITTED" ? ` (${paper.bestScore}/${paper.questionCount})` : ""}
              </p>
            </div>

            <div className="flex items-center gap-2">
              <GalaxyButton
                className="flex-1"
                onClick={() => {
                  setActivePaperCode(paper.code);
                  setQIdx(1);
                  setSessionSec(0);
                  setDraftEdits({});
                  setActionError(null);
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
