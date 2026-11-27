import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Filter, RefreshCw, SlidersHorizontal, Sparkles } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import type { DailyOrbitTier } from "@planetlogi/api-client";
import { CurrencyPill, GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { QuestionPlayer } from "../../../components/question-player/QuestionPlayer";
import { apiClient, useAuthStore } from "../../../store/authStore";

const CURRICULA_OPTIONS = [
  { code: "SINGAPORE_MOE", label: "Singapore Math MOE" },
  { code: "HOTS_WHEEL", label: "HOTS Constellation" },
  { code: "KURIKULUM_MERDEKA", label: "Kurikulum Merdeka" },
  { code: "CAMBRIDGE_0096", label: "Cambridge 0096" },
  { code: "CHALLENGING_MATHS", label: "Challenging Maths" },
] as const;

export function DailyOrbitPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation(["daily-orbit", "common"]);
  const queryClient = useQueryClient();
  const user = useAuthStore((s) => s.user);
  const grade = user?.grade ?? 1;
  const sessionCacheKey = ["daily-orbit-today", user?.id ?? "guest", locale, grade] as const;

  const [activeSlotIdx, setActiveSlotIdx] = useState(1);
  const [showTierModal, setShowTierModal] = useState(false);
  const [showCurriculaMenu, setShowCurriculaMenu] = useState(false);
  const [showChangeMenu, setShowChangeMenu] = useState(false);
  const [showBreakdown, setShowBreakdown] = useState(false);

  const sessionQuery = useQuery({
    queryKey: sessionCacheKey,
    queryFn: () => apiClient.getDailyOrbitToday(locale, grade),
  });

  const submitMutation = useMutation({
    mutationFn: (vars: {
      slotIndex: number;
      answers: Record<string, string>;
      templateCode?: string;
      seed?: number;
    }) =>
      apiClient.submitDailyOrbit(vars.slotIndex, vars.answers, locale, grade, {
        templateCode: vars.templateCode,
        seed: vars.seed,
      }),
    onSuccess: (data) => {
      queryClient.setQueryData(sessionCacheKey, data.session);
      void queryClient.invalidateQueries({ queryKey: ["streak-status"] });
    },
  });

  const skipMutation = useMutation({
    mutationFn: (slotIndex: number) => apiClient.skipDailyOrbit(slotIndex, locale, grade),
    onSuccess: (data) => {
      queryClient.setQueryData(sessionCacheKey, data.session);
    },
  });

  const changeMutation = useMutation({
    mutationFn: (vars: { slotIndex: number; mode: "easier" | "harder" | "topic" }) =>
      apiClient.changeDailyOrbitQuestion(vars.slotIndex, vars.mode, locale, grade),
    onSuccess: (data) => {
      queryClient.setQueryData(sessionCacheKey, data.session);
      setShowChangeMenu(false);
    },
  });

  const tierMutation = useMutation({
    mutationFn: (tier: DailyOrbitTier) => apiClient.setDailyOrbitTier(tier, locale, grade),
    onSuccess: (session) => {
      queryClient.setQueryData(sessionCacheKey, session);
      setShowTierModal(false);
    },
  });

  const curriculaMutation = useMutation({
    mutationFn: (curricula: string[]) => apiClient.setDailyOrbitCurricula(curricula, locale, grade),
    onSuccess: (session) => {
      queryClient.setQueryData(["daily-orbit-today", locale, grade], session);
    },
  });

  const session = sessionQuery.data;
  if (sessionQuery.isLoading || !session) {
    return (
      <GalaxyCard className="mx-auto max-w-3xl text-center py-12">
        <p className="text-slate-300">{t("common:common.loading")}</p>
      </GalaxyCard>
    );
  }

  if (!session.isOpen) {
    return (
      <GalaxyCard className="mx-auto max-w-2xl text-center py-12 space-y-3">
        <p className="text-4xl">🌙</p>
        <h1 className="text-2xl font-extrabold text-white">{t("daily-orbit:closedTitle")}</h1>
        <p className="text-sm text-slate-300">{t("daily-orbit:closedBody")}</p>
      </GalaxyCard>
    );
  }

  const activeSlot = session.slots[activeSlotIdx - 1] ?? session.slots[0]!;
  const changeRemaining = Math.max(0, session.changeQuestionLimit - session.changeQuestionUsed);

  function toggleCurriculum(code: string) {
    if (!session) return;
    const exists = session.activeCurricula.includes(code);
    const next = exists
      ? session.activeCurricula.filter((c) => c !== code)
      : [...session.activeCurricula, code];
    if (next.length > 0) {
      curriculaMutation.mutate(next);
    }
  }

  return (
    <div className="space-y-5">
      {/* Header & Controls Bar */}
      <GalaxyCard className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-2xl font-extrabold text-white">🪐 {t("daily-orbit:title")}</h1>
            <p className="text-sm text-slate-300">{t("daily-orbit:subtitle")}</p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {/* Today's SP Breakdown Dropdown */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setShowBreakdown((b) => !b)}
                className="min-h-11 rounded-xl border border-amber-400/35 bg-amber-400/15 px-3 text-sm font-bold text-amber-200 cursor-pointer"
              >
                ★ {session.todaySpBreakdown.totalToday} SP ▾
              </button>
              {showBreakdown ? (
                <div className="absolute right-0 z-30 mt-2 w-60 rounded-2xl border border-indigo-400/25 bg-[#131B3A] p-3 text-xs space-y-2 shadow-2xl">
                  <p className="font-bold text-white">{t("daily-orbit:todayBreakdown")}</p>
                  <div className="flex justify-between text-slate-300">
                    <span>{t("daily-orbit:breakdownDaily")}</span>
                    <strong>{session.todaySpBreakdown.dailyOrbit} SP</strong>
                  </div>
                  <div className="flex justify-between text-slate-300">
                    <span>{t("daily-orbit:breakdownDuels")}</span>
                    <strong>{session.todaySpBreakdown.starDuels} SP</strong>
                  </div>
                  <div className="flex justify-between text-slate-300">
                    <span>{t("daily-orbit:breakdownSunday")}</span>
                    <strong>{session.todaySpBreakdown.sundayMeteor} SP</strong>
                  </div>
                  <hr className="border-indigo-400/20" />
                  <div className="flex justify-between text-amber-300 font-bold">
                    <span>{t("daily-orbit:breakdownTotal")}</span>
                    <span>{session.todaySpBreakdown.totalToday} SP</span>
                  </div>
                </div>
              ) : null}
            </div>

            <GalaxyButton variant="secondary" onClick={() => setShowTierModal(true)}>
              <SlidersHorizontal className="h-4 w-4" aria-hidden="true" />
              <span>{t(`daily-orbit:tiers.${session.tier}`)}</span>
            </GalaxyButton>

            <div className="relative">
              <GalaxyButton
                variant="secondary"
                onClick={() => setShowCurriculaMenu((v) => !v)}
              >
                <Filter className="h-4 w-4" aria-hidden="true" />
                <span>{t("daily-orbit:curriculaButton")}</span>
              </GalaxyButton>
              {showCurriculaMenu ? (
                <div className="absolute right-0 z-30 mt-2 w-64 rounded-2xl border border-indigo-400/25 bg-[#131B3A] p-3 shadow-2xl space-y-2">
                  {CURRICULA_OPTIONS.map((opt) => (
                    <label
                      key={opt.code}
                      className="flex min-h-10 items-center gap-2.5 rounded-lg px-2 text-sm text-slate-200 hover:bg-white/5 cursor-pointer"
                    >
                      <input
                        type="checkbox"
                        checked={session.activeCurricula.includes(opt.code)}
                        onChange={() => toggleCurriculum(opt.code)}
                      />
                      <span>{opt.label}</span>
                    </label>
                  ))}
                </div>
              ) : null}
            </div>
          </div>
        </div>

        {/* 10-Question Cosmic Progression Path (Circles -> Hexagons -> Stars) */}
        <div className="grid grid-cols-5 gap-2 sm:grid-cols-10">
          {session.slots.map((slot) => {
            const isCurrent = slot.index === activeSlot.index;
            const statusTone =
              slot.status === "CORRECT"
                ? "bg-emerald-500/25 border-emerald-400 text-emerald-200"
                : slot.status === "WRONG"
                  ? "bg-red-500/25 border-red-400 text-red-200"
                  : slot.status === "SKIPPED"
                    ? "bg-slate-600/30 border-slate-400 text-slate-300"
                    : "bg-[#1E2A55] border-indigo-400/30 text-slate-200";
            const shapeIcon =
              slot.nodeShape === "star" ? "★" : slot.nodeShape === "hexagon" ? "⬡" : "●";

            return (
              <button
                key={slot.index}
                type="button"
                onClick={() => setActiveSlotIdx(slot.index)}
                className={`flex min-h-14 flex-col items-center justify-center rounded-2xl border-2 px-1 text-xs font-bold transition-all cursor-pointer ${statusTone} ${
                  isCurrent ? "ring-2 ring-[#22D3EE] scale-105" : ""
                }`}
              >
                <span className="text-[11px] opacity-80">
                  {shapeIcon} Q{slot.index}
                </span>
                <span className="text-[10px] text-amber-300">{slot.rewardSp} SP</span>
              </button>
            );
          })}
        </div>

        {session.completionBonusAwarded ? (
          <div className="inline-flex items-center gap-2 rounded-xl bg-emerald-500/20 border border-emerald-400/40 px-4 py-2 text-sm font-bold text-emerald-200">
            <Sparkles className="h-4 w-4" aria-hidden="true" />
            <span>{t("daily-orbit:completionBonus")}</span>
          </div>
        ) : null}
      </GalaxyCard>

      {/* Active Slot Question Player */}
      <QuestionPlayer
        mode="DAILY_ORBIT"
        locale={locale}
        question={activeSlot.question}
        locked={activeSlot.status !== "PENDING"}
        isCorrect={
          activeSlot.status === "PENDING" ? null : activeSlot.status === "CORRECT"
        }
        submittedAnswer={activeSlot.submittedAnswer}
        workedSolution={activeSlot.workedSolution ?? null}
        submitting={submitMutation.isPending}
        onSubmit={async (answers) => {
          await submitMutation.mutateAsync({
            slotIndex: activeSlot.index,
            answers,
            templateCode: activeSlot.question.templateCode,
            seed: activeSlot.question.seed,
          });
        }}
        onSkip={async () => {
          await skipMutation.mutateAsync(activeSlot.index);
        }}
        onNext={
          activeSlot.index < 10 ? () => setActiveSlotIdx(activeSlot.index + 1) : undefined
        }
        extraHeaderActions={
          activeSlot.status === "PENDING" ? (
            <div className="relative">
              <button
                type="button"
                disabled={changeRemaining === 0 || changeMutation.isPending}
                onClick={() => setShowChangeMenu((v) => !v)}
                className="inline-flex min-h-10 items-center gap-1.5 rounded-xl border border-indigo-300 bg-indigo-50 px-3 text-xs font-bold text-indigo-800 hover:bg-indigo-100 disabled:opacity-45 cursor-pointer"
              >
                <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
                <span>
                  {t("daily-orbit:changeQuestion")} ({changeRemaining}/{session.changeQuestionLimit})
                </span>
              </button>
              {showChangeMenu ? (
                <div className="absolute right-0 z-30 mt-2 w-48 rounded-xl border border-slate-200 bg-white p-1.5 shadow-xl">
                  {(
                    [
                      ["easier", t("daily-orbit:changeEasier")],
                      ["harder", t("daily-orbit:changeHarder")],
                      ["topic", t("daily-orbit:changeTopic")],
                    ] as const
                  ).map(([mode, label]) => (
                    <button
                      key={mode}
                      type="button"
                      onClick={() =>
                        changeMutation.mutate({ slotIndex: activeSlot.index, mode })
                      }
                      className="flex w-full min-h-10 items-center rounded-lg px-3 text-left text-xs font-semibold text-slate-800 hover:bg-indigo-50 cursor-pointer"
                    >
                      {label}
                    </button>
                  ))}
                </div>
              ) : null}
            </div>
          ) : (
            <CurrencyPill kind="SP" value={`+${activeSlot.earnedSp}`} label="SP" />
          )
        }
      />

      {/* 4-Tier Difficulty Modal */}
      {showTierModal ? (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4"
        >
          <GalaxyCard className="max-w-md w-full space-y-4">
            <h2 className="text-lg font-extrabold text-white">{t("daily-orbit:tierButton")}</h2>
            <div className="space-y-2">
              {(["ELEMENTARY", "INTERMEDIATE", "ADVANCED", "MASTER"] as const).map((tier) => {
                const unlocked = session.unlockedTiers.includes(tier);
                return (
                  <button
                    key={tier}
                    type="button"
                    disabled={!unlocked}
                    onClick={() => tierMutation.mutate(tier)}
                    className={`flex w-full min-h-12 items-center justify-between rounded-xl border px-4 text-sm font-bold cursor-pointer ${
                      session.tier === tier
                        ? "border-[#22D3EE] bg-[#6366F1]/30 text-white"
                        : "border-indigo-400/25 bg-[#1E2A55] text-slate-200 disabled:opacity-45"
                    }`}
                  >
                    <span>{t(`daily-orbit:tiers.${tier}`)}</span>
                    <span>{unlocked ? "✓" : "🔒"}</span>
                  </button>
                );
              })}
            </div>
            <div className="flex justify-end">
              <GalaxyButton variant="secondary" onClick={() => setShowTierModal(false)}>
                Close
              </GalaxyButton>
            </div>
          </GalaxyCard>
        </div>
      ) : null}
    </div>
  );
}
