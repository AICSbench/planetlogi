import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Flag, HelpCircle, PlayCircle, Sparkles, Volume2 } from "lucide-react";
import type { RenderedQuestion, WorkedSolution } from "@planetlogi/api-client";
import { DifficultyMeter, GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { DiagramRenderer } from "./DiagramRenderer";
import { MathToolsDrawer } from "./tools/MathToolsDrawer";

export type PlayerMode = "DAILY_ORBIT" | "GALAXY_TRAINING" | "SPACE_ACADEMY" | "ASTRO_QUIZ";

export interface QuestionPlayerProps {
  mode: PlayerMode;
  question: RenderedQuestion;
  locale: "en" | "id";
  locked?: boolean;
  isCorrect?: boolean | null;
  submittedAnswer?: Record<string, string>;
  workedSolution?: WorkedSolution | null;
  submitting?: boolean;
  onSubmit: (answers: Record<string, string>) => Promise<void> | void;
  onSkip?: () => Promise<void> | void;
  onNext?: () => void;
  /** May return a promise; "Saved" is shown only after it resolves, unless it resolves to false. */
  onSaveDraft?: (answers: Record<string, string>) => void | Promise<unknown>;
  onDraftChange?: (answers: Record<string, string>) => void;
  extraHeaderActions?: React.ReactNode;
}

export function QuestionPlayer(props: QuestionPlayerProps) {
  return <QuestionPlayerInner key={props.question.id} {...props} />;
}

function QuestionPlayerInner({
  mode,
  question,
  locale,
  locked = false,
  isCorrect = null,
  submittedAnswer,
  workedSolution,
  submitting = false,
  onSubmit,
  onSkip,
  onNext,
  onSaveDraft,
  onDraftChange,
  extraHeaderActions,
}: QuestionPlayerProps) {
  const { t } = useTranslation("player");
  const [draftAnswers, setDraftAnswers] = useState<Record<string, string> | null>(null);
  const [showHint, setShowHint] = useState(false);
  const [showSkipConfirm, setShowSkipConfirm] = useState(false);
  const [showFeedback, setShowFeedback] = useState(false);
  const [feedbackSent, setFeedbackSent] = useState(false);
  const [feedbackText, setFeedbackText] = useState("");
  const [savedNotice, setSavedNotice] = useState(false);
  const [retryUnlocked, setRetryUnlocked] = useState(false);
  const draftRevision = useRef(0);

  const effectiveLocked =
    (locked && !(mode === "SPACE_ACADEMY" && retryUnlocked && isCorrect === false)) || submitting;
  const answers = (effectiveLocked ? submittedAnswer : draftAnswers ?? submittedAnswer) ?? {};

  function speakQuestion() {
    if (typeof window === "undefined" || !("speechSynthesis" in window)) return;
    window.speechSynthesis.cancel();
    const utter = new SpeechSynthesisUtterance(question.ttsText || question.stem);
    utter.lang = locale === "id" ? "id-ID" : "en-US";
    utter.rate = question.grade <= 2 ? 0.92 : 1.0;
    window.speechSynthesis.speak(utter);
  }

  function handleFieldChange(key: string, val: string) {
    const next = { ...answers, [key]: val };
    draftRevision.current += 1;
    setSavedNotice(false);
    setDraftAnswers(next);
    onDraftChange?.(next);
  }

  async function handleFormSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (effectiveLocked || submitting) return;
    setRetryUnlocked(false);
    await onSubmit(answers);
  }

  const parts =
    question.parts && question.parts.length > 0 ? question.parts : [{ key: "answer" }];

  return (
    <div className="space-y-4">
      {/* Crisp Light Study Surface for glare-free elementary reading (>14:1 contrast) */}
      <GalaxyCard surface="paper" className="space-y-5 p-5 sm:p-7">
        {/* Question Header Strip */}
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-slate-200 pb-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="rounded-lg bg-indigo-100 px-2.5 py-1 text-xs font-bold text-indigo-800">
              {question.skillName}
            </span>
            <span className="font-mono text-xs text-slate-500">{question.id}</span>
            <DifficultyMeter
              level={(Math.min(5, Math.max(1, question.difficulty)) as 1 | 2 | 3 | 4 | 5)}
            />
          </div>

          <div className="flex flex-wrap items-center gap-1.5">
            <button
              type="button"
              onClick={speakQuestion}
              className="inline-flex min-h-10 items-center gap-1.5 rounded-xl border border-slate-300 bg-white px-3 text-xs font-bold text-slate-800 hover:bg-indigo-50 cursor-pointer"
            >
              <Volume2 className="h-4 w-4 text-[#6366F1]" aria-hidden="true" />
              <span>{t("readAloud")}</span>
            </button>
            {question.hints && question.hints.length > 0 ? (
              <button
                type="button"
                onClick={() => setShowHint((h) => !h)}
                className="inline-flex min-h-10 items-center gap-1 rounded-xl border border-slate-300 bg-white px-3 text-xs font-bold text-slate-800 hover:bg-amber-50 cursor-pointer"
              >
                <HelpCircle className="h-4 w-4 text-amber-500" aria-hidden="true" />
                <span>{t("hint")}</span>
              </button>
            ) : null}
            {question.videoTutorialId ? (
              <button
                type="button"
                className="inline-flex min-h-10 items-center gap-1 rounded-xl border border-slate-300 bg-white px-3 text-xs font-bold text-slate-800 hover:bg-cyan-50 cursor-pointer"
              >
                <PlayCircle className="h-4 w-4 text-cyan-600" aria-hidden="true" />
                <span>{t("videoTutorial")}</span>
              </button>
            ) : null}
            {extraHeaderActions}
            <button
              type="button"
              onClick={() => {
                setShowFeedback(true);
                setFeedbackSent(false);
              }}
              className="inline-flex min-h-10 items-center gap-1 rounded-xl border border-slate-200 px-2.5 text-xs font-semibold text-slate-500 hover:bg-slate-100 cursor-pointer"
            >
              <Flag className="h-3.5 w-3.5" aria-hidden="true" />
              <span>{t("reportQuestion")}</span>
            </button>
          </div>
        </div>

        {/* HOTS Recommended Heuristic Tag */}
        {question.heuristicName ? (
          <div className="inline-flex items-center gap-1.5 rounded-full bg-purple-100 px-3 py-1 text-xs font-bold text-purple-800">
            <Sparkles className="h-3.5 w-3.5" aria-hidden="true" />
            <span>{t("recommendedHeuristic", { name: question.heuristicName })}</span>
          </div>
        ) : null}

        {/* Question Stem */}
        <p className="text-lg sm:text-xl font-semibold leading-relaxed text-[#111827]">
          {question.stem}
        </p>

        {/* Optional Visual Diagram */}
        {question.diagram ? <DiagramRenderer diagram={question.diagram} /> : null}

        {/* Optional Hint Box */}
        {showHint && question.hints.length > 0 ? (
          <div className="rounded-xl border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-900">
            💡 {question.hints[0]}
          </div>
        ) : null}

        {/* Interactive Answer Form */}
        <form onSubmit={handleFormSubmit} className="space-y-4">
          {question.inputType === "mcq" && question.options ? (
            <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-3">
              {question.options.map((opt) => {
                const selected = answers.choice === opt.key;
                return (
                  <button
                    key={opt.key}
                    type="button"
                    disabled={effectiveLocked}
                    onClick={() => handleFieldChange("choice", opt.key)}
                    className={`min-h-14 rounded-xl border-2 px-4 py-3 text-left text-base font-bold transition-colors cursor-pointer ${
                      selected
                        ? "border-[#6366F1] bg-indigo-50 text-indigo-950"
                        : "border-slate-300 bg-white text-slate-900 hover:border-indigo-300"
                    }`}
                  >
                    <span className="mr-2 inline-flex h-7 w-7 items-center justify-center rounded-full bg-slate-200 text-xs font-extrabold">
                      {opt.key}
                    </span>
                    {opt.label}
                  </button>
                );
              })}
            </div>
          ) : (
            <div className="flex flex-wrap items-end gap-4">
              {parts.map((part) => (
                <div key={part.key} className="space-y-1">
                  {part.label ? (
                    <label
                      htmlFor={`ans-${part.key}`}
                      className="block text-xs font-bold uppercase text-slate-600"
                    >
                      {part.label}
                    </label>
                  ) : null}
                  <div className="flex items-center gap-2">
                    {part.prefix ? (
                      <span className="text-base font-bold text-slate-700">{part.prefix}</span>
                    ) : null}
                    <input
                      id={`ans-${part.key}`}
                      type="text"
                      inputMode={question.inputType === "fraction" ? "text" : "decimal"}
                      disabled={effectiveLocked}
                      value={answers[part.key] ?? ""}
                      onChange={(e) => handleFieldChange(part.key, e.target.value)}
                      placeholder={question.inputType === "fraction" ? "e.g. 3/4" : "?"}
                      className="min-h-12 w-36 rounded-xl border-2 border-slate-300 bg-white px-4 text-lg font-extrabold text-slate-900 planetlogi-tabular focus:border-[#6366F1] focus:outline-none disabled:bg-slate-100"
                    />
                    {part.suffix ? (
                      <span className="text-base font-semibold text-slate-700">{part.suffix}</span>
                    ) : null}
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Action Buttons */}
          <div className="flex flex-wrap items-center gap-2.5 pt-2">
            {!effectiveLocked ? (
              <GalaxyButton type="submit" size="lg" disabled={submitting}>
                {mode === "SPACE_ACADEMY" ? t("checkAnswer") : t("submitAnswer")}
              </GalaxyButton>
            ) : null}

            {mode === "SPACE_ACADEMY" && locked && isCorrect === false && !retryUnlocked ? (
              <GalaxyButton
                type="button"
                variant="gold"
                size="lg"
                onClick={() => setRetryUnlocked(true)}
              >
                {t("tryAgain")}
              </GalaxyButton>
            ) : null}

            {(mode === "SPACE_ACADEMY" || mode === "ASTRO_QUIZ") && onSaveDraft ? (
              <GalaxyButton
                type="button"
                variant="secondary"
                disabled={submitting}
                onClick={async () => {
                  const revision = draftRevision.current;
                  setSavedNotice(false);
                  try {
                    const saved = await onSaveDraft(answers);
                    if (saved !== false && draftRevision.current === revision) setSavedNotice(true);
                  } catch {
                    // Keep "Save draft" available; the page shows the error.
                  }
                }}
              >
                {savedNotice ? t("saved") : t("saveDraft")}
              </GalaxyButton>
            ) : null}

            {!effectiveLocked && onSkip ? (
              <button
                type="button"
                onClick={() => setShowSkipConfirm(true)}
                className="min-h-11 rounded-xl border border-slate-300 px-4 text-sm font-semibold text-slate-700 hover:bg-slate-100 cursor-pointer"
              >
                {t("showAnswer")}
              </button>
            ) : null}

            {locked && onNext ? (
              <GalaxyButton type="button" variant="gold" size="lg" onClick={onNext}>
                {t("nextQuestion")} →
              </GalaxyButton>
            ) : null}
          </div>
        </form>

        {/* Feedback Banner & Worked Solution */}
        {locked && isCorrect !== null ? (
          <div
            className={`rounded-2xl border-2 p-4 space-y-3 ${
              isCorrect
                ? "border-emerald-400 bg-emerald-50 text-emerald-950"
                : "border-amber-400 bg-amber-50 text-amber-950"
            }`}
          >
            <p className="text-base font-extrabold">
              {isCorrect ? `🎉 ${t("correctBanner")}` : `✨ ${t("wrongBanner")}`}
            </p>
            {workedSolution ? (
              <div className="space-y-2 border-t border-black/10 pt-3">
                <p className="text-xs font-bold uppercase tracking-wider opacity-80">
                  {t("workedSolution")}
                </p>
                <p className="text-sm font-bold">
                  {t("correctAnswer")}:{" "}
                  <span className="font-mono">
                    {Object.values(workedSolution.correctAnswers).join(", ")}
                  </span>
                </p>
                <ol className="list-decimal pl-5 space-y-2 text-sm">
                  {workedSolution.steps.map((step, idx) => (
                    <li key={idx} className="space-y-2">
                      <span>{step.text}</span>
                      {step.diagram ? <DiagramRenderer diagram={step.diagram} /> : null}
                    </li>
                  ))}
                </ol>
              </div>
            ) : null}
          </div>
        ) : null}
      </GalaxyCard>

      {/* 4 Interactive On-Screen Math Working Tools */}
      <MathToolsDrawer />

      {/* Skip Confirmation Modal */}
      {showSkipConfirm ? (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4"
        >
          <GalaxyCard className="max-w-md w-full space-y-4">
            <h3 className="text-lg font-extrabold text-white">{t("skipConfirmTitle")}</h3>
            <p className="text-sm text-slate-300">{t("skipConfirmBody")}</p>
            <div className="flex justify-end gap-2">
              <GalaxyButton variant="secondary" onClick={() => setShowSkipConfirm(false)}>
                {t("cancel")}
              </GalaxyButton>
              <GalaxyButton
                variant="danger"
                onClick={async () => {
                  setShowSkipConfirm(false);
                  await onSkip?.();
                }}
              >
                {t("confirmSkip")}
              </GalaxyButton>
            </div>
          </GalaxyCard>
        </div>
      ) : null}

      {/* Question Feedback Modal */}
      {showFeedback ? (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4"
        >
          <GalaxyCard className="max-w-md w-full space-y-4">
            <h3 className="text-lg font-extrabold text-white">{t("feedbackModal.title")}</h3>
            {feedbackSent ? (
              <p className="text-sm text-emerald-300">{t("feedbackModal.sent")}</p>
            ) : (
              <textarea
                rows={3}
                value={feedbackText}
                onChange={(e) => setFeedbackText(e.target.value)}
                placeholder={t("feedbackModal.placeholder")}
                className="w-full rounded-xl border border-indigo-400/30 bg-[#0B1026] p-3 text-sm text-white"
              />
            )}
            <div className="flex justify-end gap-2">
              <GalaxyButton variant="secondary" onClick={() => setShowFeedback(false)}>
                {t("cancel")}
              </GalaxyButton>
              {!feedbackSent ? (
                <GalaxyButton
                  onClick={() => {
                    setFeedbackSent(true);
                    setFeedbackText("");
                  }}
                >
                  {t("feedbackModal.send")}
                </GalaxyButton>
              ) : null}
            </div>
          </GalaxyCard>
        </div>
      ) : null}
    </div>
  );
}
