import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Flame, Sparkles } from "lucide-react";
import type { Locale } from "@planetlogi/i18n";
import type { RenderedQuestion } from "@planetlogi/api-client";
import { GalaxyButton, GalaxyCard } from "@planetlogi/ui";
import { apiClient, useAuthStore } from "../../../store/authStore";

export function EventsPage({ locale }: { locale: Locale }) {
  const { t } = useTranslation("social");
  const qc = useQueryClient();
  const user = useAuthStore((s) => s.user);
  const grade = user?.grade ?? 1;

  const [meteorSess, setMeteorSess] = useState<{ sessionId: string; questions: RenderedQuestion[] } | null>(null);
  const [meteorAnswers, setMeteorAnswers] = useState<Record<number, string>>({});
  const [meteorResult, setMeteorResult] = useState<{ correct: number; total: number; rewardSp: number } | null>(null);

  const [title, setTitle] = useState("");
  const [summary, setSummary] = useState("");
  const [creatorToast, setCreatorToast] = useState(false);

  const hubQuery = useQuery({
    queryKey: ["events-hub", locale],
    queryFn: () => apiClient.getEventsHub(locale),
  });

  const startMeteorMut = useMutation({
    mutationFn: () => apiClient.startSundayMeteor(grade, locale),
    onSuccess: (s) => {
      setMeteorSess(s);
      setMeteorAnswers({});
      setMeteorResult(null);
    },
  });

  const submitMeteorMut = useMutation({
    mutationFn: () => {
      const payload: Record<number, Record<string, string>> = {};
      Object.entries(meteorAnswers).forEach(([idx, val]) => {
        payload[Number(idx)] = { answer: val.trim() };
      });
      return apiClient.submitSundayMeteor(meteorSess!.sessionId, payload, locale);
    },
    onSuccess: (res) => {
      setMeteorSess(null);
      setMeteorResult(res);
    },
  });

  const creatorMut = useMutation({
    mutationFn: () => apiClient.submitCreatorChallenge(title, summary, grade),
    onSuccess: () => {
      setTitle("");
      setSummary("");
      setCreatorToast(true);
      void qc.invalidateQueries({ queryKey: ["events-hub"] });
    },
  });

  const hub = hubQuery.data;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-extrabold text-white">{t("events.title")}</h1>
        <p className="mt-1 text-sm text-slate-300">{t("events.subtitle")}</p>
      </div>

      {/* Sunday Meteor Challenge */}
      <GalaxyCard className="space-y-4 border-orange-400/30">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-orange-500/20 text-orange-300">
              <Flame className="h-6 w-6" />
            </span>
            <div>
              <h2 className="text-xl font-extrabold text-white">{t("events.sundayTitle")}</h2>
              <p className="text-xs text-slate-300">{hub?.sundayWindowNotice}</p>
            </div>
          </div>
          <span className="rounded-lg bg-orange-500/20 px-3 py-1 text-xs font-extrabold text-orange-200">
            {hub?.sundayMeteorOpen ? t("events.sundayOpen") : t("events.sundayPreview")}
          </span>
        </div>

        {meteorResult ? (
          <div className="rounded-xl bg-emerald-500/20 p-4 text-sm font-bold text-emerald-200">
            {t("events.sundayDone", {
              correct: meteorResult.correct,
              total: meteorResult.total,
              sp: meteorResult.rewardSp,
            })}
          </div>
        ) : null}

        {meteorSess ? (
          <div className="space-y-3">
            {meteorSess.questions.map((q, idx) => (
              <div key={q.id} className="rounded-xl bg-[#0B1026] p-3.5 space-y-2">
                <p className="text-sm font-bold text-white">
                  #{idx + 1}. {q.stem}
                </p>
                <input
                  type="text"
                  value={meteorAnswers[idx] ?? ""}
                  onChange={(e) => setMeteorAnswers((a) => ({ ...a, [idx]: e.target.value }))}
                  className="min-h-11 w-full sm:w-56 rounded-xl border border-indigo-400/35 bg-[#131B3A] px-3 text-sm font-bold text-white"
                />
              </div>
            ))}
            <GalaxyButton onClick={() => submitMeteorMut.mutate()} disabled={submitMeteorMut.isPending}>
              {t("events.sundaySubmit")}
            </GalaxyButton>
          </div>
        ) : (
          <GalaxyButton onClick={() => startMeteorMut.mutate()} disabled={startMeteorMut.isPending}>
            ☄️ {t("events.sundayStart")}
          </GalaxyButton>
        )}
      </GalaxyCard>

      {/* Quarterly Creator Challenge */}
      <div className="grid gap-6 lg:grid-cols-2">
        <GalaxyCard className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-extrabold text-white flex items-center gap-2">
              <Sparkles className="h-5 w-5 text-cyan-300" /> {t("events.creatorTitle")}
            </h2>
            <span className="rounded-lg bg-cyan-500/20 px-2.5 py-1 text-xs font-bold text-cyan-200">
              {t("events.creatorQuarter", { quarter: hub?.creatorQuarter ?? "2026-Q4" })}
            </span>
          </div>
          <p className="text-sm text-cyan-100">{hub?.creatorTheme}</p>

          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (title.trim().length >= 3 && summary.trim().length >= 10) {
                creatorMut.mutate();
              }
            }}
            className="space-y-3"
          >
            <p className="text-xs font-bold uppercase tracking-wider text-slate-300">
              {t("events.creatorFormTitle")}
            </p>
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder={t("events.creatorTitlePlaceholder")}
              className="min-h-11 w-full rounded-xl border border-indigo-400/35 bg-[#0B1026] px-3.5 text-sm text-white"
              required
              minLength={3}
            />
            <textarea
              value={summary}
              onChange={(e) => setSummary(e.target.value)}
              placeholder={t("events.creatorSummaryPlaceholder")}
              rows={3}
              className="w-full rounded-xl border border-indigo-400/35 bg-[#0B1026] p-3.5 text-sm text-white"
              required
              minLength={10}
            />
            <GalaxyButton type="submit" disabled={creatorMut.isPending}>
              {t("events.creatorSubmit")}
            </GalaxyButton>
            {creatorToast ? (
              <p className="text-sm font-bold text-emerald-300">{t("events.creatorSubmitted")}</p>
            ) : null}
          </form>
        </GalaxyCard>

        <GalaxyCard className="space-y-3">
          <h2 className="text-xl font-extrabold text-white">{t("events.creatorGallery")}</h2>
          <div className="space-y-3">
            {(hub?.creatorEntries ?? []).map((entry) => (
              <div
                key={entry.id}
                className="rounded-2xl border border-indigo-400/20 bg-[#0B1026]/80 p-4 space-y-1"
              >
                <div className="flex items-center justify-between">
                  <p className="font-bold text-white">{entry.title}</p>
                  <span className="text-xs font-semibold text-cyan-300">
                    {entry.authorMask} • G{entry.grade}
                  </span>
                </div>
                <p className="text-sm text-slate-300">{entry.summary}</p>
              </div>
            ))}
          </div>
        </GalaxyCard>
      </div>
    </div>
  );
}
