import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { CheckCircle2, PlayCircle, ScrollText } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import { CurrencyPill, GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { QuestionPlayer } from "../../../components/question-player/QuestionPlayer";
import { apiClient, useAuthStore } from "../../../store/authStore";

export function SpaceAcademyPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation(["academy", "common"]);
  const queryClient = useQueryClient();
  const user = useAuthStore((s) => s.user);
  const [grade, setGrade] = useState<number>(user?.grade ?? 1);
  const [activeLessonCode, setActiveLessonCode] = useState<string | null>(null);
  const [qIdx, setQIdx] = useState(1);

  const topicsQuery = useQuery({
    queryKey: ["academy-topics", grade, locale],
    queryFn: () => apiClient.getAcademyTopics(grade, locale),
  });

  const lessonQuery = useQuery({
    queryKey: ["academy-lesson", activeLessonCode, locale],
    queryFn: () => apiClient.getAcademyLesson(activeLessonCode!, locale),
    enabled: Boolean(activeLessonCode),
  });

  const saveMutation = useMutation({
    mutationFn: (vars: { questionIndex: number; answers: Record<string, string> }) =>
      apiClient.saveAcademyDraft(activeLessonCode!, vars.questionIndex, vars.answers, locale),
    onSuccess: (lesson) => {
      queryClient.setQueryData(["academy-lesson", activeLessonCode, locale], lesson);
    },
  });

  const checkMutation = useMutation({
    mutationFn: (vars: { questionIndex: number; answers: Record<string, string> }) =>
      apiClient.checkAcademyAnswer(activeLessonCode!, vars.questionIndex, vars.answers, locale),
    onSuccess: (data) => {
      queryClient.setQueryData(["academy-lesson", activeLessonCode, locale], data.lesson);
      void queryClient.invalidateQueries({ queryKey: ["academy-topics"] });
    },
  });

  const selfCheckMutation = useMutation({
    mutationFn: (vars: { itemKey: string; checked: boolean }) =>
      apiClient.toggleAcademySelfCheck(activeLessonCode!, vars.itemKey, vars.checked, locale),
    onSuccess: (lesson) => {
      queryClient.setQueryData(["academy-lesson", activeLessonCode, locale], lesson);
      void queryClient.invalidateQueries({ queryKey: ["academy-topics"] });
    },
  });

  // Active Lesson 4-Stage Timeline View
  if (activeLessonCode && lessonQuery.data) {
    const lesson = lessonQuery.data;
    const activeQ = lesson.doAndLearn[qIdx - 1] ?? lesson.doAndLearn[0]!;

    return (
      <div className="space-y-5">
        <GalaxyCard className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-xs font-bold uppercase text-cyan-300">
              {t("academy:ageTag", { band: lesson.ageBand })} • {lesson.completionPct}%
            </p>
            <h1 className="text-2xl font-extrabold text-white">{lesson.title}</h1>
          </div>
          <div className="flex items-center gap-2">
            {lesson.scrollEarned ? (
              <CurrencyPill kind="SCROLL" value="+1" label={t("common:currencies.scrolls")} />
            ) : null}
            <GalaxyButton variant="secondary" onClick={() => setActiveLessonCode(null)}>
              {t("academy:backToTopics")}
            </GalaxyButton>
          </div>
        </GalaxyCard>

        {/* Stage 1 & 2: Story Launch & Watch & Learn Videos */}
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <GalaxyCard className="space-y-2.5">
            <p className="text-xs font-bold uppercase text-amber-300">
              {t("academy:stages.storyLaunch")}
            </p>
            <div className="flex items-center gap-3 rounded-xl bg-[#0B1026] p-4 border border-indigo-400/20">
              <PlayCircle className="h-10 w-10 shrink-0 text-[#FBBF24]" aria-hidden="true" />
              <div>
                <p className="text-sm font-bold text-white">{lesson.storyVideoTitle}</p>
                <p className="text-xs text-slate-400">{t("academy:videoPlaceholder")}</p>
              </div>
            </div>
          </GalaxyCard>

          <GalaxyCard className="space-y-2.5">
            <p className="text-xs font-bold uppercase text-cyan-300">
              {t("academy:stages.watchLearn")}
            </p>
            <div className="flex items-center gap-3 rounded-xl bg-[#0B1026] p-4 border border-indigo-400/20">
              <PlayCircle className="h-10 w-10 shrink-0 text-[#22D3EE]" aria-hidden="true" />
              <div>
                <p className="text-sm font-bold text-white">{lesson.conceptVideoTitle}</p>
                <p className="text-xs text-slate-400">{t("academy:videoPlaceholder")}</p>
              </div>
            </div>
          </GalaxyCard>
        </div>

        {/* Stage 3: Do & Learn Guided Drills (with Save & Try Again!) */}
        <GalaxyCard className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-lg font-extrabold text-white">{t("academy:stages.doLearn")}</h2>
            <div className="flex gap-1.5">
              {lesson.doAndLearn.map((item) => (
                <button
                  key={item.index}
                  type="button"
                  onClick={() => setQIdx(item.index)}
                  className={`min-h-10 min-w-10 rounded-xl border text-xs font-bold cursor-pointer ${
                    item.index === activeQ.index
                      ? "border-[#22D3EE] bg-[#6366F1] text-white"
                      : item.solved
                        ? "border-emerald-400 bg-emerald-500/20 text-emerald-200"
                        : item.lastWrong
                          ? "border-red-400 bg-red-500/20 text-red-200"
                          : "border-indigo-400/25 bg-[#1E2A55] text-slate-200"
                  }`}
                >
                  Q{item.index}
                </button>
              ))}
            </div>
          </div>

          {lesson.scrollEarned ? (
            <div className="rounded-xl border border-emerald-400/40 bg-emerald-500/15 px-4 py-2 text-sm font-bold text-emerald-200">
              ✦ {t("academy:scrollEarnedBanner")}
            </div>
          ) : null}

          <QuestionPlayer
            mode="SPACE_ACADEMY"
            locale={locale}
            question={activeQ.question}
            locked={activeQ.solved || activeQ.lastWrong}
            isCorrect={activeQ.solved ? true : activeQ.lastWrong ? false : null}
            submittedAnswer={activeQ.draftAnswer}
            workedSolution={activeQ.workedSolution ?? null}
            submitting={checkMutation.isPending}
            onSaveDraft={(answers) => {
              saveMutation.mutate({ questionIndex: activeQ.index, answers });
            }}
            onSubmit={async (answers) => {
              await checkMutation.mutateAsync({ questionIndex: activeQ.index, answers });
            }}
            onNext={activeQ.index < 4 ? () => setQIdx(activeQ.index + 1) : undefined}
          />
        </GalaxyCard>

        {/* Stage 4: Mission Self-Check */}
        <GalaxyCard className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-lg font-extrabold text-white">{t("academy:stages.selfCheck")}</h2>
            <Link
              to="/$locale/training"
              params={{ locale }}
              className="text-xs font-bold text-cyan-300 hover:underline"
            >
              {t("academy:extraPractice")}
            </Link>
          </div>
          <div className="space-y-2">
            {lesson.selfCheck.map((item) => (
              <label
                key={item.key}
                className="flex min-h-11 items-center gap-3 rounded-xl border border-indigo-400/20 bg-[#1E2A55]/70 px-3.5 py-2 text-sm text-slate-100 cursor-pointer"
              >
                <input
                  type="checkbox"
                  checked={item.checked}
                  onChange={(e) =>
                    selfCheckMutation.mutate({ itemKey: item.key, checked: e.target.checked })
                  }
                />
                <span>{item.label}</span>
              </label>
            ))}
          </div>
        </GalaxyCard>
      </div>
    );
  }

  // Topic Hub View
  return (
    <div className="space-y-6">
      <GalaxyCard className="space-y-4">
        <div>
          <h1 className="text-2xl font-extrabold text-white">🚀 {t("academy:title")}</h1>
          <p className="text-sm text-slate-300">{t("academy:subtitle")}</p>
        </div>

        <div className="flex flex-wrap gap-1.5">
          {[1, 2, 3, 4, 5, 6].map((g) => (
            <button
              key={g}
              type="button"
              onClick={() => setGrade(g)}
              className={`min-h-10 rounded-xl px-3.5 text-xs font-bold cursor-pointer ${
                grade === g ? "bg-[#6366F1] text-white" : "bg-[#1E2A55] text-slate-200 hover:bg-[#2A3A73]"
              }`}
            >
              {t("common:common.grade", { grade: g })}
            </button>
          ))}
        </div>
      </GalaxyCard>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {(topicsQuery.data?.topics ?? []).map((topic) => (
          <GalaxyCard key={topic.code} className="space-y-3">
            <div className="flex items-start justify-between gap-2">
              <div>
                <h2 className="text-lg font-extrabold text-white">{topic.title}</h2>
                <p className="text-xs text-slate-300">
                  {t("academy:ageTag", { band: topic.ageBand })}
                </p>
              </div>
              <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/15 border border-emerald-400/35 px-2.5 py-1 text-xs font-bold text-emerald-200">
                <ScrollText className="h-3.5 w-3.5" aria-hidden="true" />
                {t("academy:scrollsProgress", {
                  earned: topic.scrollsEarned,
                  total: topic.totalScrolls,
                })}
              </span>
            </div>

            <div className="space-y-2">
              {topic.lessons.map((l) => (
                <div
                  key={l.code}
                  className="flex items-center justify-between gap-3 rounded-xl bg-[#1E2A55]/80 px-3.5 py-2.5"
                >
                  <div>
                    <p className="text-sm font-bold text-white flex items-center gap-1.5">
                      {l.scrollEarned ? (
                        <CheckCircle2 className="h-4 w-4 text-emerald-400" aria-hidden="true" />
                      ) : null}
                      <span>{l.title}</span>
                    </p>
                    <p className="text-xs text-slate-300">{l.completionPct}%</p>
                  </div>
                  <GalaxyButton
                    onClick={() => {
                      setActiveLessonCode(l.code);
                      setQIdx(1);
                    }}
                  >
                    {t("academy:openLesson")}
                  </GalaxyButton>
                </div>
              ))}
            </div>
          </GalaxyCard>
        ))}
      </div>
    </div>
  );
}
