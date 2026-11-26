import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { Locale } from "@planetlogi/i18n";
import type { GradeResult, PracticeSession } from "@planetlogi/api-client";
import {
  DifficultyMeter,
  GalaxyButton,
  GalaxyCard,
  ProficiencyPill,
  StarRating,
} from "@planetlogi/ui";
import { QuestionPlayer } from "../../../components/question-player/QuestionPlayer";
import { apiClient, useAuthStore } from "../../../store/authStore";

const QUADRANT_COLORS = ["#6366F1", "#10B981", "#F59E0B", "#0EA5E9"] as const;

export function TrainingPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation(["training", "common"]);
  const queryClient = useQueryClient();
  const user = useAuthStore((s) => s.user);

  const [selectedCurriculum, setSelectedCurriculum] = useState("SINGAPORE_MOE");
  const [grade, setGrade] = useState<number>(user?.grade ?? 1);
  const [viewMode, setViewMode] = useState<"HIGH_SCORE" | "PROFICIENCY">("PROFICIENCY");
  const [ageBand, setAgeBand] = useState("ALL");

  // Active 10-question practice session state
  const [activeSession, setActiveSession] = useState<PracticeSession | null>(null);
  const [qIdx, setQIdx] = useState(0);
  const [lastGrade, setLastGrade] = useState<GradeResult | null>(null);

  const curriculaQuery = useQuery({
    queryKey: ["training-curricula", locale],
    queryFn: () => apiClient.getCurricula(locale),
  });

  const skillsQuery = useQuery({
    queryKey: ["training-skills", selectedCurriculum, grade, locale],
    queryFn: () => apiClient.getSkills(selectedCurriculum, grade, locale),
    enabled: selectedCurriculum !== "HOTS_WHEEL",
  });

  const hotsQuery = useQuery({
    queryKey: ["training-hots", ageBand, locale],
    queryFn: () => apiClient.getHotsWheel(ageBand, locale),
    enabled: selectedCurriculum === "HOTS_WHEEL",
  });

  const startMutation = useMutation({
    mutationFn: (skillCode: string) => apiClient.startPracticeSession(skillCode, locale),
    onSuccess: (sess) => {
      setActiveSession(sess);
      setQIdx(0);
      setLastGrade(null);
    },
  });

  const submitMutation = useMutation({
    mutationFn: (vars: { sessionId: string; questionIndex: number; answers: Record<string, string> }) =>
      apiClient.submitPracticeAnswer(vars.sessionId, vars.questionIndex, vars.answers, locale),
    onSuccess: (data) => {
      setLastGrade(data.result);
      setActiveSession((prev) =>
        prev
          ? {
              ...prev,
              answered: data.session.answered,
              correct: data.session.correct,
              starsEarned: data.session.starsEarned,
            }
          : null
      );
      void queryClient.invalidateQueries({ queryKey: ["training-skills"] });
      void queryClient.invalidateQueries({ queryKey: ["training-hots"] });
      void queryClient.invalidateQueries({ queryKey: ["reports"] });
    },
  });

  // Render Active 10-Question Practice Mode
  if (activeSession) {
    const currentQuestion = activeSession.questions[qIdx]!;
    return (
      <div className="space-y-4">
        <GalaxyCard className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-xs font-bold uppercase text-cyan-300">{activeSession.skillName}</p>
            <p className="text-sm font-semibold text-white">
              {t("training:sessionProgress", {
                current: qIdx + 1,
                total: activeSession.questions.length,
              })}
            </p>
          </div>
          <div className="flex items-center gap-3">
            <StarRating stars={activeSession.starsEarned as 0 | 1 | 2 | 3} />
            <GalaxyButton
              variant="secondary"
              onClick={() => {
                setActiveSession(null);
                setLastGrade(null);
              }}
            >
              {t("training:backToSkills")}
            </GalaxyButton>
          </div>
        </GalaxyCard>

        <QuestionPlayer
          mode="GALAXY_TRAINING"
          locale={locale}
          question={currentQuestion}
          locked={lastGrade !== null}
          isCorrect={lastGrade ? lastGrade.isCorrect : null}
          workedSolution={lastGrade?.workedSolution ?? null}
          submitting={submitMutation.isPending}
          onSubmit={async (answers) => {
            await submitMutation.mutateAsync({
              sessionId: activeSession.sessionId,
              questionIndex: qIdx + 1,
              answers,
            });
          }}
          onNext={() => {
            if (qIdx + 1 < activeSession.questions.length) {
              setQIdx(qIdx + 1);
              setLastGrade(null);
            } else {
              setActiveSession(null);
              setLastGrade(null);
            }
          }}
        />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <GalaxyCard className="space-y-4">
        <div>
          <h1 className="text-2xl font-extrabold text-white">🌌 {t("training:title")}</h1>
          <p className="text-sm text-slate-300">{t("training:subtitle")}</p>
        </div>

        {/* 6 Curriculum Track Selector */}
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {(curriculaQuery.data?.curricula ?? []).map((cur) => {
            const active = selectedCurriculum === cur.code;
            return (
              <button
                key={cur.code}
                type="button"
                onClick={() => setSelectedCurriculum(cur.code)}
                className={`rounded-2xl border-2 p-4 text-left transition-all cursor-pointer ${
                  active
                    ? "border-[#22D3EE] bg-[#6366F1]/25 shadow-lg"
                    : "border-indigo-400/20 bg-[#1E2A55]/70 hover:border-indigo-400/50"
                }`}
              >
                <p className="text-base font-extrabold text-white">{cur.name}</p>
                <p className="mt-1 text-xs text-slate-300">{cur.description}</p>
              </button>
            );
          })}
        </div>
      </GalaxyCard>

      {/* HOTS 4-Quadrant Constellation Wheel OR Standard Grade Skill List */}
      {selectedCurriculum === "HOTS_WHEEL" ? (
        <GalaxyCard className="space-y-5">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 className="text-xl font-extrabold text-white">✨ {t("training:hotsTitle")}</h2>
              <p className="text-xs text-slate-300">{t("training:hotsSubtitle")}</p>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {(["ALL", "6-9", "8-11", "10-13"] as const).map((band) => (
                <button
                  key={band}
                  type="button"
                  onClick={() => setAgeBand(band)}
                  className={`min-h-10 rounded-xl px-3 text-xs font-bold cursor-pointer ${
                    ageBand === band
                      ? "bg-[#6366F1] text-white"
                      : "bg-[#1E2A55] text-slate-200 hover:bg-[#2A3A73]"
                  }`}
                >
                  {band === "ALL" ? t("training:ageBandAll") : t("training:ageBand", { band })}
                </button>
              ))}
            </div>
          </div>

          {/* Interactive 4-Quadrant SVG Wheel + Strategy Grid */}
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-12 items-center">
            <div className="lg:col-span-5 flex justify-center">
              <svg viewBox="0 0 260 260" className="w-64 h-64" role="img" aria-label="HOTS 4-Quadrant Wheel">
                <path d="M130,130 L130,12 A118,118 0 0,1 248,130 Z" fill={QUADRANT_COLORS[0]} opacity="0.85" />
                <path d="M130,130 L248,130 A118,118 0 0,1 130,248 Z" fill={QUADRANT_COLORS[1]} opacity="0.85" />
                <path d="M130,130 L130,248 A118,118 0 0,1 12,130 Z" fill={QUADRANT_COLORS[2]} opacity="0.85" />
                <path d="M130,130 L12,130 A118,118 0 0,1 130,12 Z" fill={QUADRANT_COLORS[3]} opacity="0.85" />
                <circle cx="130" cy="130" r="42" fill="#0B1026" stroke="#FBBF24" strokeWidth="3" />
                <text x="130" y="135" textAnchor="middle" fill="#FFFFFF" fontSize="14" fontWeight="800">
                  HOTS
                </text>
              </svg>
            </div>

            <div className="lg:col-span-7 grid grid-cols-1 gap-3 sm:grid-cols-2">
              {(hotsQuery.data?.quadrants ?? []).map((q, idx) => (
                <div
                  key={q.code}
                  className="rounded-2xl border border-indigo-400/20 bg-[#1E2A55]/75 p-4 space-y-2.5"
                >
                  <p
                    className="text-xs font-extrabold uppercase tracking-wider"
                    style={{ color: QUADRANT_COLORS[idx % QUADRANT_COLORS.length] }}
                  >
                    {q.name}
                  </p>
                  {q.heuristics.map((h) => (
                    <div
                      key={h.code}
                      className="flex items-center justify-between gap-2 rounded-xl bg-[#0B1026]/60 px-3 py-2 text-xs"
                    >
                      <span className="font-semibold text-white">{h.name}</span>
                      {h.skills[0] ? (
                        <button
                          type="button"
                          onClick={() => startMutation.mutate(h.skills[0]!.code)}
                          className="rounded-lg bg-[#6366F1] px-2.5 py-1 font-bold text-white hover:bg-[#5558E6] cursor-pointer"
                        >
                          {t("training:practiceBtn")}
                        </button>
                      ) : (
                        <span className="text-slate-500">—</span>
                      )}
                    </div>
                  ))}
                </div>
              ))}
            </div>
          </div>
        </GalaxyCard>
      ) : (
        <GalaxyCard className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            {/* Grade 1-6 Switcher */}
            <div className="flex flex-wrap gap-1.5">
              {[1, 2, 3, 4, 5, 6].map((g) => (
                <button
                  key={g}
                  type="button"
                  onClick={() => setGrade(g)}
                  className={`min-h-10 rounded-xl px-3.5 text-xs font-bold cursor-pointer ${
                    grade === g
                      ? "bg-[#6366F1] text-white"
                      : "bg-[#1E2A55] text-slate-200 hover:bg-[#2A3A73]"
                  }`}
                >
                  {t("training:gradeSelector", { grade: g })}
                </button>
              ))}
            </div>

            {/* High Score Mode vs Proficiency % Mode Toggle */}
            <div className="flex rounded-xl bg-[#0B1026] p-1">
              {(["PROFICIENCY", "HIGH_SCORE"] as const).map((m) => (
                <button
                  key={m}
                  type="button"
                  onClick={() => setViewMode(m)}
                  className={`min-h-9 rounded-lg px-3 text-xs font-bold cursor-pointer ${
                    viewMode === m ? "bg-[#22D3EE] text-slate-950" : "text-slate-300"
                  }`}
                >
                  {m === "PROFICIENCY" ? t("training:modeProficiency") : t("training:modeHighScore")}
                </button>
              ))}
            </div>
          </div>

          <div className="space-y-2.5">
            {(skillsQuery.data?.skills ?? []).map((sk) => (
              <div
                key={sk.code}
                className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-indigo-400/20 bg-[#1E2A55]/70 p-4"
              >
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="text-base font-bold text-white">{sk.name}</h3>
                    <DifficultyMeter
                      level={Math.min(5, Math.max(1, sk.difficulty)) as 1 | 2 | 3 | 4 | 5}
                    />
                  </div>
                  {viewMode === "PROFICIENCY" ? (
                    <div className="flex flex-wrap items-center gap-2 text-xs text-slate-300">
                      <ProficiencyPill
                        tier={sk.proficiencyTier}
                        label={t(`common:proficiency.${sk.proficiencyTier.toLowerCase()}`)}
                        pct={sk.proficiencyPct}
                      />
                      <span>{t("training:nationalAvg", { pct: sk.nationalAvgPct })}</span>
                      <span>
                        •{" "}
                        {t("training:yourProficiency", {
                          pct: sk.proficiencyPct,
                          correct: sk.correctCount,
                          attempts: sk.attemptsCount,
                        })}
                      </span>
                    </div>
                  ) : (
                    <div className="flex items-center gap-3 text-xs text-slate-300">
                      <StarRating stars={sk.bestStars as 0 | 1 | 2 | 3} />
                      <span className="planetlogi-tabular">{sk.bestSessionScore} / 10</span>
                    </div>
                  )}
                </div>

                <GalaxyButton
                  onClick={() => startMutation.mutate(sk.code)}
                  disabled={startMutation.isPending}
                >
                  {t("training:practiceBtn")} →
                </GalaxyButton>
              </div>
            ))}
          </div>
        </GalaxyCard>
      )}
    </div>
  );
}
