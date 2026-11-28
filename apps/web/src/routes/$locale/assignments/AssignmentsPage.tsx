import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { Locale } from "@planetlogi/i18n";
import { CurrencyPill, GalaxyButton, GalaxyCard, ProficiencyPill } from "@planetlogi/ui";
import { QuestionPlayer } from "../../../components/question-player/QuestionPlayer";
import { apiClient, useAuthStore } from "../../../store/authStore";

export function AssignmentsPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation(["portals", "common"]);
  const queryClient = useQueryClient();
  const user = useAuthStore((s) => s.user);

  const [statusTab, setStatusTab] = useState<"ALL" | "INCOMPLETE" | "COMPLETE">("ALL");
  const [assignedBy, setAssignedBy] = useState("ALL");
  const [showBuilder, setShowBuilder] = useState(false);

  // Parent Custom Builder state
  const [customTitle, setCustomTitle] = useState("");
  const [dueDate, setDueDate] = useState("2026-12-05");
  const [selectedTemplates, setSelectedTemplates] = useState<string[]>([]);
  const [builderToast, setBuilderToast] = useState<string | null>(null);
  const [builderError, setBuilderError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Active assignment solver state
  const [activeId, setActiveId] = useState<string | null>(null);
  const [qIdx, setQIdx] = useState(1);

  // The profile bundle resolves the authenticated parent's linked child (or
  // the student's own profile), so the builder never targets a demo ID.
  const bundleQuery = useQuery({
    queryKey: ["profile-bundle"],
    queryFn: () => apiClient.getProfileBundle(),
  });
  const linkedChild = bundleQuery.data?.child;
  const grade = linkedChild?.grade ?? user?.grade ?? 1;
  const isParent = user?.role === "PARENT";

  const listQuery = useQuery({
    queryKey: ["assignments", statusTab, assignedBy],
    queryFn: () => apiClient.getAssignments(statusTab, assignedBy, 0),
  });

  const skillsQuery = useQuery({
    queryKey: ["training-skills", "ALL", grade, locale],
    queryFn: () => apiClient.getSkills("ALL", grade, locale),
    enabled: showBuilder,
  });

  const detailQuery = useQuery({
    queryKey: ["assignment-detail", activeId, locale],
    queryFn: () => apiClient.getAssignmentDetail(activeId!, locale),
    enabled: Boolean(activeId),
  });

  const createMutation = useMutation({
    mutationFn: () => {
      if (!linkedChild?.studentId) {
        throw new Error("no linked child");
      }
      return apiClient.createCustomAssignment({
        studentId: linkedChild.studentId,
        title: customTitle.trim() || t("portals:assignments.defaultTitle"),
        dueDate,
        templateCodes: selectedTemplates,
      });
    },
    onSuccess: () => {
      setBuilderToast(t("portals:assignments.createdSuccess"));
      setBuilderError(null);
      setCustomTitle("");
      setSelectedTemplates([]);
      void queryClient.invalidateQueries({ queryKey: ["assignments"] });
    },
    onError: () => {
      setBuilderToast(null);
      setBuilderError(t("portals:assignments.createError"));
    },
  });

  const submitMutation = useMutation({
    mutationFn: (vars: { questionIndex: number; answers: Record<string, string> }) =>
      apiClient.submitAssignmentQuestion(activeId!, vars.questionIndex, vars.answers, locale),
    onSuccess: (data) => {
      queryClient.setQueryData(["assignment-detail", activeId, locale], data.assignment);
      void queryClient.invalidateQueries({ queryKey: ["assignments"] });
      setActionError(null);
    },
    onError: () => setActionError(t("portals:assignments.submitError")),
  });

  if (activeId && detailQuery.isError) {
    return (
      <GalaxyCard className="space-y-3 text-center">
        <p role="alert" className="text-sm font-bold text-red-300">
          {t("portals:assignments.loadError")}
        </p>
        <GalaxyButton variant="secondary" onClick={() => setActiveId(null)}>
          {t("portals:assignments.back")}
        </GalaxyButton>
      </GalaxyCard>
    );
  }

  if (activeId && !detailQuery.data) {
    return (
      <GalaxyCard className="text-center text-slate-300">{t("common:common.loading")}</GalaxyCard>
    );
  }

  if (activeId && detailQuery.data) {
    const detail = detailQuery.data;
    const activeQ = detail.questions[qIdx - 1] ?? detail.questions[0]!;
    return (
      <div className="space-y-4">
        <GalaxyCard className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-xs font-bold text-cyan-300">
              {detail.summary.assignedByName} • {detail.summary.completedCount}/
              {detail.summary.questionCount}
            </p>
            <h1 className="text-xl font-extrabold text-white">{detail.summary.title}</h1>
          </div>
          <GalaxyButton variant="secondary" onClick={() => setActiveId(null)}>
            ← {t("portals:assignments.back")}
          </GalaxyButton>
        </GalaxyCard>

        <QuestionPlayer
          mode="GALAXY_TRAINING"
          locale={locale}
          question={activeQ.question}
          locked={activeQ.isCorrect !== undefined}
          isCorrect={activeQ.isCorrect ?? null}
          submittedAnswer={activeQ.submittedAnswer}
          workedSolution={activeQ.workedSolution ?? null}
          submitting={submitMutation.isPending}
          onSubmit={async (answers) => {
            await submitMutation.mutateAsync({ questionIndex: activeQ.index, answers });
          }}
          onNext={
            activeQ.index < detail.questions.length ? () => setQIdx(activeQ.index + 1) : undefined
          }
        />

        {actionError ? (
          <p role="alert" className="text-sm font-bold text-red-300">
            {actionError}
          </p>
        ) : null}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <GalaxyCard className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-2xl font-extrabold text-white">📋 {t("portals:assignments.title")}</h1>
            <p className="text-sm text-slate-300">{t("portals:assignments.subtitle")}</p>
          </div>
          {isParent ? (
            <GalaxyButton variant="gold" onClick={() => setShowBuilder((v) => !v)}>
              {t("portals:assignments.createCustomBtn")}
            </GalaxyButton>
          ) : null}
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex gap-1.5">
            {(
              [
                ["ALL", t("portals:assignments.tabAll")],
                ["INCOMPLETE", t("portals:assignments.tabIncomplete")],
                ["COMPLETE", t("portals:assignments.tabComplete")],
              ] as const
            ).map(([k, label]) => (
              <button
                key={k}
                type="button"
                onClick={() => setStatusTab(k)}
                className={`min-h-10 rounded-xl px-3.5 text-xs font-bold cursor-pointer ${
                  statusTab === k ? "bg-[#6366F1] text-white" : "bg-[#1E2A55] text-slate-200"
                }`}
              >
                {label}
              </button>
            ))}
          </div>

          <select
            value={assignedBy}
            onChange={(e) => setAssignedBy(e.target.value)}
            aria-label={t("portals:assignments.assignedByFilter")}
            className="min-h-10 rounded-xl border border-indigo-400/30 bg-[#0B1026] px-3 text-xs font-semibold text-white"
          >
            <option value="ALL">{t("portals:assignments.assignedByAll")}</option>
            <option value="SYSTEM">{t("portals:assignments.assignedBySystem")}</option>
            <option value="PARENT">{t("portals:assignments.assignedByParent")}</option>
            <option value="TEACHER">{t("portals:assignments.assignedByTeacher")}</option>
          </select>
        </div>
      </GalaxyCard>

      {/* Parent Personalised Assignment Builder */}
      {isParent && showBuilder ? (
        <GalaxyCard className="border-amber-400/40 space-y-4">
          <div>
            <h2 className="text-lg font-extrabold text-[#FBBF24]">
              {t("portals:assignments.builderTitle")}
            </h2>
            <p className="text-xs text-slate-300">{t("portals:assignments.builderSubtitle")}</p>
            {linkedChild ? (
              <p className="text-xs font-bold text-cyan-200">
                {t("portals:assignments.assigningTo", {
                  name: linkedChild.firstName,
                  grade: linkedChild.grade,
                })}
              </p>
            ) : bundleQuery.isError ? (
              <p role="alert" className="text-xs font-bold text-red-300">
                {t("portals:assignments.childLoadError")}
              </p>
            ) : null}
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label className="block text-xs font-bold text-slate-200">
                {t("portals:assignments.titleLabel")}
              </label>
              <input
                type="text"
                value={customTitle}
                onChange={(e) => setCustomTitle(e.target.value)}
                placeholder={t("portals:assignments.titlePlaceholder")}
                className="mt-1 min-h-11 w-full rounded-xl border border-indigo-400/30 bg-[#0B1026] px-3 text-sm text-white"
              />
            </div>
            <div>
              <label className="block text-xs font-bold text-slate-200">
                {t("portals:assignments.dueDateLabel")}
              </label>
              <input
                type="date"
                value={dueDate}
                onChange={(e) => setDueDate(e.target.value)}
                className="mt-1 min-h-11 w-full rounded-xl border border-indigo-400/30 bg-[#0B1026] px-3 text-sm text-white"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            {(skillsQuery.data?.skills ?? []).map((sk) => {
              const checked = selectedTemplates.includes(sk.templateCode);
              return (
                <label
                  key={sk.code}
                  className="flex min-h-12 items-center justify-between gap-2 rounded-xl border border-indigo-400/20 bg-[#1E2A55]/75 px-3 py-2 text-xs text-white cursor-pointer"
                >
                  <span className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() =>
                        setSelectedTemplates((prev) =>
                          checked
                            ? prev.filter((c) => c !== sk.templateCode)
                            : [...prev, sk.templateCode]
                        )
                      }
                    />
                    <span className="font-semibold">{sk.name}</span>
                  </span>
                  <ProficiencyPill
                    tier={sk.proficiencyTier}
                    label={t(`common:proficiency.${sk.proficiencyTier.toLowerCase()}`)}
                    pct={sk.proficiencyPct}
                  />
                </label>
              );
            })}
          </div>

          {builderToast ? <p className="text-sm font-bold text-emerald-300">✓ {builderToast}</p> : null}
          {builderError ? (
            <p role="alert" className="text-sm font-bold text-red-300">
              {builderError}
            </p>
          ) : null}

          <GalaxyButton
            variant="gold"
            disabled={
              selectedTemplates.length === 0 ||
              !linkedChild?.studentId ||
              createMutation.isPending
            }
            onClick={() => createMutation.mutate()}
          >
            {t("portals:assignments.assignNow")} ({selectedTemplates.length})
          </GalaxyButton>
        </GalaxyCard>
      ) : null}

      {/* Student Assignment Cards */}
      {listQuery.isError ? (
        <GalaxyCard className="space-y-3 text-center">
          <p role="alert" className="text-sm font-bold text-red-300">
            {t("portals:assignments.loadError")}
          </p>
          <GalaxyButton variant="secondary" onClick={() => void listQuery.refetch()}>
            {t("common:common.retry")}
          </GalaxyButton>
        </GalaxyCard>
      ) : listQuery.isPending ? (
        <GalaxyCard className="text-center text-slate-300">{t("common:common.loading")}</GalaxyCard>
      ) : null}

      <div className="space-y-3">
        {(listQuery.data?.assignments ?? []).map((item) => (
          <GalaxyCard key={item.id} className="flex flex-wrap items-center justify-between gap-3">
            <div className="space-y-1">
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="text-base font-extrabold text-white">{item.title}</h2>
                <span className="rounded-lg bg-indigo-500/30 px-2 py-0.5 text-[11px] font-bold text-indigo-200">
                  {item.assignedByName}
                </span>
              </div>
              <p className="text-xs text-slate-300">
                {t("portals:assignments.dueLabel", { date: item.dueDate })} • {item.completedCount}/
                {t("portals:assignments.questionShort", { count: item.questionCount })} •{" "}
                {t(`portals:assignments.statuses.${item.status}`)}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <CurrencyPill kind="AC" value={`+${item.rewardAc}`} label="AC" />
              <GalaxyButton
                onClick={() => {
                  setActiveId(item.id);
                  setQIdx(1);
                  setActionError(null);
                }}
              >
                {t("portals:assignments.open")} →
              </GalaxyButton>
            </div>
          </GalaxyCard>
        ))}
      </div>
    </div>
  );
}
