import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { CheckCircle2, PlayCircle, ScrollText } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import { CurrencyPill, GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { QuestionPlayer } from "../../../components/question-player/QuestionPlayer";
import { apiClient, currentSessionGeneration, useAuthStore } from "../../../store/authStore";

type LessonMutationVars = {
  sessionKey: string;
  generation: number;
  lessonCode: string;
  locale: string;
  questionIndex: number;
  answers: Record<string, string>;
};

const YOUTUBE_ID = /^[A-Za-z0-9_-]{11}$/;

function LessonVideo({
  title,
  youtubeId,
  url,
  captionsUrl,
  locale,
  placeholder,
}: {
  title: string;
  youtubeId?: string;
  url?: string;
  captionsUrl?: string;
  locale: string;
  placeholder: string;
}) {
  return (
    <div className="space-y-2">
      <p className="text-sm font-bold text-white">{title}</p>
      {youtubeId && YOUTUBE_ID.test(youtubeId) ? (
        // Privacy-enhanced embed (no cookies until playback) with captions in the UI locale.
        <iframe
          src={`https://www.youtube-nocookie.com/embed/${youtubeId}?rel=0&modestbranding=1&playsinline=1&cc_load_policy=1&cc_lang_pref=${locale}&hl=${locale}`}
          title={title}
          className="aspect-video w-full rounded-xl border border-indigo-400/20 bg-black"
          allow="accelerometer; encrypted-media; gyroscope; picture-in-picture; fullscreen"
          referrerPolicy="strict-origin-when-cross-origin"
          loading="lazy"
          allowFullScreen
        />
      ) : url ? (
        <video
          controls
          preload="metadata"
          className="aspect-video w-full rounded-xl border border-indigo-400/20 bg-black"
          aria-label={title}
        >
          <source src={url} />
          {captionsUrl ? (
            <track
              kind="captions"
              src={captionsUrl}
              srcLang={locale}
              label={locale === "id" ? "Bahasa Indonesia" : "English"}
              default
            />
          ) : null}
        </video>
      ) : (
        <div className="flex items-center gap-3 rounded-xl border border-indigo-400/20 bg-[#0B1026] p-4">
          <PlayCircle className="h-10 w-10 shrink-0 text-[#22D3EE]" aria-hidden="true" />
          <p className="text-xs text-slate-400">{placeholder}</p>
        </div>
      )}
    </div>
  );
}

export function SpaceAcademyPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation(["academy", "common"]);
  const queryClient = useQueryClient();
  const user = useAuthStore((s) => s.user);
  // Derive the grade until the learner explicitly picks one, because auth
  // hydration can finish after this page's first render.
  const [selectedGrade, setSelectedGrade] = useState<number | null>(null);
  const grade = selectedGrade ?? user?.grade ?? 1;
  const [activeLessonCode, setActiveLessonCode] = useState<string | null>(null);
  const [qIdx, setQIdx] = useState(1);
  const [actionError, setActionError] = useState<string | null>(null);
  const sessionKey = user?.id ?? "guest";

  const topicsQuery = useQuery({
    queryKey: ["academy-topics", sessionKey, grade, locale],
    queryFn: () => apiClient.getAcademyTopics(grade, locale),
  });

  const lessonQuery = useQuery({
    queryKey: ["academy-lesson", sessionKey, activeLessonCode, locale],
    queryFn: () => apiClient.getAcademyLesson(activeLessonCode!, locale),
    enabled: Boolean(activeLessonCode),
  });

  const saveMutation = useMutation({
    mutationFn: (vars: LessonMutationVars) =>
      apiClient.saveAcademyDraft(vars.lessonCode, vars.questionIndex, vars.answers, vars.locale),
    onSuccess: (lesson, vars) => {
      if (currentSessionGeneration() !== vars.generation) return;
      queryClient.setQueryData(["academy-lesson", vars.sessionKey, vars.lessonCode, vars.locale], lesson);
      setActionError(null);
    },
    onError: () => setActionError(t("academy:actionError")),
  });

  const checkMutation = useMutation({
    mutationFn: (vars: LessonMutationVars) =>
      apiClient.checkAcademyAnswer(vars.lessonCode, vars.questionIndex, vars.answers, vars.locale),
    onSuccess: (data, vars) => {
      if (currentSessionGeneration() !== vars.generation) return;
      queryClient.setQueryData(["academy-lesson", vars.sessionKey, vars.lessonCode, vars.locale], data.lesson);
      void queryClient.invalidateQueries({ queryKey: ["academy-topics", vars.sessionKey] });
      setActionError(null);
    },
    onError: () => setActionError(t("academy:actionError")),
  });

  const selfCheckMutation = useMutation({
    mutationFn: (vars: {
      sessionKey: string;
      generation: number;
      lessonCode: string;
      locale: string;
      itemKey: string;
      checked: boolean;
    }) => apiClient.toggleAcademySelfCheck(vars.lessonCode, vars.itemKey, vars.checked, vars.locale),
    onSuccess: (lesson, vars) => {
      if (currentSessionGeneration() !== vars.generation) return;
      queryClient.setQueryData(["academy-lesson", vars.sessionKey, vars.lessonCode, vars.locale], lesson);
      void queryClient.invalidateQueries({ queryKey: ["academy-topics", vars.sessionKey] });
      setActionError(null);
    },
    onError: () => setActionError(t("academy:actionError")),
  });

  const retryMutation = useMutation({
    mutationFn: (vars: {
      sessionKey: string;
      generation: number;
      lessonCode: string;
      locale: string;
    }) => apiClient.retryAcademyScroll(vars.lessonCode, vars.locale),
    onSuccess: (lesson, vars) => {
      if (currentSessionGeneration() !== vars.generation) return;
      queryClient.setQueryData(["academy-lesson", vars.sessionKey, vars.lessonCode, vars.locale], lesson);
      void queryClient.invalidateQueries({ queryKey: ["academy-topics", vars.sessionKey] });
      setActionError(null);
    },
    onError: () => setActionError(t("academy:actionError")),
  });

  if (activeLessonCode && lessonQuery.isError) {
    return (
      <GalaxyCard className="space-y-3 text-center">
        <p role="alert" className="text-sm font-bold text-red-300">
          {t("academy:loadError")}
        </p>
        <GalaxyButton variant="secondary" onClick={() => setActiveLessonCode(null)}>
          {t("academy:backToTopics")}
        </GalaxyButton>
      </GalaxyCard>
    );
  }

  if (activeLessonCode && !lessonQuery.data) {
    return (
      <GalaxyCard className="text-center text-slate-300">{t("common:common.loading")}</GalaxyCard>
    );
  }

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
            <LessonVideo
              title={lesson.storyVideoTitle}
              youtubeId={lesson.storyYoutubeId}
              url={lesson.storyVideoUrl}
              captionsUrl={lesson.storyCaptionsUrl}
              locale={locale}
              placeholder={t("academy:videoPlaceholder")}
            />
          </GalaxyCard>

          <GalaxyCard className="space-y-2.5">
            <p className="text-xs font-bold uppercase text-cyan-300">
              {t("academy:stages.watchLearn")}
            </p>
            <LessonVideo
              title={lesson.conceptVideoTitle}
              youtubeId={lesson.conceptYoutubeId}
              url={lesson.conceptVideoUrl}
              captionsUrl={lesson.conceptCaptionsUrl}
              locale={locale}
              placeholder={t("academy:videoPlaceholder")}
            />
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
                  className={`min-h-11 min-w-11 rounded-xl border text-xs font-bold cursor-pointer ${
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
          ) : lesson.doAndLearn.every((item) => item.solved) ? (
            <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-amber-400/40 bg-amber-400/10 px-4 py-2">
              <p className="text-sm font-bold text-amber-200">{t("academy:scrollRetryHint")}</p>
              <GalaxyButton
                variant="gold"
                disabled={retryMutation.isPending}
                onClick={() =>
                  retryMutation.mutate({
                    sessionKey,
                    generation: currentSessionGeneration(),
                    lessonCode: lesson.code,
                    locale,
                  })
                }
              >
                {t("academy:retryScroll")}
              </GalaxyButton>
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
            onSaveDraft={(answers) =>
              saveMutation.mutateAsync({
                sessionKey,
                generation: currentSessionGeneration(),
                lessonCode: lesson.code,
                locale,
                questionIndex: activeQ.index,
                answers,
              })
            }
            onSubmit={async (answers) => {
              await checkMutation.mutateAsync({
                sessionKey,
                generation: currentSessionGeneration(),
                lessonCode: lesson.code,
                locale,
                questionIndex: activeQ.index,
                answers,
              });
            }}
            onNext={activeQ.index < 4 ? () => setQIdx(activeQ.index + 1) : undefined}
          />

          {actionError ? (
            <p role="alert" className="text-sm font-bold text-red-300">
              {actionError}
            </p>
          ) : null}
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
                    selfCheckMutation.mutate({
                      sessionKey,
                      generation: currentSessionGeneration(),
                      lessonCode: lesson.code,
                      locale,
                      itemKey: item.key,
                      checked: e.target.checked,
                    })
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
              onClick={() => setSelectedGrade(g)}
              className={`min-h-11 rounded-xl px-3.5 text-xs font-bold cursor-pointer ${
                grade === g ? "bg-[#6366F1] text-white" : "bg-[#1E2A55] text-slate-200 hover:bg-[#2A3A73]"
              }`}
            >
              {t("common:common.grade", { grade: g })}
            </button>
          ))}
        </div>
      </GalaxyCard>

      {topicsQuery.isError ? (
        <GalaxyCard className="space-y-3 text-center">
          <p role="alert" className="text-sm font-bold text-red-300">
            {t("academy:loadError")}
          </p>
          <GalaxyButton variant="secondary" onClick={() => void topicsQuery.refetch()}>
            {t("common:common.retry")}
          </GalaxyButton>
        </GalaxyCard>
      ) : topicsQuery.isPending ? (
        <GalaxyCard className="text-center text-slate-300">{t("common:common.loading")}</GalaxyCard>
      ) : null}

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
                      setActionError(null);
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
